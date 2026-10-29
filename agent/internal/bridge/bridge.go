package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
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
	VSPHubURL      string
}

type Bridge struct {
	cfg                Config
	kernel             *kernel.Client
	shadow             *shadow.Project
	logger             *logx.Logger
	transportPublishCh chan map[string]any
	realtimePublishCh  chan map[string]any
	legacyDiag         legacyTelemetryDiag
	// shadowRefresh 默认指向 (*Bridge).refreshShadow；测试注入用（kernel.Client
	// 为具体类型，无法伪造 VSP 回包）。
	shadowRefresh func(ctx context.Context, reason string)
}

type legacyTelemetryDiag struct {
	enabled bool
	total   int64
	counts  map[string]int64
	bytes   map[string]int64
	lastLog time.Time
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
	b := &Bridge{cfg: cfg, kernel: kernelClient, shadow: shadowProject, logger: logger}
	b.shadowRefresh = b.refreshShadow
	if strings.TrimSpace(cfg.VSPHubURL) != "" {
		// Realtime streams are latest-only. Keep transport independent from the
		// heavier meter payloads so multi-track load cannot queue stale playheads.
		b.transportPublishCh = make(chan map[string]any, 1)
		b.realtimePublishCh = make(chan map[string]any, 1)
	}
	b.legacyDiag.enabled = envBool("VIT_BRIDGE_LEGACY_TELEMETRY_DIAG", false)
	return b
}

const maxDirectUDPReplyBytes = 32 * 1024
const maxDirectUDPTelemetryBytes = 32 * 1024

// FIX-TRACKLIST-SHADOW-FRESHNESS-1 B：外部（GUI/内核直连）结构变更只以 delta 到达时，
// shadow 的轨道集合保持陈旧。结构 delta 提示走该 reason；runShadowRefresh 对其去抖
// （突发 node_added 合并为一次权威快照刷新），recording_stopped 保持即时刷新。
const shadowRefreshReasonRecording = "recording_stopped"
const shadowRefreshReasonStructural = "structural_delta"
const structuralShadowRefreshDebounce = time.Second

