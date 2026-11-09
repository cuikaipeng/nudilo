package channels

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"callbell/internal/model"
)

// Read-only API calls. Never follow redirects, acknowledge Telegram updates,
// delete Webhooks, or send the pairing message on the user's behalf.
func (c *Client) pairGet(ctx context.Context, ch model.Channel, endpoint, token string, target any) error {
	for {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := c.HTTP.Do(req)
		if err != nil {
			return pairConnectionError()
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1024*1024+1))
		resp.Body.Close()
		if resp.StatusCode == 429 {
			delay, _ := strconv.Atoi(resp.Header.Get("Retry-After"))
			var retry struct {
				Parameters struct {
					After int `json:"retry_after"`
				} `json:"parameters"`
			}
			_ = json.Unmarshal(body, &retry)
			if err = pairWait(ctx, time.Duration(max(1, min(3600, max(delay, retry.Parameters.After))))*time.Second); err != nil {
				return err
			}
			continue
		}
		if ch == model.Telegram && resp.StatusCode == 409 {
			return invalid("This Telegram bot has a Webhook or another update reader. Use a dedicated notification bot.")
		}
		if readErr != nil || len(body) > 1024*1024 || resp.StatusCode != 200 {
			return pairConnectionError()
		}
		var status struct {
			OK    bool   `json:"ok"`
			Error string `json:"error"`
		}
		if json.Unmarshal(body, &status) != nil || !status.OK {
			if ch == model.Slack && status.Error == "missing_scope" {
				return invalid("For Slack pairing, grant im:read and im:history, then reinstall the app.")
			}
			return invalid("Cannot read bot messages. Check the token and whether another service is using this bot.")
		}
		if json.Unmarshal(body, target) != nil {
			return pairConnectionError()
		}
		return nil
	}
}

var telegramUsername = regexp.MustCompile(`^[A-Za-z0-9_]{5,32}$`)

func (c *Client) pairTelegram(ctx context.Context, token, code string, since time.Time, ready func(string)) (PairResult, error) {
	var me struct {
		Result struct {
			IsBot    bool   `json:"is_bot"`
			Username string `json:"username"`
		} `json:"result"`
	}
	if err := c.pairGet(ctx, model.Telegram, "https://api.telegram.org/bot"+token+"/getMe", "", &me); err != nil {
		return PairResult{}, err
	}
	if !me.Result.IsBot || !telegramUsername.MatchString(me.Result.Username) {
		return PairResult{}, invalid("Enter the complete Bot Token from BotFather.")
	}
	announced := false
	for {
		id, err := c.pollTelegramPair(ctx, token, code, since)
		if err != nil {
			return PairResult{}, err
		}
		if !announced {
			ready("https://t.me/" + me.Result.Username + "?start=" + code)
			announced = true
		}
		if id != "" {
			return PairResult{RecipientID: id}, nil
		}
		if err = pairWait(ctx, 3*time.Second); err != nil {
			return PairResult{}, err
		}
	}
}

func (c *Client) pollTelegramPair(ctx context.Context, token, code string, since time.Time) (string, error) {
	var data struct {
		Result []struct {
			Message struct {
				Text string `json:"text"`
				Date int64  `json:"date"`
				From struct {
					ID    int64 `json:"id"`
					IsBot bool  `json:"is_bot"`
				} `json:"from"`
				Chat struct {
					ID   int64  `json:"id"`
					Type string `json:"type"`
				} `json:"chat"`
				Forward json.RawMessage `json:"forward_origin"`
			} `json:"message"`
		} `json:"result"`
	}
	if err := c.pairGet(ctx, model.Telegram, "https://api.telegram.org/bot"+token+"/getUpdates?limit=100&timeout=0", "", &data); err != nil {
		return "", err
	}
	id := ""
	for _, update := range data.Result {
		m := update.Message
		if m.Chat.Type != "private" || m.Chat.ID <= 0 || m.From.ID != m.Chat.ID || m.From.IsBot || len(m.Forward) > 0 || !freshPairTime(time.Unix(m.Date, 0), since) || (m.Text != code && m.Text != "/start "+code) {
			continue
		}
		candidate := strconv.FormatInt(m.Chat.ID, 10)
		if id != "" && id != candidate {
			return "", invalid("Multiple accounts used this code. Generate a new one.")
		}
		id = candidate
	}
	if id == "" && len(data.Result) >= 100 {
		return "", invalid("This bot has too many pending updates. Use a dedicated notification bot.")
	}
	return id, nil
}

