package channels

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"time"

	"callbell/internal/model"
	"github.com/coder/websocket"
)

// Protocol reference: https://github.com/open-dingtalk/dingtalk-stream-sdk-go
const dingMessageTopic = "/v1.0/im/bot/messages/get"

func (c *Client) pairDingTalk(ctx context.Context, credentials model.Credentials, code string, since time.Time, ready func(string)) (PairResult, error) {
	r, err := c.apiPost(ctx, "https://api.dingtalk.com/v1.0/gateway/connections/open", "", false, map[string]any{
		"clientId": credentials.AppID, "clientSecret": credentials.AppSecret, "ua": "Nudilo/1.0",
		"subscriptions": []map[string]string{{"type": "CALLBACK", "topic": dingMessageTopic}},
	})
	if err != nil {
		return PairResult{}, err
	}
	var bootstrap struct {
		Endpoint string `json:"endpoint"`
		Ticket   string `json:"ticket"`
	}
	if r.status != 200 || json.Unmarshal(r.body, &bootstrap) != nil || bootstrap.Ticket == "" || !pairSocketURL(model.DingTalk, bootstrap.Endpoint) {
		return PairResult{}, invalid("Cannot start DingTalk pairing. Check Client ID, Client Secret and the robot's Stream mode.")
	}
	u, _ := url.Parse(bootstrap.Endpoint)
	q := u.Query()
	q.Set("ticket", bootstrap.Ticket)
	u.RawQuery = q.Encode()
	conn, err := c.pairSocket(ctx, model.DingTalk, u.String())
	if err != nil {
		return PairResult{}, err
	}
	defer conn.CloseNow()
	ready("")
	for {
		_, raw, err := conn.Read(ctx)
		if err != nil {
			return PairResult{}, pairConnectionError()
		}
		var frame struct {
			Type    string            `json:"type"`
			Headers map[string]string `json:"headers"`
			Data    string            `json:"data"`
		}
		if json.Unmarshal(raw, &frame) != nil || frame.Headers["messageId"] == "" {
			return PairResult{}, pairConnectionError()
		}
		result, status, response := PairResult{}, 200, ""
		switch {
		case frame.Type == "SYSTEM" && frame.Headers["topic"] == "ping":
			response = frame.Data
		case frame.Type == "SYSTEM" && frame.Headers["topic"] == "disconnect":
		case frame.Type == "CALLBACK" && frame.Headers["topic"] == dingMessageTopic:
			result = matchDingPair([]byte(frame.Data), code, since)
		default:
			status = 404
		}
		ack, _ := json.Marshal(map[string]any{"code": status, "message": "OK", "data": response, "headers": map[string]string{"messageId": frame.Headers["messageId"], "contentType": "application/json"}})
		if conn.Write(ctx, websocket.MessageText, ack) != nil {
			return PairResult{}, pairConnectionError()
		}
		if result.RecipientID != "" {
			return result, nil
		}
		if frame.Type == "SYSTEM" && frame.Headers["topic"] == "disconnect" {
			return PairResult{}, pairConnectionError()
		}
	}
}

func matchDingPair(raw []byte, code string, since time.Time) PairResult {
	var message struct {
		ConversationType string `json:"conversationType"`
		MsgType          string `json:"msgtype"`
		CreatedAt        int64  `json:"createAt"`
		SenderID         string `json:"senderStaffId"`
		RobotCode        string `json:"robotCode"`
		SenderCorp       string `json:"senderCorpId"`
		BotCorp          string `json:"chatbotCorpId"`
		Text             struct {
			Content string `json:"content"`
		} `json:"text"`
	}
	if json.Unmarshal(raw, &message) != nil || message.ConversationType != "1" || message.MsgType != "text" || strings.TrimSpace(message.Text.Content) != code || !freshPairTime(time.UnixMilli(message.CreatedAt), since) || message.SenderCorp == "" || message.SenderCorp != message.BotCorp || !opaqueID.MatchString(message.SenderID) || !opaqueID.MatchString(message.RobotCode) {
		return PairResult{}
	}
	return PairResult{RecipientID: message.SenderID, RobotCode: message.RobotCode}
}
