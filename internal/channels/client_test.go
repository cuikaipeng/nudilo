package channels

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"callbell/internal/model"
)

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func credential(ch model.Channel) model.Credentials {
	switch ch {
	case model.Feishu:
		return model.Credentials{Webhook: "https://open.feishu.cn/open-apis/bot/v2/hook/example", Secret: "secret"}
	case model.Lark:
		return model.Credentials{Webhook: "https://open.larksuite.com/open-apis/bot/v2/hook/example", Secret: "secret"}
	case model.DingTalk:
		return model.Credentials{Webhook: "https://oapi.dingtalk.com/robot/send?access_token=example", Secret: "secret"}
	case model.Slack:
		return model.Credentials{Webhook: "https://hooks.slack.com/services/T/B/example"}
	default:
		return model.Credentials{Token: "123456:abcdefghijklmnopqrstuvwxyz", ChatID: "12345"}
	}
}

func TestAllProvidersPayloadsAndAcknowledgments(t *testing.T) {
	for _, ch := range model.Channels {
		t.Run(string(ch), func(t *testing.T) {
			client := NewClient(roundTrip(func(r *http.Request) (*http.Response, error) {
				if r.Method != "POST" || r.URL.Scheme != "https" {
					t.Fatal("unsafe request")
				}
				b, _ := io.ReadAll(r.Body)
				if !strings.Contains(string(b), "Codex") {
					t.Fatal("missing notification text")
				}
				var body map[string]any
				if err := json.Unmarshal(b, &body); err != nil {
					t.Fatal(err)
				}
				response := `{"code":0}`
				switch ch {
				case model.Feishu, model.Lark:
					if body["sign"] == nil || body["timestamp"] == nil {
						t.Fatal("missing signature")
					}
				case model.DingTalk:
					response = `{"errcode":0}`
					if r.URL.Query().Get("sign") == "" {
						t.Fatal("missing signature")
					}
				case model.Slack:
					response = "ok"
					if body["mrkdwn"] != false {
						t.Fatal("unescaped markup")
					}
				case model.Telegram:
					response = `{"ok":true}`
					if body["parse_mode"] != nil || body["chat_id"] != "12345" {
						t.Fatal("unsafe telegram text")
					}
				}
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(response))}, nil
			}))
			if err := client.Send(context.Background(), ch, credential(ch), model.Event{Kind: model.Test, Project: "test", CreatedAt: time.Now()}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSignatures(t *testing.T) {
	now := time.Unix(1700000000, 0)
	event := model.Event{Kind: model.Test}
	req, _ := Request(context.Background(), model.Feishu, credential(model.Feishu), event, now)
	var body map[string]any
	_ = json.NewDecoder(req.Body).Decode(&body)
	// These constants are independently generated HMAC-SHA256 vectors.
	if body["sign"] != "fiWS2+gh28DOydAv7hzONH/mDn9+b1Y4Y5ivXWXy8vA=" {
		t.Fatal("Feishu signature does not match independent HMAC vector")
	}
	req, _ = Request(context.Background(), model.DingTalk, credential(model.DingTalk), event, now)
	if req.URL.Query().Get("timestamp") != "1700000000000" || req.URL.Query().Get("sign") != "OuzzJR5+xZ4/EYwqtNt6sMYZQMTa/HEGvc9miJe7XzY=" {
		t.Fatal("DingTalk signature does not match independent HMAC vector")
	}
}
func TestFailuresAndRetry(t *testing.T) {
	for _, tc := range []struct {
		ch     model.Channel
		status int
		body   string
	}{{model.Feishu, 200, `{"code":19021}`}, {model.DingTalk, 200, `{"errcode":310000}`}, {model.Telegram, 200, `{"ok":false,"error_code":400}`}, {model.Slack, 200, "invalid_payload"}, {model.Lark, 200, `{}`}, {model.Feishu, 302, ""}} {
		if ValidateResponse(tc.ch, tc.status, nil, []byte(tc.body)) == nil {
			t.Fatalf("false success: %+v", tc)
		}
	}
	err := ValidateResponse(model.Telegram, 429, http.Header{"Retry-After": []string{"120"}}, []byte(`{"parameters":{"retry_after":180}}`))
	if delay, ok := Retry(err, 1); !ok || delay != 180*time.Second {
		t.Fatal("rate limit ignored")
	}
	if _, ok := Retry(err, 3); ok {
		t.Fatal("retry unbounded")
	}
	if strings.Contains(err.Error(), "access_token") {
		t.Fatal("credential leaked")
	}
}
func TestRejectForeignWebhooks(t *testing.T) {
	for _, raw := range []string{"http://open.feishu.cn/open-apis/bot/v2/hook/x", "https://open.feishu.cn.evil.test/open-apis/bot/v2/hook/x", "https://user:secret@open.feishu.cn/open-apis/bot/v2/hook/x", "https://open.feishu.cn:8443/open-apis/bot/v2/hook/x"} {
		if Validate(model.Feishu, model.Credentials{Webhook: raw}) == nil {
			t.Fatal(raw)
		}
	}
}
func TestTelegramPairingDoesNotSelectUnrelatedChat(t *testing.T) {
	client := NewClient(roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.Query().Has("offset") {
			t.Fatal("acknowledges unrelated updates")
		}
		response := `{"ok":true,"result":[{"message":{"date":1700000000,"text":"/start","chat":{"id":1,"type":"private"}}},{"message":{"date":1700000000,"text":"callbell-abc","chat":{"id":2,"type":"group"}}},{"message":{"date":1699990000,"text":"callbell-abc","chat":{"id":3,"type":"private"}}},{"message":{"date":1700000000,"text":"callbell-abc","chat":{"id":4,"type":"private"},"from":{"id":4}}}]}`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(response))}, nil
	}))
	id, err := client.pollTelegramPair(context.Background(), credential(model.Telegram).Token, "callbell-abc", time.Unix(1700000000, 0))
	if err != nil || id != "4" {
		t.Fatalf("%s %v", id, err)
	}
}
