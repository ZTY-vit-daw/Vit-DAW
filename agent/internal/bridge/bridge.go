package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-zeromq/zmq4"

	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/logx"
	"vit-daw-agent/internal/shadow"
)

type Config struct {
	ZMQSubURL      string
	GodotIP        string
	UDPToGodot     int
	UDPFromGodot   int
	ReqMaxRetries  int
	ReqTimeout     time.Duration
	TelemetryRetry time.Duration
	FileReplyDir   string
	TelemetryHook  func(map[string]any)
}

type Bridge struct {
	cfg    Config
	kernel *kernel.Client
	shadow *shadow.Project
	logger *logx.Logger
}

func New(cfg Config, kernelClient *kernel.Client, shadowProject *shadow.Project, logger *logx.Logger) *Bridge {
	if strings.TrimSpace(cfg.ZMQSubURL) == "" {
		cfg.ZMQSubURL = "tcp://127.0.0.1:5556"
	}
	if strings.TrimSpace(cfg.GodotIP) == "" {
		cfg.GodotIP = "127.0.0.1"
	}
	if cfg.UDPToGodot == 0 {
		cfg.UDPToGodot = 4444
	}
	if cfg.UDPFromGodot == 0 {
		cfg.UDPFromGodot = 4445
	}
	if cfg.ReqMaxRetries < 0 {
		cfg.ReqMaxRetries = 0
	}
	if cfg.TelemetryRetry <= 0 {
		cfg.TelemetryRetry = time.Second
	}
	if strings.TrimSpace(cfg.FileReplyDir) == "" {
		cfg.FileReplyDir = defaultFileReplyDir()
	}
	return &Bridge{cfg: cfg, kernel: kernelClient, shadow: shadowProject, logger: logger}
}

const maxDirectUDPReplyBytes = 32 * 1024

func (b *Bridge) Run(ctx context.Context) error {
	refreshCh := make(chan struct{}, 1)
	errCh := make(chan error, 2)
	go func() { errCh <- b.runTelemetry(ctx, refreshCh) }()
	go func() { errCh <- b.runShadowRefresh(ctx, refreshCh) }()
	go func() { errCh <- b.runControl(ctx) }()

	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return err
	}
}

func (b *Bridge) runControl(ctx context.Context) error {
	addr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", b.cfg.GodotIP, b.cfg.UDPFromGodot))
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	if b.logger != nil {
		b.logger.Info("control loop started: UDP:%d -> %s", b.cfg.UDPFromGodot, b.kernel.Endpoint)
	}

	buf := make([]byte, 65535)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		_ = conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		n, remote, err := conn.ReadFromUDP(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			if strings.Contains(strings.ToLower(err.Error()), "forcibly closed") {
				continue
			}
			return err
		}
		if n <= 0 {
			continue
		}
		payload := string(buf[:n])
		parsed := parseObject(payload)
		reply := b.forwardCommand(ctx, payload)
		b.writeControlReply(conn, remote, parsed, reply)
	}
}

func (b *Bridge) writeControlReply(conn *net.UDPConn, remote *net.UDPAddr, request map[string]any, reply string) {
	out, spilled, err := b.controlReplyPayload(request, reply)
	if err != nil && b.logger != nil {
		b.logger.Warn("control reply file fallback failed; attempting direct UDP bytes=%d error=%v", len(reply), err)
	}
	n, writeErr := conn.WriteToUDP(out, remote)
	if writeErr == nil {
		if n != len(out) && b.logger != nil {
			b.logger.Warn("control UDP send wrote partial reply bytes=%d/%d", n, len(out))
		}
		return
	}
	if spilled {
		if b.logger != nil {
			b.logger.Warn("control UDP send failed for file reply envelope bytes=%d error=%v", len(out), writeErr)
		}
		return
	}
	fallback, _, fallbackErr := b.spillControlReply(request, []byte(reply))
	if fallbackErr != nil {
		if b.logger != nil {
			b.logger.Warn("control UDP send failed and file fallback failed direct_bytes=%d send_error=%v fallback_error=%v", len(reply), writeErr, fallbackErr)
		}
		return
	}
	if _, retryErr := conn.WriteToUDP(fallback, remote); retryErr != nil && b.logger != nil {
		b.logger.Warn("control UDP file reply retry failed envelope_bytes=%d error=%v", len(fallback), retryErr)
	}
}

func (b *Bridge) controlReplyPayload(request map[string]any, reply string) ([]byte, bool, error) {
	out := []byte(reply)
	if len(out) <= maxDirectUDPReplyBytes {
		return out, false, nil
	}
	return b.spillControlReply(request, out)
}

func (b *Bridge) spillControlReply(request map[string]any, reply []byte) ([]byte, bool, error) {
	dir := strings.TrimSpace(b.cfg.FileReplyDir)
	if dir == "" {
		dir = defaultFileReplyDir()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return reply, false, err
	}
	requestID := strings.TrimSpace(fmt.Sprint(request["request_id"]))
	command := commandName(request)
	filename := fmt.Sprintf("%s_%s_%s.json",
		time.Now().Format("20060102_150405_000000000"),
		sanitizeFileComponent(command, "command"),
		sanitizeFileComponent(requestID, "request"))
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, reply, 0o644); err != nil {
		return reply, false, err
	}
	envelope := map[string]any{
		"status":      "ok",
		"transport":   "file_reply",
		"reply_file":  filepath.ToSlash(path),
		"reply_bytes": len(reply),
		"cmd":         command,
		"request_id":  requestID,
	}
	out, err := json.Marshal(envelope)
	if err != nil {
		return reply, false, err
	}
	if b.logger != nil {
		b.logger.Info("control reply spilled to file cmd=%s request_id=%s bytes=%d file=%s", command, requestID, len(reply), path)
	}
	return out, true, nil
}