type slackPairMessage struct {
	Type    string          `json:"type"`
	Subtype string          `json:"subtype"`
	User    string          `json:"user"`
	BotID   string          `json:"bot_id"`
	Text    string          `json:"text"`
	TS      string          `json:"ts"`
	Edited  json.RawMessage `json:"edited"`
}

func matchSlackPair(m slackPairMessage, user, code string, since time.Time) bool {
	seconds, err := strconv.ParseInt(strings.SplitN(m.TS, ".", 2)[0], 10, 64)
	return err == nil && freshPairTime(time.Unix(seconds, 0), since) && m.Type == "message" && m.Subtype == "" && m.BotID == "" && len(m.Edited) == 0 && m.Text == code && m.User == user && slackUserID.MatchString(user)
}

// Slack's bot token can discover its private conversations with im:read and
// im:history. This avoids an additional app-level token, Socket Mode and a
// public callback server. Only messages newer than this attempt are requested.
func (c *Client) pairSlack(ctx context.Context, token, code string, since time.Time, ready func(string)) (PairResult, error) {
	announced := false
	var lastHistory, lastList time.Time
	get := func(method string, params url.Values, target any) error {
		last, spacing := &lastHistory, 1300*time.Millisecond
		if method == "conversations.list" {
			last, spacing = &lastList, 3100*time.Millisecond
		}
		if delay := time.Until(last.Add(spacing)); delay > 0 {
			if err := pairWait(ctx, delay); err != nil {
				return err
			}
		}
		*last = time.Now()
		return c.pairGet(ctx, model.Slack, "https://slack.com/api/"+method+"?"+params.Encode(), token, target)
	}
	for {
		cursor, found := "", ""
		for page := 0; ; page++ {
			var list struct {
				Channels []struct {
					ID   string `json:"id"`
					User string `json:"user"`
					IsIM bool   `json:"is_im"`
				} `json:"channels"`
				Meta struct {
					Next string `json:"next_cursor"`
				} `json:"response_metadata"`
			}
			if err := get("conversations.list", url.Values{"types": {"im"}, "limit": {"100"}, "cursor": {cursor}}, &list); err != nil {
				return PairResult{}, err
			}
			if !announced {
				ready("")
				announced = true
			}
			for _, conversation := range list.Channels {
				if !conversation.IsIM || !strings.HasPrefix(conversation.ID, "D") || !slackUserID.MatchString(conversation.User) {
					continue
				}
				historyCursor := ""
				for historyPage := 0; ; historyPage++ {
					var history struct {
						Messages []slackPairMessage `json:"messages"`
						HasMore  bool               `json:"has_more"`
						Meta     struct {
							Next string `json:"next_cursor"`
						} `json:"response_metadata"`
					}
					if err := get("conversations.history", url.Values{"channel": {conversation.ID}, "oldest": {strconv.FormatInt(since.Unix()-5, 10)}, "limit": {"100"}, "cursor": {historyCursor}}, &history); err != nil {
						return PairResult{}, err
					}
					for _, message := range history.Messages {
						if matchSlackPair(message, conversation.User, code, since) {
							if found != "" && found != message.User {
								return PairResult{}, invalid("Multiple accounts used this code. Generate a new one.")
							}
							found = message.User
						}
					}
					if history.Meta.Next == "" {
						if history.HasMore {
							return PairResult{}, invalid("Too many bot conversations to scan. Use a dedicated notification bot.")
						}
						break
					}
					if history.Meta.Next == historyCursor || historyPage >= 49 {
						return PairResult{}, pairConnectionError()
					}
					historyCursor = history.Meta.Next
				}
			}
			if list.Meta.Next == "" {
				break
			}
			if list.Meta.Next == cursor || page >= 49 {
				return PairResult{}, pairConnectionError()
			}
			cursor = list.Meta.Next
		}
		if found != "" {
			return PairResult{RecipientID: found}, nil
		}
		if err := pairWait(ctx, 5*time.Second); err != nil {
			return PairResult{}, err
		}
	}
}
