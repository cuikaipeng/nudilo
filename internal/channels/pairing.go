package channels

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"

	"callbell/internal/model"
	"github.com/coder/websocket"
)

const PairDuration = 3 * time.Minute

type PairResult struct {
	RecipientID, RecipientType, RobotCode string
}

func NewPairCode() (string, error) {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", invalid("Could not generate a pairing code. Try again.")
	}
	return "nudilo-" + hex.EncodeToString(b[:]), nil
}

// ValidatePairCredentials checks only the platform credentials, before opening
// a temporary receiver. A recipient is deliberately not required here.
func ValidatePairCredentials(ch model.Channel, c model.Credentials) error {
	c.Normalize(ch)
	if c.Mode != model.Direct {
		return invalid("Choose Personal DM to pair an account.")
	}
	if len(c.Token) > 4096 || len(c.AppID) > 200 || len(c.AppSecret) > 4096 {
		return invalid("The configuration is too long. Copy it again.")
	}
	switch ch {
	case model.Telegram:
		if !tokenPattern.MatchString(c.Token) {
			return invalidField("token", "Enter the complete Bot Token from BotFather.")
		}
	case model.Slack:
		if !slackBotToken.MatchString(c.Token) {
			return invalidField("token", "Enter a Slack Bot User OAuth Token starting with xoxb-.")
		}
	case model.Feishu, model.Lark, model.DingTalk:
		if !opaqueID.MatchString(c.AppID) {
			return invalidField("app-id", "Enter the application ID.")
		}
		if c.AppSecret == "" || strings.ContainsAny(c.AppSecret, "\r\n") {
			return invalidField("app-secret", "Enter the application secret.")
		}
	default:
		return invalid("This channel is not supported.")
	}
	return nil
}

var pairCodePattern = regexp.MustCompile(`^nudilo-[a-f0-9]{12}$`)

// Pair listens only for the lifetime of this attempt. ready is called after the
// receiver is usable; its optional link opens the Telegram bot. No incoming
// message content, temporary URL, or pairing code is persisted or logged.
func (c *Client) Pair(parent context.Context, ch model.Channel, credentials model.Credentials, code string, since time.Time, ready func(string)) (result PairResult, err error) {
	credentials.Normalize(ch)
	if err = ValidatePairCredentials(ch, credentials); err != nil {
		return result, err
	}
	if !pairCodePattern.MatchString(code) || since.IsZero() || since.After(time.Now().Add(5*time.Second)) {
		return result, invalid("Generate a pairing code first.")
	}
	ctx, cancel := context.WithDeadline(parent, since.Add(PairDuration))
	defer cancel()
	defer func() {
		if ctx.Err() != nil {
			result = PairResult{}
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				err = invalid("Pairing timed out. Check Setup help and try again.")
			} else {
				err = context.Canceled
			}
		}
	}()
	if ready == nil {
		ready = func(string) {}
	}
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	switch ch {
	case model.Feishu, model.Lark:
		result, err = c.pairLark(ctx, ch, credentials, code, since, ready)
	case model.DingTalk:
		result, err = c.pairDingTalk(ctx, credentials, code, since, ready)
	case model.Slack:
		result, err = c.pairSlack(ctx, credentials.Token, code, since, ready)
	case model.Telegram:
		result, err = c.pairTelegram(ctx, credentials.Token, code, since, ready)
	}
	if err == nil {
		credentials.RecipientID, credentials.RecipientType = result.RecipientID, result.RecipientType
		if ch == model.DingTalk {
			credentials.RobotCode = result.RobotCode
		}
		err = Validate(ch, credentials)
	}
	return result, err
}

func freshPairTime(t, since time.Time) bool {
	return !t.Before(since.Add(-5*time.Second)) && !t.After(time.Now().Add(time.Minute))
}

func pairWait(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func pairConnectionError() error {
	return invalid("Pairing connection failed. Check your network and Setup help, then try again.")
}

// Endpoints are obtained from the authenticated platform bootstrap, never from
// user text. Restrict them to that platform before sending temporary tickets.
func pairSocketURL(ch model.Channel, raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "wss" || u.User != nil || u.Fragment != "" || (u.Port() != "" && u.Port() != "443") {
		return false
	}
	domains := []string{"feishu.cn"}
	if ch == model.Lark {
		domains = []string{"larksuite.com"}
	} else if ch == model.DingTalk {
		domains = []string{"dingtalk.com"}
	}
	for _, domain := range domains {
		if u.Hostname() == domain || strings.HasSuffix(u.Hostname(), "."+domain) {
			return true
		}
	}
	return false
}

func (c *Client) pairSocket(ctx context.Context, ch model.Channel, endpoint string) (*websocket.Conn, error) {
	if !pairSocketURL(ch, endpoint) {
		return nil, pairConnectionError()
	}
	conn, _, err := websocket.Dial(ctx, endpoint, &websocket.DialOptions{HTTPClient: c.HTTP})
	if err != nil {
		return nil, pairConnectionError()
	}
	conn.SetReadLimit(1024 * 1024)
	return conn, nil
}
