package channels

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"callbell/internal/model"
	"github.com/coder/websocket"
)

const fixtureCode = "nudilo-0123456789ab"

func fixturePairCredentials(ch model.Channel) model.Credentials {
	c := model.Credentials{Mode: model.Direct, AppID: "cli_example", AppSecret: "private-secret", Token: "xoxb-example"}
	if ch == model.Telegram {
		c.Token = "123456:abcdefghijklmnopqrstuvwxyz"
	}
	return c
}

func jsonResponse(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}

// Real TLS + WebSocket handshakes; only the transport's destination changes.
// Production endpoint checks, HTTP headers and protocol parsing remain active.
func pairTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	base := server.Client().Transport
	u, _ := url.Parse(server.URL)
	return NewClient(roundTrip(func(req *http.Request) (*http.Response, error) {
		copy := req.Clone(req.Context())
		copy.Header.Set("X-Test-Original-Host", req.URL.Host)
		copy.URL.Host, copy.URL.Scheme = u.Host, u.Scheme
		return base.RoundTrip(copy)
	}))
}

func larkEvent(at time.Time) map[string]any {
	return map[string]any{
		"schema": "2.0", "header": map[string]any{"app_id": "cli_example", "event_type": "im.message.receive_v1"},
		"event": map[string]any{
			"sender":  map[string]any{"sender_type": "user", "sender_id": map[string]any{"open_id": "ou_recipient"}},
			"message": map[string]any{"chat_type": "p2p", "message_type": "text", "create_time": fmt.Sprint(at.UnixMilli()), "content": `{"text":"` + fixtureCode + `"}`},
		},
	}
}

func dingEvent(at time.Time) map[string]any {
	return map[string]any{"conversationType": "1", "msgtype": "text", "createAt": at.UnixMilli(), "senderStaffId": "user123", "robotCode": "ding-robot", "senderCorpId": "corp", "chatbotCorpId": "corp", "text": map[string]any{"content": fixtureCode}}
}

func rawJSON(v any) []byte { data, _ := json.Marshal(v); return data }

func TestPairingRejectsWrongOriginCodeAndAge(t *testing.T) {
	now := time.Now()
	for _, mutate := range []func(map[string]any){
		func(d map[string]any) { d["header"].(map[string]any)["app_id"] = "another-app" },
		func(d map[string]any) { d["header"].(map[string]any)["event_type"] = "im.message.read_v1" },
		func(d map[string]any) { d["event"].(map[string]any)["message"].(map[string]any)["chat_type"] = "group" },
		func(d map[string]any) {
			d["event"].(map[string]any)["message"].(map[string]any)["content"] = `{"text":"wrong"}`
		},
		func(d map[string]any) {
			d["event"].(map[string]any)["message"].(map[string]any)["create_time"] = fmt.Sprint(now.Add(-time.Hour).UnixMilli())
		},
		func(d map[string]any) { d["event"].(map[string]any)["sender"].(map[string]any)["sender_type"] = "app" },
	} {
		d := larkEvent(now)
		mutate(d)
		if matchLarkPair(rawJSON(d), "cli_example", fixtureCode, now).RecipientID != "" {
			t.Fatal("accepted unrelated Feishu/Lark message")
		}
	}
	for _, field := range []string{"conversationType", "msgtype", "senderStaffId", "robotCode", "senderCorpId", "chatbotCorpId"} {
		d := dingEvent(now)
		d[field] = ""
		if matchDingPair(rawJSON(d), fixtureCode, now).RecipientID != "" {
			t.Fatal("accepted incomplete DingTalk message", field)
		}
	}
	for _, d := range []map[string]any{dingEvent(now.Add(-time.Hour)), dingEvent(now.Add(time.Hour))} {
		if matchDingPair(rawJSON(d), fixtureCode, now).RecipientID != "" {
			t.Fatal("accepted stale or future DingTalk message")
		}
	}
	d := dingEvent(now)
	d["text"] = map[string]string{"content": "wrong"}
	if matchDingPair(rawJSON(d), fixtureCode, now).RecipientID != "" {
		t.Fatal("wrong DingTalk code")
	}
}

