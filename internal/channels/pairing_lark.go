package channels

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"time"

	"callbell/internal/model"
	"github.com/coder/websocket"
	"google.golang.org/protobuf/encoding/protowire"
)

// Wire field numbers and heartbeat/ACK conventions follow the official SDK:
// https://github.com/larksuite/oapi-sdk-go/tree/v3_main/ws
// This small pairing-only receiver avoids the full event/service SDK and logs.
type larkFrame struct {
	method   uint64
	headers  map[string]string
	payload  []byte
	envelope []byte // routing fields, including unknown fields, echoed in ACKs
}

func decodeLarkFrame(data []byte) (larkFrame, error) {
	f := larkFrame{headers: map[string]string{}}
	for len(data) > 0 {
		num, typ, n := protowire.ConsumeTag(data)
		if n < 0 {
			return f, errors.New("invalid frame")
		}
		m := protowire.ConsumeFieldValue(num, typ, data[n:])
		if m < 0 {
			return f, errors.New("invalid frame")
		}
		switch num {
		case 5, 8:
			if typ != protowire.BytesType {
				return f, errors.New("invalid payload")
			}
			value, _ := protowire.ConsumeBytes(data[n : n+m])
			if num == 8 {
				f.payload = value
			} else {
				key, value, err := decodeLarkHeader(value)
				if err != nil {
					return f, err
				}
				f.headers[key] = value
			}
		default:
			if num == 4 {
				if typ != protowire.VarintType {
					return f, errors.New("invalid method")
				}
				f.method, _ = protowire.ConsumeVarint(data[n : n+m])
			}
			f.envelope = append(f.envelope, data[:n+m]...)
		}
		data = data[n+m:]
	}
	return f, nil
}

func decodeLarkHeader(data []byte) (key, value string, err error) {
	for len(data) > 0 {
		num, typ, n := protowire.ConsumeTag(data)
		if n < 0 || typ != protowire.BytesType {
			return "", "", errors.New("invalid header")
		}
		v, m := protowire.ConsumeBytes(data[n:])
		if m < 0 {
			return "", "", errors.New("invalid header")
		}
		if num == 1 {
			key = string(v)
		} else if num == 2 {
			value = string(v)
		}
		data = data[n+m:]
	}
	return
}

func protoBytes(dst []byte, field protowire.Number, value []byte) []byte {
	return protowire.AppendBytes(protowire.AppendTag(dst, field, protowire.BytesType), value)
}

func (f larkFrame) encode() []byte {
	data := append([]byte(nil), f.envelope...)
	for key, value := range f.headers {
		h := protoBytes(nil, 1, []byte(key))
		h = protoBytes(h, 2, []byte(value))
		data = protoBytes(data, 5, h)
	}
	return protoBytes(data, 8, f.payload)
}

func larkPing(service uint64) []byte {
	f := larkFrame{headers: map[string]string{"type": "ping"}}
	for i, value := range []uint64{0, 0, service, 0} {
		f.envelope = protowire.AppendVarint(protowire.AppendTag(f.envelope, protowire.Number(i+1), protowire.VarintType), value)
	}
	return f.encode()
}

type larkParts struct {
	parts [][]byte
	size  int
	until time.Time
}

func assembleLark(parts map[string]*larkParts, f larkFrame) ([]byte, error) {
	sum, _ := strconv.Atoi(f.headers["sum"])
	if sum <= 1 {
		return f.payload, nil
	}
	seq, err := strconv.Atoi(f.headers["seq"])
	id := f.headers["message_id"]
	if err != nil || sum > 64 || seq < 0 || seq >= sum || id == "" {
		return nil, errors.New("invalid fragments")
	}
	for key, p := range parts {
		if time.Now().After(p.until) {
			delete(parts, key)
		}
	}
	p := parts[id]
	if p == nil {
		if len(parts) >= 16 {
			return nil, errors.New("too many fragments")
		}
		p = &larkParts{parts: make([][]byte, sum), until: time.Now().Add(5 * time.Second)}
		parts[id] = p
	}
	if len(p.parts) != sum {
		return nil, errors.New("inconsistent fragments")
	}
	p.size += len(f.payload) - len(p.parts[seq])
	if p.size > 1024*1024 {
		return nil, errors.New("message too large")
	}
	p.parts[seq] = f.payload
	for _, part := range p.parts {
		if part == nil {
			return nil, nil
		}
	}
	delete(parts, id)
	return bytes.Join(p.parts, nil), nil
}

