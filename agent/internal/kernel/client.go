package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-zeromq/zmq4"
)

type Client struct {
	Endpoint     string
	Timeout      time.Duration
	mu           sync.Mutex
	sessionID    string
	featureFlags map[string]bool
}

func New(endpoint string, timeout time.Duration) *Client {
	if strings.TrimSpace(endpoint) == "" {
		endpoint = "tcp://127.0.0.1:5555"
	}
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	return &Client{Endpoint: endpoint, Timeout: timeout, featureFlags: map[string]bool{}}
}

func (c *Client) SendRaw(ctx context.Context, payload string) (string, error) {
	if c == nil {
		return "", fmt.Errorf("kernel client is nil")
	}
	opCtx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()

	sock := zmq4.NewReq(opCtx, zmq4.WithTimeout(c.Timeout), zmq4.WithDialerTimeout(2*time.Second), zmq4.WithDialerMaxRetries(1))
	defer sock.Close()

	if err := sock.Dial(c.Endpoint); err != nil {
		return "", fmt.Errorf("kernel dial %s: %w", c.Endpoint, err)
	}
	if err := sock.Send(zmq4.NewMsgString(payload)); err != nil {
		return "", fmt.Errorf("kernel send: %w", err)
	}
	reply, err := sock.Recv()
	if err != nil {
		return "", fmt.Errorf("kernel recv: %w", err)
	}
	return msgPayload(reply), nil
}

func (c *Client) SendCommand(ctx context.Context, cmd map[string]any) (map[string]any, string, error) {
	payload, err := json.Marshal(cmd)
	if err != nil {
		return nil, "", fmt.Errorf("marshal command: %w", err)
	}
	raw, err := c.SendRaw(ctx, string(payload))
	if err != nil {
		return nil, "", err
	}
	var reply map[string]any
	if err := json.Unmarshal([]byte(raw), &reply); err != nil {
		return nil, raw, fmt.Errorf("decode kernel reply: %w", err)
	}
	return reply, raw, nil
}

func ErrorReply(message string) string {
	b, _ := json.Marshal(map[string]any{
		"status":  "error",
		"message": message,
	})
	return string(b)
}

func msgPayload(msg zmq4.Msg) string {
	if len(msg.Frames) == 0 {
		return ""
	}
	return string(msg.Frames[len(msg.Frames)-1])
}
