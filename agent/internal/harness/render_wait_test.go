package harness

import (
	"context"
	"testing"
	"time"
)

func TestWaitRenderReturnsCachedTerminalTelemetry(t *testing.T) {
	h := NewWithSender(nil, nil, nil)
	h.IngestKernelTelemetry(map[string]any{"topic": "render", "subtopic": "render_done", "job_id": "job-1", "status": "ok", "file_path": `D:\tmp\before.wav`})

	result, err := h.WaitRender(context.Background(), "job-1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ready" || result.FilePath != `D:\tmp\before.wav` {
		t.Fatalf("result=%+v", result)
	}
}

func TestWaitRenderWakesOnFailureTelemetry(t *testing.T) {
	h := NewWithSender(nil, nil, nil)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan RenderResult, 1)
	errs := make(chan error, 1)
	go func() {
		result, err := h.WaitRender(ctx, "job-2")
		done <- result
		errs <- err
	}()
	time.Sleep(10 * time.Millisecond)
	h.IngestKernelTelemetry(map[string]any{"topic": "render", "subtopic": "render_failed", "job_id": "job-2", "status": "error", "message": "render failed"})
	if err := <-errs; err != nil {
		t.Fatal(err)
	}
	if result := <-done; result.Status != "failed" || result.Error != "render failed" {
		t.Fatalf("result=%+v", result)
	}
}
