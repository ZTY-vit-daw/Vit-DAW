package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vit-daw-agent/internal/logx"
	"vit-daw-agent/internal/tim"
)

// L2-1-TIM-SMOKE-1 red test: constructing a Harness with a logger must wire
// tim.AssertWarnLogger through logx so [tim.assert] fail rows reach stdout and
// agent_last.log (design docs/TIM_ASSERTER_V1_DESIGN.md §1.4). Before the
// wiring, tim.AssertWarnLogger stays nil and this test fails.

func TestHarnessWiresTIMAssertWarnLoggerThroughLogx(t *testing.T) {
	prev := tim.AssertWarnLogger
	t.Cleanup(func() { tim.AssertWarnLogger = prev })

	logPath := filepath.Join(t.TempDir(), "agent_last.log")
	logger := logx.New(false, logPath, 100)
	NewWithSender(nil, nil, logger)

	if tim.AssertWarnLogger == nil {
		t.Fatalf("tim.AssertWarnLogger not wired by Harness construction")
	}
	tim.AssertWarnLogger("[tim.assert] asserter=probe check=probe status=fail track=t1 code=probe_fail")

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("agent log not written: %v", err)
	}
	line := strings.TrimSpace(string(data))
	if !strings.Contains(line, "[WARN]") {
		t.Fatalf("wired line not at WARN level: %s", line)
	}
	if !strings.Contains(line, "[tim.assert] asserter=probe check=probe status=fail track=t1 code=probe_fail") {
		t.Fatalf("log line payload mangled: %s", line)
	}
}

func TestHarnessWithoutLoggerKeepsTIMHookUntouched(t *testing.T) {
	prev := tim.AssertWarnLogger
	t.Cleanup(func() { tim.AssertWarnLogger = prev })

	tim.AssertWarnLogger = nil
	NewWithSender(nil, nil, nil)
	if tim.AssertWarnLogger != nil {
		t.Fatalf("nil logger must keep tim.AssertWarnLogger nil-silent")
	}
}
