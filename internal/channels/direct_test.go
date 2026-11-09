package channels

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"callbell/internal/model"
)

func directCredentials(ch model.Channel) model.Credentials {
	c := model.Credentials{Mode: model.Direct, AppID: "app-fixture", AppSecret: "private-secret", RobotCode: "ding-fixture", RecipientType: "email", RecipientID: "recipient@example.com", Webhook: "https://unused.invalid/do-not-send-here"}
	switch ch {
	case model.DingTalk:
		c.RecipientID = "staff-fixture"
	case model.Slack:
		c.Token, c.RecipientID = "xoxb-private-fixture", "U12345FIXTURE"
	case model.Telegram:
		c.Token, c.RecipientID = "123456:abcdefghijklmnopqrstuvwxyz", "12345"
	}
	return c
}

func reply(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
}

func authResponse(ch model.Channel) string {
	switch ch {
	case model.Feishu:
		return `{"code":0,"access_token":"private-access-token","expires_in":7200}`
	case model.Lark:
		return `{"code":0,"tenant_access_token":"private-access-token","expire":7200}`
	default:
		return `{"accessToken":"private-access-token","expireIn":7200}`
	}
}

func directSuccess(ch model.Channel) string {
	switch ch {
	case model.Feishu, model.Lark:
		return `{"code":0,"data":{"message_id":"om_fixture"}}`
	case model.DingTalk:
		return `{"processQueryKey":"accepted-fixture"}`
	case model.Slack:
		return `{"ok":true,"channel":"D12345","ts":"123.456"}`
	default:
		return `{"ok":true}`
	}
}

func isAuth(r *http.Request) bool {
	return strings.Contains(r.URL.Path, "token") || strings.Contains(r.URL.Path, "accessToken")
}

