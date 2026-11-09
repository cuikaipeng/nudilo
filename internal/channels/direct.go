package channels

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"time"

	"callbell/internal/locale"
	"callbell/internal/model"
)

var opaqueID = regexp.MustCompile(`^[A-Za-z0-9_.:-]+$`)
var slackUserID = regexp.MustCompile(`^[UW][A-Z0-9]{5,}$`)
var slackBotToken = regexp.MustCompile(`^xoxb-[A-Za-z0-9-]+$`)

func validateDirect(ch model.Channel, c model.Credentials) error {
	if err := ValidatePairCredentials(ch, c); err != nil {
		return err
	}
	if c.RecipientID == "" {
		return invalidField("pairing", "Pair your account before saving.")
	}
	switch ch {
	case model.Telegram:
		if !tokenPattern.MatchString(c.Token) {
			return invalidField("token", "Enter the complete Bot Token from BotFather.")
		}
		id, err := strconv.ParseInt(c.RecipientID, 10, 64)
		if err != nil || id <= 0 {
			return invalidField("private-chat", "Identify your account or enter your positive private Chat ID.")
		}
	case model.Slack:
		if !slackBotToken.MatchString(c.Token) {
			return invalidField("token", "Enter a Slack Bot User OAuth Token starting with xoxb-.")
		}
		if !slackUserID.MatchString(c.RecipientID) {
			return invalidField("recipient", "Enter one Slack member ID, starting with U or W.")
		}
	case model.Feishu, model.Lark, model.DingTalk:
		if !opaqueID.MatchString(c.AppID) {
			return invalidField("app-id", "Enter the application ID.")
		}
		if c.AppSecret == "" || strings.ContainsAny(c.AppSecret, "\r\n") {
			return invalidField("app-secret", "Enter the application secret.")
		}
		if ch == model.DingTalk {
			if !opaqueID.MatchString(c.RobotCode) {
				return invalidField("robot-code", "Enter the Robot Code from your DingTalk app.")
			}
			if !opaqueID.MatchString(c.RecipientID) {
				return invalidField("recipient", "Enter one DingTalk user ID from your organization.")
			}
			return nil
		}
		switch c.RecipientType {
		case "email":
			address, err := mail.ParseAddress(c.RecipientID)
			if err != nil || address.Address != c.RecipientID {
				return invalidField("recipient", "Enter the recipient's account email address.")
			}
		case "open_id":
			if !strings.HasPrefix(c.RecipientID, "ou_") || len(c.RecipientID) <= 3 || !opaqueID.MatchString(c.RecipientID) {
				return invalidField("recipient", "Enter an Open ID starting with ou_ from this application.")
			}
		default:
			return invalidField("recipient", "Choose Email or Open ID for a personal message.")
		}
	default:
		return invalid("This channel is not supported.")
	}
	return nil
}

type cachedToken struct {
	key     [32]byte
	value   string
	expires time.Time
}

type apiReply struct {
	status int
	header http.Header
	body   []byte
}

