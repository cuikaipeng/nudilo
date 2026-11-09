package channels

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"callbell/internal/locale"
	"callbell/internal/model"
)

type Failure struct {
	// Field identifies a local form error; network failures leave it empty.
	Field      string
	Detail     locale.Message
	Message    string
	Retryable  bool
	RetryAfter time.Duration
}

func (f *Failure) Error() string {
	if f.Detail.Key != "" {
		return f.Detail.Error()
	}
	return f.Message
}
func (f *Failure) Unwrap() error {
	if f.Detail.Key != "" {
		return f.Detail
	}
	return locale.M(f.Message)
}
func invalid(message string, args ...any) error { return &Failure{Detail: locale.M(message, args...)} }
func invalidField(field, message string, args ...any) error {
	return &Failure{Field: field, Detail: locale.M(message, args...)}
}

var tokenPattern = regexp.MustCompile(`^[0-9]+:[A-Za-z0-9_-]{20,}$`)
var chatPattern = regexp.MustCompile(`^(-?[0-9]+|@[A-Za-z0-9_]{5,})$`)

func Validate(channel model.Channel, c model.Credentials) error {
	c.Normalize(channel)
	if len(c.Webhook) > 1800 || len(c.Token) > 4096 || len(c.Secret) > 1024 || len(c.ChatID) > 100 || len(c.AppID) > 200 || len(c.AppSecret) > 4096 || len(c.RobotCode) > 200 || len(c.RecipientID) > 254 {
		return invalid("The configuration is too long. Copy it again.")
	}
	if c.Mode != model.Group && c.Mode != model.Direct {
		return invalid("Choose a notification destination.")
	}
	if c.Mode == model.Direct {
		return validateDirect(channel, c)
	}
	if channel == model.Telegram {
		if !tokenPattern.MatchString(c.Token) {
			return invalidField("token", "Enter the complete Bot Token from BotFather.")
		}
		if !chatPattern.MatchString(c.ChatID) || (!strings.HasPrefix(c.ChatID, "-") && !strings.HasPrefix(c.ChatID, "@")) {
			return invalidField("group-chat", "Enter a group Chat ID starting with - or a channel @username.")
		}
		return nil
	}
	u, err := url.Parse(c.Webhook)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Fragment != "" || (u.Port() != "" && u.Port() != "443") {
		return invalidField("webhook", "Paste the complete HTTPS bot URL.")
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	switch channel {
	case model.Feishu, model.Lark:
		host := "open.feishu.cn"
		if channel == model.Lark {
			host = "open.larksuite.com"
		}
		if u.Hostname() != host || len(parts) != 5 || strings.Join(parts[:4], "/") != "open-apis/bot/v2/hook" || parts[4] == "" || u.RawQuery != "" {
			return invalidField("webhook", "Enter the complete {channel} group bot Webhook.", locale.M(channel.Name()))
		}
	case model.DingTalk:
		if u.Hostname() != "oapi.dingtalk.com" || u.Path != "/robot/send" || u.Query().Get("access_token") == "" {
			return invalidField("webhook", "Enter a DingTalk bot URL containing access_token.")
		}
	case model.Slack:
		if (u.Hostname() != "hooks.slack.com" && u.Hostname() != "hooks.slack-gov.com") || len(parts) != 4 || parts[0] != "services" || u.RawQuery != "" {
			return invalidField("webhook", "Enter a Slack Incoming Webhook URL.")
		}
	default:
		return invalid("This channel is not supported.")
	}
	return nil
}

func Request(ctx context.Context, channel model.Channel, c model.Credentials, event model.Event, now time.Time) (*http.Request, error) {
	c.Normalize(channel)
	if err := Validate(channel, c); err != nil {
		return nil, err
	}
	if c.Mode == model.Direct && channel != model.Telegram {
		return nil, invalid("Application credentials are required for personal messages.")
	}
	endpoint := c.Webhook
	var payload any
	switch channel {
	case model.Feishu, model.Lark:
		body := map[string]any{"msg_type": "text", "content": map[string]string{"text": event.Message()}}
		if c.Secret != "" {
			stamp := strconv.FormatInt(now.Unix(), 10)
			body["timestamp"] = stamp
			body["sign"] = sign(stamp+"\n"+c.Secret, "")
		}
		payload = body
	case model.DingTalk:
		payload = map[string]any{"msgtype": "text", "text": map[string]string{"content": event.Message()}, "at": map[string]bool{"isAtAll": false}}
		if c.Secret != "" {
			u, _ := url.Parse(endpoint)
			q := u.Query()
			stamp := strconv.FormatInt(now.UnixMilli(), 10)
			q.Set("timestamp", stamp)
			q.Set("sign", sign(c.Secret, stamp+"\n"+c.Secret))
			u.RawQuery = q.Encode()
			endpoint = u.String()
		}
	case model.Telegram:
		endpoint = "https://api.telegram.org/bot" + c.Token + "/sendMessage"
		chatID := c.ChatID
		if c.Mode == model.Direct {
			chatID = c.RecipientID
		}
		payload = map[string]any{"chat_id": chatID, "text": event.Message(), "link_preview_options": map[string]bool{"is_disabled": true}}
	case model.Slack:
		payload = map[string]any{"text": event.Message(), "mrkdwn": false, "unfurl_links": false, "unfurl_media": false}
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(b))
	if err != nil {
		return nil, invalid("The bot URL is invalid.")
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	return req, nil
}
func sign(key, text string) string {
	h := hmac.New(sha256.New, []byte(key))
	h.Write([]byte(text))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

type Client struct {
	HTTP    *http.Client
	tokenMu sync.Mutex
	tokens  map[model.Channel]cachedToken
}

func NewClient(transport http.RoundTripper) *Client {
	return &Client{HTTP: &http.Client{Transport: transport, Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (c *Client) Send(ctx context.Context, channel model.Channel, credentials model.Credentials, event model.Event) error {
	credentials.Normalize(channel)
	if err := Validate(channel, credentials); err != nil {
		return err
	}
	if credentials.Mode == model.Direct && channel != model.Telegram {
		return c.sendDirect(ctx, channel, credentials, event)
	}
	req, err := Request(ctx, channel, credentials, event, time.Now())
	if err != nil {
		return err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return &Failure{Message: "Connection failed. Check your network or system proxy.", Retryable: true}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 65537))
	if err != nil || len(body) > 65536 {
		return &Failure{Message: "The platform response was incomplete. Delivery is unconfirmed.", Retryable: true}
	}
	return ValidateResponse(channel, resp.StatusCode, resp.Header, body)
}

func ValidateResponse(channel model.Channel, status int, headers http.Header, body []byte) error {
	if status < 200 || status >= 300 {
		wait, _ := strconv.Atoi(headers.Get("Retry-After"))
		var data struct {
			Parameters struct {
				Retry int `json:"retry_after"`
			} `json:"parameters"`
		}
		_ = json.Unmarshal(body, &data)
		if data.Parameters.Retry > wait {
			wait = data.Parameters.Retry
		}
		if wait > 3600 {
			wait = 3600
		}
		if wait < 0 {
			wait = 0
		}
		return &Failure{Detail: locale.M("{channel} returned HTTP {code}. Check the connection and permissions.", locale.M(channel.Name()), status), Retryable: status == 429 || status >= 500, RetryAfter: time.Duration(wait) * time.Second}
	}
	if channel == model.Slack {
		if strings.TrimSpace(string(body)) == "ok" {
			return nil
		}
		return invalid("Slack did not confirm delivery. Check the Webhook and channel permissions.")
	}
	var result struct {
		Code       *int  `json:"code"`
		LegacyCode *int  `json:"StatusCode"`
		ErrCode    *int  `json:"errcode"`
		OK         *bool `json:"ok"`
		ErrorCode  int   `json:"error_code"`
	}
	if json.Unmarshal(body, &result) != nil {
		return invalid("The platform response was not recognized. Delivery is unconfirmed.")
	}
	if channel == model.Telegram {
		if result.OK != nil && *result.OK {
			return nil
		}
		return invalid("Telegram did not confirm delivery ({code}). Send /start to the bot and check the Chat ID.", result.ErrorCode)
	}
	code := result.Code
	if code == nil {
		code = result.LegacyCode
	}
	if channel == model.DingTalk {
		code = result.ErrCode
	}
	if code == nil {
		return invalid("The platform response has no result. Delivery is unconfirmed.")
	}
	if *code == 0 {
		return nil
	}
	return &Failure{Detail: locale.M("{channel} rejected the message ({code}). Check signing, keywords and bot settings.", locale.M(channel.Name()), *code), Retryable: *code == 11232 || *code == 130101}
}

func Retry(err error, attempts int) (time.Duration, bool) {
	var failure *Failure
	if !errors.As(err, &failure) || !failure.Retryable || attempts >= 3 {
		return 0, false
	}
	delay := 5 * time.Second
	if attempts > 1 {
		delay = 30 * time.Second
	}
	if failure.RetryAfter > delay {
		delay = failure.RetryAfter
	}
	return delay, true
}