func (c *Client) pairLark(ctx context.Context, ch model.Channel, credentials model.Credentials, code string, since time.Time, ready func(string)) (PairResult, error) {
	r, err := c.apiPost(ctx, larkHost(ch)+"/callback/ws/endpoint", "", false, map[string]string{"AppID": credentials.AppID, "AppSecret": credentials.AppSecret})
	if err != nil {
		return PairResult{}, err
	}
	var bootstrap struct {
		Code *int `json:"code"`
		Data struct {
			URL          string                     `json:"URL"`
			ClientConfig struct{ PingInterval int } `json:"ClientConfig"`
		} `json:"data"`
	}
	if r.status != 200 || json.Unmarshal(r.body, &bootstrap) != nil || bootstrap.Code == nil || *bootstrap.Code != 0 {
		return PairResult{}, invalid("Cannot start pairing. Check the app credentials and enable long-connection message events in Setup help.")
	}
	conn, err := c.pairSocket(ctx, ch, bootstrap.Data.URL)
	if err != nil {
		return PairResult{}, err
	}
	defer conn.CloseNow()
	u, _ := url.Parse(bootstrap.Data.URL)
	service, err := strconv.ParseUint(u.Query().Get("service_id"), 10, 32)
	if err != nil {
		return PairResult{}, pairConnectionError()
	}
	ping := larkPing(service)
	if err = conn.Write(ctx, websocket.MessageBinary, ping); err != nil {
		return PairResult{}, pairConnectionError()
	}
	heartbeatCtx, stop := context.WithCancel(ctx)
	done := make(chan struct{})
	defer func() { stop(); <-done }()
	interval := time.Duration(max(10, min(90, bootstrap.Data.ClientConfig.PingInterval))) * time.Second
	go func() {
		defer close(done)
		for pairWait(heartbeatCtx, interval) == nil {
			if conn.Write(heartbeatCtx, websocket.MessageBinary, ping) != nil {
				conn.CloseNow()
				return
			}
		}
	}()
	ready("")
	parts := map[string]*larkParts{}
	for {
		kind, raw, err := conn.Read(ctx)
		if err != nil {
			return PairResult{}, pairConnectionError()
		}
		if kind != websocket.MessageBinary {
			continue
		}
		frame, err := decodeLarkFrame(raw)
		if err != nil {
			return PairResult{}, pairConnectionError()
		}
		if frame.method != 1 || frame.headers["type"] != "event" {
			continue
		}
		payload, err := assembleLark(parts, frame)
		if err != nil {
			return PairResult{}, pairConnectionError()
		}
		if payload == nil {
			continue
		}
		result := matchLarkPair(payload, credentials.AppID, code, since)
		frame.payload = []byte(`{"code":200,"headers":{},"data":null}`)
		frame.headers["biz_rt"] = "0"
		if conn.Write(ctx, websocket.MessageBinary, frame.encode()) != nil {
			return PairResult{}, pairConnectionError()
		}
		if result.RecipientID != "" {
			return result, nil
		}
	}
}

func matchLarkPair(raw []byte, appID, code string, since time.Time) PairResult {
	var data struct {
		Header struct {
			AppID     string `json:"app_id"`
			EventType string `json:"event_type"`
		} `json:"header"`
		Event struct {
			Sender struct {
				Type string `json:"sender_type"`
				ID   struct {
					OpenID string `json:"open_id"`
				} `json:"sender_id"`
			} `json:"sender"`
			Message struct {
				ChatType string `json:"chat_type"`
				Type     string `json:"message_type"`
				Content  string `json:"content"`
				Time     string `json:"create_time"`
			} `json:"message"`
		} `json:"event"`
	}
	if json.Unmarshal(raw, &data) != nil || data.Header.AppID != appID || data.Header.EventType != "im.message.receive_v1" || data.Event.Sender.Type != "user" || data.Event.Message.ChatType != "p2p" || data.Event.Message.Type != "text" {
		return PairResult{}
	}
	stamp, err := strconv.ParseInt(data.Event.Message.Time, 10, 64)
	var content struct {
		Text string `json:"text"`
	}
	if err != nil || !freshPairTime(time.UnixMilli(stamp), since) || json.Unmarshal([]byte(data.Event.Message.Content), &content) != nil || content.Text != code {
		return PairResult{}
	}
	return PairResult{RecipientID: data.Event.Sender.ID.OpenID, RecipientType: "open_id"}
}
