package channels

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"callbell/internal/model"
)

func mockReply(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
}

func TestTelegramPairingBotLinkAndPrivateStart(t *testing.T) {
	now := time.Now()
	calls := 0
	client := NewClient(roundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Method != "GET" || r.URL.Query().Has("offset") || r.URL.Query().Has("allowed_updates") {
			t.Fatal("mutates Telegram updates")
		}
		if strings.HasSuffix(r.URL.Path, "getMe") {
			return mockReply(200, `{"ok":true,"result":{"is_bot":true,"username":"NudiloBot"}}`), nil
		}
		updates := []any{}
		for _, tc := range []struct {
			id           int
			kind, text   string
			stamp        int64
			bot, forward bool
		}{
			{1, "group", fixtureCode, now.Unix(), false, false},
			{2, "private", fixtureCode, now.Add(-time.Hour).Unix(), false, false},
			{3, "private", fixtureCode, now.Unix(), true, false},
			{4, "private", fixtureCode, now.Unix(), false, true},
			{5, "private", "wrong-code", now.Unix(), false, false},
			{6, "private", "/start " + fixtureCode, now.Unix(), false, false},
		} {
			message := map[string]any{"date": tc.stamp, "text": tc.text, "chat": map[string]any{"id": tc.id, "type": tc.kind}, "from": map[string]any{"id": tc.id, "is_bot": tc.bot}}
			if tc.forward {
				message["forward_origin"] = map[string]string{"type": "user"}
			}
			updates = append(updates, map[string]any{"message": message})
		}
		return mockReply(200, string(rawJSON(map[string]any{"ok": true, "result": updates}))), nil
	}))
	link := ""
	result, err := client.Pair(context.Background(), model.Telegram, fixturePairCredentials(model.Telegram), fixtureCode, now, func(s string) { link = s })
	if err != nil || result.RecipientID != "6" || calls != 2 || link != "https://t.me/NudiloBot?start="+fixtureCode {
		t.Fatal(result, err, link, calls)
	}
}

func TestTelegramPairingConflictAndDuplicateRecipients(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		status         int
		body, expected string
	}{
		{409, `{"ok":false,"description":"private token must not escape"}`, "Webhook"},
		{200, fmt.Sprintf(`{"ok":true,"result":[{"message":{"date":%d,"text":"%s","chat":{"id":1,"type":"private"},"from":{"id":1}}},{"message":{"date":%d,"text":"%s","chat":{"id":2,"type":"private"},"from":{"id":2}}}]}`, now.Unix(), fixtureCode, now.Unix(), fixtureCode), "Multiple accounts"},
	} {
		client := NewClient(roundTrip(func(*http.Request) (*http.Response, error) { return mockReply(tc.status, tc.body), nil }))
		_, err := client.pollTelegramPair(context.Background(), fixturePairCredentials(model.Telegram).Token, fixtureCode, now)
		if err == nil || !strings.Contains(err.Error(), tc.expected) || strings.Contains(err.Error(), "private token") {
			t.Fatal(err)
		}
	}
}

func TestSlackPairingPaginatesPrivateConversationsAndHistory(t *testing.T) {
	now := time.Now()
	listPages, historyPages := 0, 0
	client := NewClient(roundTrip(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "Bearer xoxb-example" || r.Method != "GET" {
			t.Fatal("incorrect bot authorization")
		}
		q := r.URL.Query()
		if strings.HasSuffix(r.URL.Path, "conversations.list") {
			listPages++
			if q.Get("types") != "im" {
				t.Fatal("reads group conversations")
			}
			if q.Get("cursor") == "" {
				return mockReply(200, `{"ok":true,"channels":[{"id":"Cgroup","is_im":false,"user":"UOTHER"}],"response_metadata":{"next_cursor":"page2"}}`), nil
			}
			return mockReply(200, `{"ok":true,"channels":[{"id":"Dprivate","is_im":true,"user":"U123456"}],"response_metadata":{"next_cursor":""}}`), nil
		}
		historyPages++
		if q.Get("channel") != "Dprivate" || q.Get("oldest") == "" {
			t.Fatal("unbounded history read")
		}
		if q.Get("cursor") == "" {
			return mockReply(200, `{"ok":true,"messages":[],"has_more":true,"response_metadata":{"next_cursor":"older-page"}}`), nil
		}
		return mockReply(200, fmt.Sprintf(`{"ok":true,"messages":[{"type":"message","user":"U123456","text":"%s","ts":"%d.000001"}],"has_more":false}`, fixtureCode, now.Unix())), nil
	}))
	result, err := client.Pair(context.Background(), model.Slack, fixturePairCredentials(model.Slack), fixtureCode, now, nil)
	if err != nil || result.RecipientID != "U123456" || listPages != 2 || historyPages != 2 {
		t.Fatal(result, err, listPages, historyPages)
	}
}

func TestSlackPairingDoesNotAcceptBotEditsOrOtherUsers(t *testing.T) {
	now := time.Now()
	valid := slackPairMessage{Type: "message", User: "U123456", Text: fixtureCode, TS: fmt.Sprint(now.Unix()) + ".000001"}
	if !matchSlackPair(valid, "U123456", fixtureCode, now) {
		t.Fatal("valid message rejected")
	}
	for _, mutate := range []func(*slackPairMessage){
		func(m *slackPairMessage) { m.User = "UOTHER" }, func(m *slackPairMessage) { m.BotID = "B123" },
		func(m *slackPairMessage) { m.Subtype = "bot_message" }, func(m *slackPairMessage) { m.Edited = []byte(`{}`) },
		func(m *slackPairMessage) { m.TS = "1.0" }, func(m *slackPairMessage) { m.Text = "wrong" },
	} {
		m := valid
		mutate(&m)
		if matchSlackPair(m, "U123456", fixtureCode, now) {
			t.Fatal("unrelated message accepted", m)
		}
	}
}

func TestPollingPermissionErrorsAndRateLimitCancellation(t *testing.T) {
	client := NewClient(roundTrip(func(*http.Request) (*http.Response, error) {
		return mockReply(200, `{"ok":false,"error":"missing_scope","token":"secret"}`), nil
	}))
	_, err := client.Pair(context.Background(), model.Slack, fixturePairCredentials(model.Slack), fixtureCode, time.Now(), nil)
	if err == nil || !strings.Contains(err.Error(), "im:history") || strings.Contains(err.Error(), "secret") {
		t.Fatal(err)
	}
	client = NewClient(roundTrip(func(*http.Request) (*http.Response, error) {
		r := mockReply(429, `{}`)
		r.Header.Set("Retry-After", "120")
		return r, nil
	}))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err = client.Pair(ctx, model.Slack, fixturePairCredentials(model.Slack), fixtureCode, time.Now(), nil)
	if err == nil || time.Since(started) > time.Second {
		t.Fatal("rate-limited pairing did not cancel", err)
	}
}