func TestLarkAndFeishuPairingHandshakeFragmentsAndACK(t *testing.T) {
	for _, ch := range []model.Channel{model.Feishu, model.Lark} {
		t.Run(string(ch), func(t *testing.T) {
			now := time.Now()
			done := make(chan struct{})
			client := pairTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/callback/ws/endpoint" {
					var body map[string]string
					_ = json.NewDecoder(r.Body).Decode(&body)
					if body["AppID"] != "cli_example" || body["AppSecret"] != "private-secret" || "https://"+r.Header.Get("X-Test-Original-Host") != larkHost(ch) {
						t.Error("incorrect bootstrap")
					}
					jsonResponse(w, map[string]any{"code": 0, "data": map[string]any{"URL": strings.Replace(larkHost(ch), "https://", "wss://", 1) + "/socket?service_id=9", "ClientConfig": map[string]int{"PingInterval": 30}}})
					return
				}
				defer close(done)
				conn, err := websocket.Accept(w, r, nil)
				if err != nil {
					t.Error(err)
					return
				}
				defer conn.CloseNow()
				ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
				defer cancel()
				kind, ping, err := conn.Read(ctx)
				if err != nil || kind != websocket.MessageBinary || !strings.Contains(string(ping), "ping") {
					t.Error("missing binary heartbeat", err)
					return
				}
				// Fixed protocol routing fields: SeqID=1, LogID=2, service=9,
				// method=1, type=event, LogIDNew=new-1 (unknown to our parser).
				prefix, _ := hex.DecodeString("08011002180920012a0d0a047479706512056576656e744a056e65772d31")
				payload := rawJSON(larkEvent(now))
				for i, part := range [][]byte{payload[:len(payload)/2], payload[len(payload)/2:]} {
					frame := append([]byte(nil), prefix...)
					for key, value := range map[string]string{"sum": "2", "seq": fmt.Sprint(i), "message_id": "fixture"} {
						frame = protoBytes(frame, 5, protoBytes(protoBytes(nil, 1, []byte(key)), 2, []byte(value)))
					}
					if err = conn.Write(ctx, websocket.MessageBinary, protoBytes(frame, 8, part)); err != nil {
						t.Error(err)
						return
					}
				}
				_, ack, err := conn.Read(ctx)
				if err != nil {
					t.Error("missing ACK", err)
					return
				}
				f, err := decodeLarkFrame(ack)
				if err != nil || !strings.Contains(string(f.envelope), "new-1") || string(f.payload) != `{"code":200,"headers":{},"data":null}` {
					t.Error("ACK lost routing or result")
				}
				if _, _, err = conn.Read(ctx); err == nil {
					t.Error("pairing connection left open")
				}
			})
			ready := false
			result, err := client.Pair(context.Background(), ch, fixturePairCredentials(ch), fixtureCode, now, func(string) { ready = true })
			if err != nil || !ready || result.RecipientID != "ou_recipient" || result.RecipientType != "open_id" {
				t.Fatal(result, err)
			}
			<-done
		})
	}
}

func TestDingTalkPairingPingAndAutomaticRobotCode(t *testing.T) {
	now := time.Now()
	done := make(chan struct{})
	client := pairTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1.0/gateway/connections/open" {
			var body struct {
				ClientID      string                         `json:"clientId"`
				Subscriptions []struct{ Type, Topic string } `json:"subscriptions"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.ClientID != "cli_example" || len(body.Subscriptions) != 1 || body.Subscriptions[0].Topic != dingMessageTopic {
				t.Error("incorrect DingTalk bootstrap")
			}
			jsonResponse(w, map[string]string{"endpoint": "wss://wss-open-connection.dingtalk.com/socket", "ticket": "short-lived"})
			return
		}
		defer close(done)
		if r.URL.Query().Get("ticket") != "short-lived" {
			t.Error("ticket missing")
		}
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.CloseNow()
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		for i, frame := range []map[string]any{
			{"type": "SYSTEM", "headers": map[string]string{"topic": "ping", "messageId": "ping-id"}, "data": "heartbeat"},
			{"type": "CALLBACK", "headers": map[string]string{"topic": dingMessageTopic, "messageId": "message-id"}, "data": string(rawJSON(dingEvent(now)))},
		} {
			if err = conn.Write(ctx, websocket.MessageText, rawJSON(frame)); err != nil {
				t.Error(err)
				return
			}
			_, raw, err := conn.Read(ctx)
			if err != nil {
				t.Error(err)
				return
			}
			var ack struct {
				Code    int               `json:"code"`
				Headers map[string]string `json:"headers"`
				Data    string            `json:"data"`
			}
			_ = json.Unmarshal(raw, &ack)
			if ack.Code != 200 || ack.Headers["messageId"] != frame["headers"].(map[string]string)["messageId"] || (i == 0 && ack.Data != "heartbeat") {
				t.Error("invalid ACK")
			}
		}
		if _, _, err = conn.Read(ctx); err == nil {
			t.Error("DingTalk connection left open")
		}
	})
	result, err := client.Pair(context.Background(), model.DingTalk, fixturePairCredentials(model.DingTalk), fixtureCode, now, nil)
	if err != nil || result.RecipientID != "user123" || result.RobotCode != "ding-robot" {
		t.Fatal(result, err)
	}
	<-done
}

func TestPairCancellationClosesSocket(t *testing.T) {
	done := make(chan struct{})
	client := pairTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1.0/gateway/connections/open" {
			jsonResponse(w, map[string]string{"endpoint": "wss://gateway.dingtalk.com/socket", "ticket": "secret"})
			return
		}
		defer close(done)
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.CloseNow()
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if _, _, err = conn.Read(ctx); err == nil {
			t.Error("did not close")
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	_, err := client.Pair(ctx, model.DingTalk, fixturePairCredentials(model.DingTalk), fixtureCode, time.Now(), func(string) { cancel() })
	if err != context.Canceled {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("socket leaked")
	}
}

func TestPairExpiryAndUntrustedSocketNeverDial(t *testing.T) {
	var calls atomic.Int32
	client := NewClient(roundTrip(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		return nil, fmt.Errorf("secret in URL %s", r.URL)
	}))
	_, err := client.Pair(context.Background(), model.Feishu, fixturePairCredentials(model.Feishu), fixtureCode, time.Now().Add(-PairDuration), nil)
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatal(err)
	}
	for _, endpoint := range []string{"ws://open.feishu.cn/socket", "wss://open.feishu.cn.evil.test/socket", "wss://user:secret@open.feishu.cn/socket", "wss://127.0.0.1/socket", "wss://open.feishu.cn:123/socket"} {
		before := calls.Load()
		_, err = client.pairSocket(context.Background(), model.Feishu, endpoint)
		if err == nil || calls.Load() != before || strings.Contains(err.Error(), "secret") {
			t.Fatal("unsafe endpoint", endpoint, err)
		}
	}
}