func (b *Bridge) forwardCommand(ctx context.Context, payload string) string {
	parsed := parseObject(payload)
	var reply string
	var err error
	for i := 0; i <= b.cfg.ReqMaxRetries; i++ {
		reply, err = b.kernel.SendRaw(ctx, payload)
		if err == nil {
			break
		}
	}
	if err != nil {
		if b.logger != nil {
			b.logger.Warn("kernel request failed: %v", err)
		}
		return kernel.ErrorReply("agent timeout to VitApp: " + err.Error())
	}
	if commandName(parsed) == "get_project_state" {
		b.initializeShadowFromReply(reply)
	}
	return reply
}

func (b *Bridge) runTelemetry(ctx context.Context, refreshCh chan<- struct{}) error {
	dest, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", b.cfg.GodotIP, b.cfg.UDPToGodot))
	if err != nil {
		return err
	}
	conn, err := net.DialUDP("udp4", nil, dest)
	if err != nil {
		return err
	}
	defer conn.Close()

	for {
		if err := b.telemetryOnce(ctx, conn, refreshCh); err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			}
			if b.logger != nil {
				b.logger.Warn("telemetry loop reconnecting after error: %v", err)
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(b.cfg.TelemetryRetry):
			}
			continue
		}
	}
}

func (b *Bridge) telemetryOnce(ctx context.Context, conn *net.UDPConn, refreshCh chan<- struct{}) error {
	sub := zmq4.NewSub(ctx, zmq4.WithTimeout(time.Second), zmq4.WithAutomaticReconnect(true))
	defer sub.Close()
	if err := sub.SetOption(zmq4.OptionSubscribe, ""); err != nil {
		return err
	}
	if err := sub.Dial(b.cfg.ZMQSubURL); err != nil {
		return err
	}
	if b.logger != nil {
		b.logger.Info("telemetry thread started: %s -> UDP:%d", b.cfg.ZMQSubURL, b.cfg.UDPToGodot)
	}

	var lastSeq int64
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		msg, err := sub.Recv()
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || strings.Contains(strings.ToLower(err.Error()), "timeout") {
				continue
			}
			return err
		}
		out := b.normalizeTelemetry(msgPayload(msg), refreshCh, &lastSeq)
		if len(out) == 0 {
			continue
		}
		if _, err := conn.Write(out); err != nil && b.logger != nil {
			b.logger.Warn("telemetry UDP send failed: %v", err)
		}
	}
}

func (b *Bridge) normalizeTelemetry(text string, refreshCh chan<- struct{}, lastSeq *int64) []byte {
	var d map[string]any
	if err := json.Unmarshal([]byte(text), &d); err != nil {
		return []byte(text)
	}
	if fmt.Sprint(d["type"]) == "delta_update" {
		if b.shadow != nil {
			b.shadow.ApplyDelta(d)
		}
		seq := int64From(d["seq_id"])
		if seq > 0 && lastSeq != nil {
			if *lastSeq > 0 && seq != *lastSeq+1 && b.logger != nil {
				b.logger.Warn("[telemetry] delta seq gap last=%d current=%d", *lastSeq, seq)
			}
			*lastSeq = seq
		}
	}
	if fmt.Sprint(d["topic"]) == "recording" && strings.TrimSpace(fmt.Sprint(d["subtopic"])) == "recording_stopped" {
		select {
		case refreshCh <- struct{}{}:
		default:
		}
	}
	if b.cfg.TelemetryHook != nil {
		b.cfg.TelemetryHook(d)
	}
	out, err := json.Marshal(d)
	if err != nil {
		return []byte(text)
	}
	return out
}

func (b *Bridge) runShadowRefresh(ctx context.Context, refreshCh <-chan struct{}) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-refreshCh:
			b.refreshShadow(ctx, "recording_stopped")
		}
	}
}

func (b *Bridge) refreshShadow(ctx context.Context, reason string) {
	reply, _, err := b.kernel.SendCommand(ctx, map[string]any{"cmd": "get_project_state"})
	if err != nil {
		if b.logger != nil {
			b.logger.Warn("[shadow] %s refresh failed: %v", reason, err)
		}
		return
	}
	if fmt.Sprint(reply["status"]) == "ok" {
		b.shadow.Initialize(reply)
		if b.logger != nil {
			b.logger.Info("[shadow] refreshed after %s", reason)
		}
	}
}

func (b *Bridge) initializeShadowFromReply(raw string) {
	var reply map[string]any
	if err := json.Unmarshal([]byte(raw), &reply); err != nil {
		return
	}
	if fmt.Sprint(reply["status"]) == "ok" && b.shadow != nil {
		b.shadow.Initialize(reply)
	}
}

func parseObject(payload string) map[string]any {
	var out map[string]any
	if err := json.Unmarshal([]byte(payload), &out); err != nil {
		return nil
	}
	return out
}

func commandName(d map[string]any) string {
	if d == nil {
		return ""
	}
	for _, key := range []string{"cmd", "action", "command"} {
		if v, ok := d[key]; ok {
			s := strings.TrimSpace(fmt.Sprint(v))
			if s != "" {
				return s
			}
		}
	}
	return ""
}

func defaultFileReplyDir() string {
	return filepath.Join(os.TempDir(), "vit_daw_agent_replies")
}

func sanitizeFileComponent(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		value = fallback
	}
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('_')
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return fallback
	}
	if len(out) > 80 {
		return out[:80]
	}
	return out
}

func int64From(v any) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case int:
		return int64(x)
	case float64:
		return int64(x)
	default:
		return 0
	}
}

func msgPayload(msg zmq4.Msg) string {
	if len(msg.Frames) == 0 {
		return ""
	}
	return string(msg.Frames[len(msg.Frames)-1])
}