func (b *Bridge) Run(ctx context.Context) error {
	refreshCh := make(chan string, 1)
	errCh := make(chan error, 3)
	if b.realtimePublishCh != nil {
		go b.runVSPRealtimePublisher(ctx)
		go b.runVSPTransportPublisher(ctx)
	}
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

const maxSpillFilenameAttempts = 100

// spillFileStamp renders local time as 20060102_150405_<9-digit nanoseconds>;
// the nanosecond segment must come from Nanosecond() — a "0" run inside a Go
// layout is a literal, not a fractional-second token (needs ".000..." after
// the seconds field), which is what pinned every spill stamp to ..._000000000.
func spillFileStamp(t time.Time) string {
	return fmt.Sprintf("%s_%09d", t.Format("20060102_150405"), t.Nanosecond())
}

// writeSpillFileExclusive creates <dir>/<filename> without ever overwriting:
// on an existing target it retries with _1, _2, ... appended before .json.
func writeSpillFileExclusive(dir, filename string, data []byte) (string, error) {
	base := strings.TrimSuffix(filename, ".json")
	var path string
	for attempt := 0; attempt < maxSpillFilenameAttempts; attempt++ {
		name := base
		if attempt > 0 {
			name = fmt.Sprintf("%s_%d", base, attempt)
		}
		path = filepath.Join(dir, name+".json")
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			if os.IsExist(err) {
				continue
			}
			return path, err
		}
		_, writeErr := f.Write(data)
		closeErr := f.Close()
		if writeErr != nil {
			return path, writeErr
		}
		return path, closeErr
	}
	return path, fmt.Errorf("spill file %s: no free name after %d collisions", filename, maxSpillFilenameAttempts)
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
		spillFileStamp(time.Now()),
		sanitizeFileComponent(command, "command"),
		sanitizeFileComponent(requestID, "request"))
	path, err := writeSpillFileExclusive(dir, filename, reply)
	if err != nil {
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

func (b *Bridge) runTelemetry(ctx context.Context, refreshCh chan<- string) error {
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

func (b *Bridge) telemetryOnce(ctx context.Context, conn *net.UDPConn, refreshCh chan<- string) error {
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
		b.noteLegacyTelemetryUDP(out)
		wire, spilled, spillErr := b.telemetryPayload(out)
		if spillErr != nil && b.logger != nil {
			b.logger.Warn("telemetry file fallback failed; attempting direct UDP bytes=%d error=%v", len(out), spillErr)
		}
		if _, err := conn.Write(wire); err != nil && b.logger != nil {
			if spilled {
				b.logger.Warn("telemetry UDP send failed for file envelope bytes=%d error=%v", len(wire), err)
			} else {
				b.logger.Warn("telemetry UDP send failed bytes=%d error=%v", len(wire), err)
			}
		}
	}
}

func (b *Bridge) noteLegacyTelemetryUDP(packet []byte) {
	if b == nil || !b.legacyDiag.enabled || b.logger == nil {
		return
	}
	meta := parseObject(string(packet))
	label := telemetryPacketLabel(meta)
	if label == "" {
		label = "telemetry"
	}
	if b.legacyDiag.counts == nil {
		b.legacyDiag.counts = map[string]int64{}
	}
	if b.legacyDiag.bytes == nil {
		b.legacyDiag.bytes = map[string]int64{}
	}
	b.legacyDiag.total++
	b.legacyDiag.counts[label]++
	b.legacyDiag.bytes[label] += int64(len(packet))
	now := time.Now()
	if !b.legacyDiag.lastLog.IsZero() && now.Sub(b.legacyDiag.lastLog) < 2*time.Second && b.legacyDiag.total%250 != 0 {
		return
	}
	b.legacyDiag.lastLog = now
	b.logger.Info("[telemetry] legacy_udp diag total=%d last=%s last_bytes=%d top=%s",
		b.legacyDiag.total,
		label,
		len(packet),
		formatLegacyTelemetryDiag(b.legacyDiag.counts, b.legacyDiag.bytes, 8),
	)
}

func (b *Bridge) telemetryPayload(packet []byte) ([]byte, bool, error) {
	if len(packet) <= maxDirectUDPTelemetryBytes {
		return packet, false, nil
	}
	return b.spillTelemetryPacket(packet)
}

func (b *Bridge) spillTelemetryPacket(packet []byte) ([]byte, bool, error) {
	dir := strings.TrimSpace(b.cfg.FileReplyDir)
	if dir == "" {
		dir = defaultFileReplyDir()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return packet, false, err
	}
	meta := parseObject(string(packet))
	label := telemetryPacketLabel(meta)
	filename := fmt.Sprintf("%s_telemetry_%s.json",
		spillFileStamp(time.Now()),
		sanitizeFileComponent(label, "event"))
	path, err := writeSpillFileExclusive(dir, filename, packet)
	if err != nil {
		return packet, false, err
	}
	envelope := map[string]any{
		"transport":       "file_reply",
		"reply_file":      filepath.ToSlash(path),
		"telemetry_file":  filepath.ToSlash(path),
		"reply_bytes":     len(packet),
		"telemetry_bytes": len(packet),
		"command":         stringField(meta, "command"),
		"topic":           stringField(meta, "topic"),
		"type":            stringField(meta, "type"),
	}
	out, err := json.Marshal(envelope)
	if err != nil {
		return packet, false, err
	}
	if b.logger != nil {
		b.logger.Info("telemetry spilled to file label=%s bytes=%d file=%s", label, len(packet), path)
	}
	return out, true, nil
}

func (b *Bridge) normalizeTelemetry(text string, refreshCh chan<- string, lastSeq *int64) []byte {
	var d map[string]any
	if err := json.Unmarshal([]byte(text), &d); err != nil {
		return []byte(text)
	}
	if fmt.Sprint(d["type"]) == "delta_update" {
		if b.shadow != nil {
			b.shadow.ApplyDelta(d)
			// FIX-TRACKLIST-SHADOW-FRESHNESS-1 B：结构 delta（外部新建/删除轨道）
			// 快照吸收不了，置提示后由 runShadowRefresh 去抖拉一次权威快照。
			if b.shadow.ConsumeStructureRefreshHint() {
				b.signalShadowRefresh(refreshCh, shadowRefreshReasonStructural)
			}
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
		b.signalShadowRefresh(refreshCh, shadowRefreshReasonRecording)
	}
	if b.cfg.TelemetryHook != nil {
		b.cfg.TelemetryHook(d)
	}
	b.enqueueVSPRealtimeTelemetry(d)
	if b.vspHubOwnsLegacyTelemetryPacket(d) {
		return nil
	}
	out, err := json.Marshal(d)
	if err != nil {
		return []byte(text)
	}
	return out
}

// signalShadowRefresh 非阻塞投递一次 shadow 刷新信号（容量 1 天然合并并发提示）。
func (b *Bridge) signalShadowRefresh(refreshCh chan<- string, reason string) {
	if refreshCh == nil {
		return
	}
	select {
	case refreshCh <- reason:
	default:
	}
}

func (b *Bridge) dispatchShadowRefresh(ctx context.Context, reason string) {
	if b.shadowRefresh == nil {
		b.refreshShadow(ctx, reason)
		return
	}
	b.shadowRefresh(ctx, reason)
}

func (b *Bridge) vspHubOwnsLegacyTelemetryPacket(packet map[string]any) bool {
	if b == nil || b.realtimePublishCh == nil {
		return false
	}
	if envBool("VIT_BRIDGE_KEEP_LEGACY_REALTIME_UDP", false) {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(stringField(packet, "type")), "delta_update") {
		return !envBool("VIT_BRIDGE_KEEP_LEGACY_DELTA_UDP", false)
	}
	topic := strings.ToLower(strings.TrimSpace(stringField(packet, "topic")))
	return topic == "levels" || topic == "transport"
}

func envBool(key string, fallback bool) bool {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if raw == "" {
		return fallback
	}
	switch raw {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func (b *Bridge) runShadowRefresh(ctx context.Context, refreshCh <-chan string) error {
	var structuralArmed <-chan time.Time
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case reason := <-refreshCh:
			if reason == shadowRefreshReasonStructural {
				// 去抖：外部导入/工程加载会连发大量 node_added，合并为一次刷新。
				if structuralArmed == nil {
					structuralArmed = time.After(structuralShadowRefreshDebounce)
				}
				continue
			}
			b.dispatchShadowRefresh(ctx, reason)
		case <-structuralArmed:
			structuralArmed = nil
			b.dispatchShadowRefresh(ctx, shadowRefreshReasonStructural)
			// 与快照竞态的 delta 会重新置提示：再武装一轮，避免改动被去抖窗口吞掉。
			if b.shadow != nil && b.shadow.ConsumeStructureRefreshHint() {
				structuralArmed = time.After(structuralShadowRefreshDebounce)
			}
		}
	}
}

// refreshShadow 拉一次权威工程快照写入 shadow（VSP state snapshot 优先，legacy
// get_project_state 回退）。FIX-TRACKLIST-SHADOW-FRESHNESS-1 B。
func (b *Bridge) refreshShadow(ctx context.Context, reason string) {
	if b == nil || b.kernel == nil || b.shadow == nil {
		return
	}
	if observe, err := b.kernel.VSPStateSnapshot(ctx, "project.timeline"); err == nil && observe != nil && observe.OK() && len(observe.LegacyState) > 0 {
		b.shadow.Initialize(observe.LegacyState)
		if b.logger != nil {
			b.logger.Info("[shadow] refreshed after %s source=vsp.state.snapshot revision=%d", reason, observe.Revision)
		}
		return
	} else if err != nil && b.logger != nil {
		b.logger.Warn("[shadow] %s vsp state snapshot failed, falling back to legacy get_project_state: %v", reason, err)
	}
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

func telemetryPacketLabel(d map[string]any) string {
	if d == nil {
		return "telemetry"
	}
	command := stringField(d, "command")
	if command != "" {
		return command
	}
	topic := stringField(d, "topic")
	subtopic := stringField(d, "subtopic")
	if topic != "" && subtopic != "" {
		return topic + "_" + subtopic
	}
	if topic != "" {
		return topic
	}
	typ := stringField(d, "type")
	if typ != "" {
		return typ
	}
	return "telemetry"
}

func formatLegacyTelemetryDiag(counts map[string]int64, bytesByLabel map[string]int64, limit int) string {
	if len(counts) == 0 {
		return ""
	}
	type row struct {
		label string
		count int64
		bytes int64
	}
	rows := make([]row, 0, len(counts))
	for label, count := range counts {
		rows = append(rows, row{
			label: label,
			count: count,
			bytes: bytesByLabel[label],
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].count == rows[j].count {
			return rows[i].label < rows[j].label
		}
		return rows[i].count > rows[j].count
	})
	if limit <= 0 || limit > len(rows) {
		limit = len(rows)
	}
	parts := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		parts = append(parts, fmt.Sprintf("%s:%d/%dB", rows[i].label, rows[i].count, rows[i].bytes))
	}
	return strings.Join(parts, ",")
}

func stringField(d map[string]any, key string) string {
	if d == nil {
		return ""
	}
	v, ok := d[key]
	if !ok || v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
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
