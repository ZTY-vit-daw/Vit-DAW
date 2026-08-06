// Package vst3host supervises the crash-isolated native VST3 worker used by
// pluginprobe. The worker speaks newline-delimited JSON over stdio and exposes
// observation operations only.
package vst3host

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const (
	WorkerProtocol          = "vit.pluginprobe.vst3_worker.v1"
	MaxWorkerResponseBytes  = 32 << 20
	MaxWorkerDiagnosticByte = 64 << 10
)

// WorkerError is returned when the isolated worker fails, exits, or returns a
// structured operation error. Its bounded diagnostic data excludes raw plugin
// state.
type WorkerError struct {
	Code       string
	Message    string
	Diagnostic string
}

func (e *WorkerError) Error() string {
	if e == nil {
		return ""
	}
	if e.Code == "" {
		return e.Message
	}
	return e.Code + ": " + e.Message
}

type workerResponse struct {
	OK       bool            `json:"ok"`
	Protocol string          `json:"protocol"`
	Result   json.RawMessage `json:"result"`
	Error    struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Logs []string `json:"logs"`
}

// Response is an operation result emitted by the native worker.
type Response struct {
	Result json.RawMessage
	Logs   []string
}

// Worker represents exactly one native process. Calls are serialized because
// each loaded plugin has process-local state.
type Worker struct {
	mu       sync.Mutex
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	stdout   *bufio.Reader
	stderr   *boundedBuffer
	waitDone chan error
	closed   bool
}

// Start launches a native worker without a visible window on Windows. The
// caller must call Close. A separate worker is used for each loaded plugin so
// a VST3 crash cannot take down pluginprobe.
func Start(path string) (*Worker, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("native VST3 worker path is required")
	}
	if info, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("inspect native VST3 worker: %w", err)
	} else if info.IsDir() {
		return nil, fmt.Errorf("native VST3 worker path is a directory: %s", path)
	}
	cmd := exec.Command(path)
	configureCommand(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("open native worker stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("open native worker stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("open native worker stderr: %w", err)
	}
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("start native worker: %w", err)
	}
	w := &Worker{
		cmd:      cmd,
		stdin:    stdin,
		stdout:   bufio.NewReaderSize(stdout, 64<<10),
		stderr:   newBoundedBuffer(MaxWorkerDiagnosticByte),
		waitDone: make(chan error, 1),
	}
	go func() { _, _ = io.Copy(w.stderr, stderr) }()
	go func() { w.waitDone <- cmd.Wait() }()
	return w, nil
}

// PID returns the native worker process ID while it is alive. It is intended
// only for bounded, local crash-isolation evidence; it does not expose a
// control surface for the child process.
func (w *Worker) PID() int {
	if w == nil {
		return 0
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed || w.cmd == nil || w.cmd.Process == nil {
		return 0
	}
	return w.cmd.Process.Pid
}

// Call sends one command and waits for one fresh worker response. If the
// context expires, the worker is terminated rather than reused with uncertain
// plugin state; the parent pluginprobe process remains healthy.
func (w *Worker) Call(ctx context.Context, operation string, request map[string]any) (Response, error) {
	if w == nil {
		return Response{}, &WorkerError{Code: "worker_unavailable", Message: "native VST3 worker is nil"}
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return Response{}, &WorkerError{Code: "worker_closed", Message: "native VST3 worker is closed"}
	}
	if !observationOperation(operation) {
		return Response{}, &WorkerError{Code: "operation_not_observational", Message: "pluginprobe does not permit operation " + strings.TrimSpace(operation)}
	}
	if request == nil {
		request = map[string]any{}
	}
	request["op"] = operation
	encoded, err := json.Marshal(request)
	if err != nil {
		return Response{}, fmt.Errorf("encode %s request: %w", operation, err)
	}
	if _, err := w.stdin.Write(append(encoded, '\n')); err != nil {
		return Response{}, w.processError("worker_write_failed", fmt.Sprintf("write %s request: %v", operation, err))
	}

	type lineResult struct {
		line []byte
		err  error
	}
	lineDone := make(chan lineResult, 1)
	go func() {
		line, readErr := readLineLimit(w.stdout, MaxWorkerResponseBytes)
		lineDone <- lineResult{line: line, err: readErr}
	}()
	select {
	case <-ctx.Done():
		w.stopLocked()
		return Response{}, w.processError("worker_timeout", fmt.Sprintf("%s did not complete before context deadline", operation))
	case result := <-lineDone:
		if result.err != nil {
			w.closed = true
			return Response{}, w.processError("worker_crashed", fmt.Sprintf("read %s response: %v", operation, result.err))
		}
		var response workerResponse
		if err := json.Unmarshal(result.line, &response); err != nil {
			return Response{}, w.processError("worker_invalid_response", fmt.Sprintf("decode %s response: %v", operation, err))
		}
		if response.Protocol != WorkerProtocol {
			return Response{}, w.processError("worker_protocol_mismatch", fmt.Sprintf("native worker protocol %q is not %q", response.Protocol, WorkerProtocol))
		}
		if !response.OK {
			return Response{}, &WorkerError{Code: response.Error.Code, Message: response.Error.Message, Diagnostic: w.stderr.String()}
		}
		return Response{Result: append(json.RawMessage(nil), response.Result...), Logs: append([]string(nil), response.Logs...)}, nil
	}
}

func observationOperation(operation string) bool {
	switch strings.ToLower(strings.TrimSpace(operation)) {
	case "load", "snapshot", "render", "unload":
		return true
	default:
		return false
	}
}

// Close ends the native child and discards the isolated plugin instance.
func (w *Worker) Close() error {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	w.stopLocked()
	return nil
}

func (w *Worker) stopLocked() {
	if w.closed {
		return
	}
	w.closed = true
	if w.stdin != nil {
		_ = w.stdin.Close()
	}
	select {
	case <-w.waitDone:
	case <-time.After(1500 * time.Millisecond):
		if w.cmd != nil && w.cmd.Process != nil {
			_ = w.cmd.Process.Kill()
		}
		select {
		case <-w.waitDone:
		case <-time.After(1500 * time.Millisecond):
		}
	}
}

func (w *Worker) processError(code, message string) error {
	diagnostic := ""
	if w != nil && w.stderr != nil {
		diagnostic = w.stderr.String()
	}
	select {
	case err := <-w.waitDone:
		if err != nil {
			message += "; process exit: " + err.Error()
		}
	default:
	}
	return &WorkerError{Code: code, Message: message, Diagnostic: diagnostic}
}

func readLineLimit(reader *bufio.Reader, limit int) ([]byte, error) {
	var out []byte
	for {
		part, err := reader.ReadSlice('\n')
		out = append(out, part...)
		if len(out) > limit {
			return nil, fmt.Errorf("worker response exceeds %d-byte limit", limit)
		}
		if err == nil {
			return bytes.TrimSpace(out), nil
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		return nil, err
	}
}

type boundedBuffer struct {
	mu    sync.Mutex
	limit int
	data  []byte
}

func newBoundedBuffer(limit int) *boundedBuffer { return &boundedBuffer{limit: limit} }

func (b *boundedBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(data) >= b.limit {
		b.data = append([]byte(nil), data[len(data)-b.limit:]...)
		return len(data), nil
	}
	if overflow := len(b.data) + len(data) - b.limit; overflow > 0 {
		b.data = append([]byte(nil), b.data[overflow:]...)
	}
	b.data = append(b.data, data...)
	return len(data), nil
}

func (b *boundedBuffer) String() string {
	if b == nil {
		return ""
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.TrimSpace(string(b.data))
}