func TestPrivateNotificationsUseTheirOwnAPIAndSingleRecipient(t *testing.T) {
	for _, ch := range model.Channels {
		t.Run(string(ch), func(t *testing.T) {
			credentials := directCredentials(ch)
			auths, sends := 0, 0
			var previousUUID string
			client := NewClient(roundTrip(func(r *http.Request) (*http.Response, error) {
				if r.URL.Scheme != "https" || r.Method != "POST" {
					t.Fatal("unsafe request")
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if isAuth(r) {
					auths++
					switch ch {
					case model.Feishu:
						if r.URL.String() != "https://accounts.feishu.cn/oauth/v3/token" || body["client_id"] != credentials.AppID || body["client_secret"] != credentials.AppSecret || body["grant_type"] != "client_credentials" || body["scope"] != "im:message:send_as_bot" {
							t.Fatal("wrong Feishu authentication")
						}
					case model.Lark:
						if r.URL.String() != "https://open.larksuite.com/open-apis/auth/v3/tenant_access_token/internal" || body["app_id"] != credentials.AppID || body["app_secret"] != credentials.AppSecret {
							t.Fatal("wrong Lark authentication")
						}
					case model.DingTalk:
						if r.URL.String() != "https://api.dingtalk.com/v1.0/oauth2/accessToken" || body["appKey"] != credentials.AppID || body["appSecret"] != credentials.AppSecret {
							t.Fatal("wrong DingTalk authentication")
						}
					default:
						t.Fatal("unexpected token exchange")
					}
					return reply(200, authResponse(ch)), nil
				}
				sends++
				switch ch {
				case model.Feishu, model.Lark:
					if r.URL.String() != larkHost(ch)+"/open-apis/im/v1/messages?receive_id_type=email" || r.Header.Get("Authorization") != "Bearer private-access-token" || body["receive_id"] != credentials.RecipientID || body["msg_type"] != "text" {
						t.Fatal("wrong private recipient or authorization")
					}
					var content map[string]string
					if json.Unmarshal([]byte(body["content"].(string)), &content) != nil || !strings.Contains(content["text"], "fixture-project") {
						t.Fatal("content must be a JSON string")
					}
					uuid, ok := body["uuid"].(string)
					if !ok || len(uuid) > 50 || uuid == "" || (sends > 1 && uuid != previousUUID) {
						t.Fatal("unstable deduplication ID")
					}
					previousUUID = uuid
				case model.DingTalk:
					if r.URL.String() != "https://api.dingtalk.com/v1.0/robot/oToMessages/batchSend" || r.Header.Get("x-acs-dingtalk-access-token") != "private-access-token" || body["robotCode"] != credentials.RobotCode || body["msgKey"] != "sampleText" {
						t.Fatal("wrong DingTalk send")
					}
					ids := body["userIds"].([]any)
					if len(ids) != 1 || ids[0] != credentials.RecipientID {
						t.Fatal("wrong recipient list")
					}
					var content map[string]string
					if json.Unmarshal([]byte(body["msgParam"].(string)), &content) != nil || !strings.Contains(content["content"], "fixture-project") {
						t.Fatal("msgParam must be JSON text")
					}
				case model.Slack:
					if r.URL.String() != "https://slack.com/api/chat.postMessage" || r.Header.Get("Authorization") != "Bearer "+credentials.Token || body["channel"] != credentials.RecipientID || body["mrkdwn"] != false || body["unfurl_links"] != false {
						t.Fatal("wrong Slack DM")
					}
				case model.Telegram:
					if r.URL.Host != "api.telegram.org" || body["chat_id"] != credentials.RecipientID || body["parse_mode"] != nil {
						t.Fatal("wrong Telegram private chat")
					}
				}
				return reply(200, directSuccess(ch)), nil
			}))
			event := model.Event{ID: "notification-fixture", Kind: model.InputRequired, Project: "fixture-project", Language: "zh-Hans"}
			for i := 0; i < 2; i++ {
				if err := client.Send(context.Background(), ch, credentials, event); err != nil {
					t.Fatal(err)
				}
			}
			expectedAuth := 1
			if ch == model.Telegram || ch == model.Slack {
				expectedAuth = 0
			}
			if auths != expectedAuth || sends != 2 {
				t.Fatal("token not cached", auths, sends)
			}
		})
	}
}

func TestAppTokensRefreshAfterExpiryCredentialChangeAndRejection(t *testing.T) {
	for _, ch := range []model.Channel{model.Feishu, model.Lark, model.DingTalk} {
		t.Run(string(ch), func(t *testing.T) {
			auths := 0
			reject := false
			client := NewClient(roundTrip(func(r *http.Request) (*http.Response, error) {
				if isAuth(r) {
					auths++
					return reply(200, authResponse(ch)), nil
				}
				if reject {
					reject = false
					return reply(401, `{"code":99991663}`), nil
				}
				return reply(200, directSuccess(ch)), nil
			}))
			credentials := directCredentials(ch)
			send := func() {
				t.Helper()
				if err := client.Send(context.Background(), ch, credentials, model.Event{ID: "one"}); err != nil {
					t.Fatal(err)
				}
			}
			send()
			send()
			if auths != 1 {
				t.Fatal("uncached token")
			}
			client.tokenMu.Lock()
			cached := client.tokens[ch]
			cached.expires = time.Now().Add(-time.Second)
			client.tokens[ch] = cached
			client.tokenMu.Unlock()
			send()
			if auths != 2 {
				t.Fatal("expired token reused")
			}
			credentials.AppSecret = "replacement-secret"
			send()
			if auths != 3 {
				t.Fatal("credentials shared a token")
			}
			reject = true
			send()
			if auths != 4 {
				t.Fatal("rejected token not refreshed")
			}
		})
	}
}

func TestDirectResponsesDoNotReportFalseDelivery(t *testing.T) {
	cases := []struct {
		ch    model.Channel
		body  string
		retry bool
	}{
		{model.Feishu, `{"code":0}`, false}, {model.Lark, `{}`, false}, {model.Feishu, `{"code":230013,"msg":"private-secret"}`, false},
		{model.DingTalk, `{}`, false}, {model.DingTalk, `{"processQueryKey":"x","invalidStaffIdList":["private-secret"]}`, false},
		{model.DingTalk, `{"processQueryKey":"x","filteredStaffIdList":["private-secret"]}`, false}, {model.DingTalk, `{"processQueryKey":"x","flowControlledStaffIdList":["private-secret"]}`, true},
		{model.Slack, `{"ok":true}`, false}, {model.Slack, `{"ok":true,"channel":"C123","ts":"1"}`, false}, {model.Slack, `{"ok":false,"error":"missing_scope"}`, false},
		{model.Slack, `{"ok":false,"error":"rate_limited"}`, true}, {model.Slack, `{"ok":false,"error":"private-secret"}`, false},
	}
	for _, tc := range cases {
		err := validateDirectResponse(tc.ch, apiReply{status: 200, body: []byte(tc.body)})
		if err == nil || strings.Contains(err.Error(), "private-secret") {
			t.Fatal("false success or leaked response", tc.ch)
		}
		if _, retry := Retry(err, 1); retry != tc.retry {
			t.Fatal("wrong retry policy", err)
		}
	}
}

func TestPrivateValidationCannotAddressGroupsOrMultipleUsers(t *testing.T) {
	for _, ch := range model.Channels {
		c := directCredentials(ch)
		c.RecipientID = "person1,person2"
		if Validate(ch, c) == nil {
			t.Fatal("multiple recipients accepted", ch)
		}
	}
	for _, id := range []string{"-10012345", "@group", "0"} {
		c := directCredentials(model.Telegram)
		c.RecipientID = id
		if Validate(model.Telegram, c) == nil {
			t.Fatal("group accepted as a personal chat")
		}
	}
	c := directCredentials(model.Feishu)
	c.RecipientType = "chat_id"
	c.RecipientID = "oc_example"
	if Validate(model.Feishu, c) == nil {
		t.Fatal("group accepted as a person")
	}
	c = directCredentials(model.Slack)
	c.Token = "xoxp-personal-user-token"
	if Validate(model.Slack, c) == nil {
		t.Fatal("non-bot identity accepted")
	}
}

func TestPrivateAuthFailuresNeverSendOrExposeSecrets(t *testing.T) {
	for _, body := range []string{`{"code":20002,"error_description":"private-secret"}`, `{}`, `{"code":0,"access_token":"x","expires_in":0}`, `{"code":0,"access_token":"x\r\ninjected","expires_in":7200}`} {
		calls := 0
		client := NewClient(roundTrip(func(r *http.Request) (*http.Response, error) {
			calls++
			if !isAuth(r) {
				t.Fatal("message sent without authentication")
			}
			return reply(200, body), nil
		}))
		err := client.Send(context.Background(), model.Feishu, directCredentials(model.Feishu), model.Event{})
		if err == nil || calls != 1 || strings.Contains(err.Error(), "private-secret") {
			t.Fatal("invalid auth response accepted")
		}
	}
	client := NewClient(roundTrip(func(*http.Request) (*http.Response, error) { return nil, errors.New("network leaked private-secret") }))
	err := client.Send(context.Background(), model.Slack, directCredentials(model.Slack), model.Event{})
	if err == nil || strings.Contains(err.Error(), "private-secret") {
		t.Fatal("transport secrets exposed")
	}
}

func TestPrivateCredentialsDoNotFollowRedirects(t *testing.T) {
	calls := 0
	client := NewClient(roundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls > 1 {
			t.Fatal("credential forwarded")
		}
		response := reply(307, "")
		response.Header.Set("Location", "https://attacker.invalid/collect")
		return response, nil
	}))
	if err := client.Send(context.Background(), model.Slack, directCredentials(model.Slack), model.Event{}); err == nil {
		t.Fatal("redirect accepted")
	}
}
