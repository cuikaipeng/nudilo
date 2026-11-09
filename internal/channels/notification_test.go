package channels

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"callbell/internal/model"
)

func TestEveryDestinationUsesThePreparedNotification(t *testing.T) {
	p := model.Defaults()
	p.Language = "zh-Hans"
	p.SetNotificationContent("zh-Hans", model.NotificationContent{Title: "小助手", InputRequired: "项目 {project} 在等你回复。"})
	event := p.PrepareNotification(model.Event{Kind: model.InputRequired, Project: "中文项目", CreatedAt: time.Now()}, "en")
	for _, ch := range model.Channels {
		for _, mode := range []model.DeliveryMode{model.Group, model.Direct} {
			t.Run(string(ch)+"-"+string(mode), func(t *testing.T) {
				credentials := credential(ch)
				credentials.Mode = mode
				if mode == model.Direct {
					credentials = directCredentials(ch)
				} else if ch == model.Telegram {
					credentials.ChatID = "-10012345"
				}
				sends := 0
				client := NewClient(roundTrip(func(r *http.Request) (*http.Response, error) {
					if mode == model.Direct && ch != model.Telegram && ch != model.Slack && isAuth(r) {
						return reply(200, authResponse(ch)), nil
					}
					sends++
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Fatal(err)
					}
					var text string
					switch ch {
					case model.Feishu, model.Lark:
						if mode == model.Direct {
							var content map[string]string
							if err := json.Unmarshal([]byte(body["content"].(string)), &content); err != nil {
								t.Fatal(err)
							}
							text = content["text"]
						} else {
							text = body["content"].(map[string]any)["text"].(string)
						}
					case model.DingTalk:
						if mode == model.Direct {
							var content map[string]string
							if err := json.Unmarshal([]byte(body["msgParam"].(string)), &content); err != nil {
								t.Fatal(err)
							}
							text = content["content"]
						} else {
							text = body["text"].(map[string]any)["content"].(string)
						}
					default:
						text = body["text"].(string)
					}
					if text != event.Message() {
						t.Fatalf("transport replaced rendered text: %q", text)
					}
					response := directSuccess(ch)
					if mode == model.Group {
						switch ch {
						case model.DingTalk:
							response = `{"errcode":0}`
						case model.Slack:
							response = "ok"
						}
					}
					return reply(200, response), nil
				}))
				if err := client.Send(context.Background(), ch, credentials, event); err != nil {
					t.Fatal(err)
				}
				if sends != 1 {
					t.Fatal("expected one send", sends)
				}
			})
		}
	}
}