func (c *Client) apiPost(ctx context.Context, endpoint, token string, ding bool, payload any) (apiReply, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return apiReply{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return apiReply{}, invalid("The bot URL is invalid.")
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	if token != "" {
		if ding {
			req.Header.Set("x-acs-dingtalk-access-token", token)
		} else {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return apiReply{}, &Failure{Message: "Connection failed. Check your network or system proxy.", Retryable: true}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 65537))
	if err != nil || len(body) > 65536 {
		return apiReply{}, &Failure{Message: "The platform response was incomplete. Delivery is unconfirmed.", Retryable: true}
	}
	return apiReply{resp.StatusCode, resp.Header, body}, nil
}

func (r apiReply) httpError(ch model.Channel) error {
	if r.status >= 200 && r.status < 300 {
		return nil
	}
	return ValidateResponse(ch, r.status, r.header, r.body)
}

func larkHost(ch model.Channel) string {
	if ch == model.Lark {
		return "https://open.larksuite.com"
	}
	return "https://open.feishu.cn"
}

func (c *Client) appToken(ctx context.Context, ch model.Channel, credentials model.Credentials) (string, error) {
	key := sha256.Sum256([]byte(credentials.AppID + "\x00" + credentials.AppSecret))
	c.tokenMu.Lock()
	cached := c.tokens[ch]
	c.tokenMu.Unlock()
	if cached.key == key && time.Now().Before(cached.expires) {
		return cached.value, nil
	}
	endpoint := "https://accounts.feishu.cn/oauth/v3/token"
	payload := map[string]string{"grant_type": "client_credentials", "client_id": credentials.AppID, "client_secret": credentials.AppSecret, "scope": "im:message:send_as_bot"}
	if ch == model.Lark {
		// Lark's official SDK uses this custom-app token endpoint.
		endpoint = larkHost(ch) + "/open-apis/auth/v3/tenant_access_token/internal"
		payload = map[string]string{"app_id": credentials.AppID, "app_secret": credentials.AppSecret}
	} else if ch == model.DingTalk {
		endpoint = "https://api.dingtalk.com/v1.0/oauth2/accessToken"
		payload = map[string]string{"appKey": credentials.AppID, "appSecret": credentials.AppSecret}
	}
	r, err := c.apiPost(ctx, endpoint, "", false, payload)
	if err != nil {
		return "", err
	}
	if err = r.httpError(ch); err != nil {
		return "", err
	}
	var data struct {
		Code      *int   `json:"code"`
		Token     string `json:"access_token"`
		TTL       int    `json:"expires_in"`
		LarkToken string `json:"tenant_access_token"`
		LarkTTL   int    `json:"expire"`
		DingToken string `json:"accessToken"`
		DingTTL   int    `json:"expireIn"`
	}
	if json.Unmarshal(r.body, &data) != nil || (ch != model.DingTalk && (data.Code == nil || *data.Code != 0)) {
		return "", invalid("Could not authenticate {channel}. Check application credentials and publishing status.", locale.M(ch.Name()))
	}
	token, ttl := data.Token, data.TTL
	if ch == model.Lark {
		token, ttl = data.LarkToken, data.LarkTTL
	}
	if ch == model.DingTalk {
		token, ttl = data.DingToken, data.DingTTL
	}
	if token == "" || len(token) > 16384 || strings.ContainsAny(token, "\r\n") || ttl <= 0 || ttl > 86400 {
		return "", invalid("The platform did not return a valid access token.")
	}
	c.tokenMu.Lock()
	if c.tokens == nil {
		c.tokens = make(map[model.Channel]cachedToken)
	}
	c.tokens[ch] = cachedToken{key, token, time.Now().Add(time.Duration(ttl-min(60, ttl/10)) * time.Second)}
	c.tokenMu.Unlock()
	return token, nil
}

func (c *Client) forgetToken(ch model.Channel, token string) {
	c.tokenMu.Lock()
	if c.tokens[ch].value == token {
		delete(c.tokens, ch)
	}
	c.tokenMu.Unlock()
}

func tokenRejected(ch model.Channel, r apiReply) bool {
	if r.status == http.StatusUnauthorized {
		return true
	}
	if ch == model.Feishu || ch == model.Lark {
		var result struct {
			Code int `json:"code"`
		}
		_ = json.Unmarshal(r.body, &result)
		return result.Code == 99991663 || result.Code == 99991664 || result.Code == 99991671
	}
	return false
}

func (c *Client) sendDirect(ctx context.Context, ch model.Channel, credentials model.Credentials, event model.Event) error {
	// Retry authentication once only after an explicit rejection. A timeout may
	// mean a message was accepted, so it is left to the existing bounded queue.
	for attempt := 0; attempt < 2; attempt++ {
		token := credentials.Token
		if ch != model.Slack {
			var err error
			token, err = c.appToken(ctx, ch, credentials)
			if err != nil {
				return err
			}
		}
		endpoint := ""
		var payload any
		switch ch {
		case model.Feishu, model.Lark:
			endpoint = larkHost(ch) + "/open-apis/im/v1/messages?receive_id_type=" + credentials.RecipientType
			content, _ := json.Marshal(map[string]string{"text": event.Message()})
			body := map[string]any{"receive_id": credentials.RecipientID, "msg_type": "text", "content": string(content)}
			if event.ID != "" {
				digest := sha256.Sum256([]byte(event.ID + "\x00" + credentials.AppID + "\x00" + credentials.RecipientType + "\x00" + credentials.RecipientID))
				body["uuid"] = "nudilo-" + hex.EncodeToString(digest[:16])
			}
			payload = body
		case model.DingTalk:
			endpoint = "https://api.dingtalk.com/v1.0/robot/oToMessages/batchSend"
			content, _ := json.Marshal(map[string]string{"content": event.Message()})
			payload = map[string]any{"robotCode": credentials.RobotCode, "userIds": []string{credentials.RecipientID}, "msgKey": "sampleText", "msgParam": string(content)}
		case model.Slack:
			endpoint = "https://slack.com/api/chat.postMessage"
			payload = map[string]any{"channel": credentials.RecipientID, "text": event.Message(), "mrkdwn": false, "parse": "none", "unfurl_links": false, "unfurl_media": false}
		}
		r, err := c.apiPost(ctx, endpoint, token, ch == model.DingTalk, payload)
		if err != nil {
			return err
		}
		if ch != model.Slack && tokenRejected(ch, r) {
			c.forgetToken(ch, token)
			if attempt == 0 {
				continue
			}
		}
		return validateDirectResponse(ch, r)
	}
	return invalid("The application token was rejected. Check credentials and permissions.")
}

func validateDirectResponse(ch model.Channel, r apiReply) error {
	if err := r.httpError(ch); err != nil {
		return err
	}
	unconfirmed := "The platform response was not recognized. Delivery is unconfirmed."
	switch ch {
	case model.Feishu, model.Lark:
		var result struct {
			Code *int `json:"code"`
			Data struct {
				MessageID string `json:"message_id"`
			} `json:"data"`
		}
		if json.Unmarshal(r.body, &result) != nil || result.Code == nil {
			return invalid(unconfirmed)
		}
		if *result.Code == 0 && result.Data.MessageID != "" {
			return nil
		}
		if *result.Code == 0 {
			return invalid(unconfirmed)
		}
		return &Failure{Detail: locale.M("{channel} rejected the personal message ({code}). Check the recipient, bot permissions and app availability.", locale.M(ch.Name()), *result.Code), Retryable: *result.Code == 99991400 || *result.Code == 230020 || *result.Code == 99991500}
	case model.DingTalk:
		var result struct {
			Key      string   `json:"processQueryKey"`
			Invalid  []string `json:"invalidStaffIdList"`
			Filtered []string `json:"filteredStaffIdList"`
			Limited  []string `json:"flowControlledStaffIdList"`
		}
		if json.Unmarshal(r.body, &result) != nil {
			return invalid(unconfirmed)
		}
		if len(result.Invalid) > 0 || len(result.Filtered) > 0 {
			return invalid("DingTalk could not reach this user. Check the user ID and the robot's availability.")
		}
		if len(result.Limited) > 0 {
			return &Failure{Message: "DingTalk temporarily limited personal messages. Nudilo will retry.", Retryable: true, RetryAfter: time.Minute}
		}
		if result.Key == "" {
			return invalid(unconfirmed)
		}
		return nil
	case model.Slack:
		var result struct {
			OK      *bool  `json:"ok"`
			Error   string `json:"error"`
			Channel string `json:"channel"`
			TS      string `json:"ts"`
		}
		if json.Unmarshal(r.body, &result) != nil || result.OK == nil {
			return invalid(unconfirmed)
		}
		if *result.OK && strings.HasPrefix(result.Channel, "D") && result.TS != "" {
			return nil
		}
		if *result.OK {
			return invalid(unconfirmed)
		}
		switch result.Error {
		case "missing_scope":
			return invalid("Grant chat:write to your Slack bot and reinstall the app in the workspace.")
		case "invalid_auth", "token_revoked", "token_expired", "not_authed", "account_inactive":
			return invalid("The Slack bot token is invalid or expired. Reinstall the app and copy its Bot User OAuth Token.")
		case "channel_not_found", "user_not_found", "user_not_visible", "no_permission":
			return invalid("Slack could not reach this member. Check the member ID and workspace access.")
		case "messages_tab_disabled":
			return invalid("Enable the Messages tab in your Slack app's App Home.")
		case "ratelimited", "rate_limited", "internal_error", "service_unavailable":
			return &Failure{Message: "Slack temporarily rejected the message. Nudilo will retry.", Retryable: true, RetryAfter: time.Minute}
		default:
			return invalid("Slack rejected the personal message. Check bot permissions and workspace policies.")
		}
	}
	return invalid(unconfirmed)
}
