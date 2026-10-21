<#
D1-STALL-REPRO-SELFTEST-1: deterministic self-test for the d1_stall_repro_smoke
judgment loop (STALLSMOKE-NO-DELIVERY-CLASS-1 self-verification face).

Drives the REAL d1_stall_repro_smoke.ps1 against a stubbed state surface:
zero kernel, zero agent, zero LLM. A stub kernel executable holds port 5555 and
a stub agent executable serves /agent/runtime/status + /agent/events per
scenario, so every verdict/exit mapping of the classification table is pinned
without the probabilistic LLM carrier (the silent-delivery-death shape is by
design unreachable on a fixed stack, so the new parked_owe_no_delivery class
can only be exercised through state injection).

Scenarios (the full classification table):
  no_delivery          goal waiting_continue, no live continuations, zero
                       chain_result events, stable across polls  -> exit 1
  parked_owe           waiting_continue + owed receipt needle        -> exit 0
  parked_owe_silent    chain_result present, needle-free body        -> exit 1
  finished             goal completed, task settled                  -> exit 0
  defect_fake_complete goal completed, task needs_experiment         -> exit 1

Usage:
  powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\d1_stall_repro_selftest.ps1
  powershell ... -RunRoot <dir>
#>

[CmdletBinding()]
param(
    [string]$RepoRoot = "",
    [string]$RunRoot = ""
)

$ErrorActionPreference = "Stop"

function Resolve-RepoRoot {
    param([string]$Explicit)
    if (-not [string]::IsNullOrWhiteSpace($Explicit)) { return (Resolve-Path -LiteralPath $Explicit).Path }
    return (Split-Path -Parent (Split-Path -Parent $MyInvocation.ScriptName))
}

function Write-Step { param([string]$Message) Write-Host ""; Write-Host ("== " + $Message) -ForegroundColor Cyan }
function Write-Ok { param([string]$Message) Write-Host ("ok: " + $Message) -ForegroundColor Green }
function Write-Bad { param([string]$Message) Write-Host ("fail: " + $Message) -ForegroundColor Red }

# ASCII-safe file: this script stays pure ASCII (the smoke script convention:
# BOM-less .ps1 reads as ANSI); non-ASCII state lives only inside the stub's
# UTF-8 byte literal below.

$RepoRoot = Resolve-RepoRoot -Explicit $RepoRoot
$SmokeScript = Join-Path $RepoRoot "scripts\d1_stall_repro_smoke.ps1"
if (-not (Test-Path -LiteralPath $SmokeScript -PathType Leaf)) { throw ("smoke script missing: " + $SmokeScript) }
if ([string]::IsNullOrWhiteSpace($RunRoot)) {
    $RunRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("d1_stall_selftest_" + (Get-Date -Format "yyyyMMdd_HHmmss"))
}
New-Item -ItemType Directory -Force -Path $RunRoot | Out-Null
Write-Host ("selftest run root: " + $RunRoot)

# ---------------------------------------------------------------------------
# Stub sources (single-file Go programs, stdlib only, built into the run dir;
# never committed as .go files and never written into the source tree).
# ---------------------------------------------------------------------------
$AgentStubSource = @'
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type stubState struct {
	GoalStatus   string
	TaskState    string
	Continuations []map[string]string
	Events       []map[string]interface{}
}

func main() {
	form := "no_delivery"
	addr := "127.0.0.1:7878"
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		if args[i] == "-form" && i+1 < len(args) {
			form = args[i+1]
			i++
		}
		if args[i] == "-http" && i+1 < len(args) {
			addr = args[i+1]
			i++
		}
	}

	state := stubState{GoalStatus: "waiting_continue", TaskState: "needs_experiment", Continuations: []map[string]string{}}
	events := []map[string]interface{}{}
	switch form {
	case "parked_owe":
		// resumeNeedle is the smoke script's NeedleContinue embedded as UTF-8
		// bytes so this source stays pure ASCII.
		resumeNeedle := string([]byte{0xe7, 0xbb, 0xa7, 0xe7, 0xbb, 0xad})
		events = append(events, map[string]interface{}{"type": "chain.result", "item_id": "chain_result", "body": "auto continuation budget exhausted; the task is not finished; send " + resumeNeedle + " to resume the owed evaluation."})
	case "parked_owe_silent":
		events = append(events, map[string]interface{}{"type": "chain.result", "item_id": "chain_result", "body": "chain terminal reached; internal grading recorded without a user-language summary."})
	case "finished":
		state.GoalStatus = "completed"
		state.TaskState = "settled"
	case "defect_fake_complete":
		state.GoalStatus = "completed"
		state.TaskState = "needs_experiment"
	}

	mux := http.NewServeMux()
	writeJSON := func(w http.ResponseWriter, payload interface{}) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	}
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/agent/state", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"shadow": map[string]interface{}{"initialized": true, "track_count": 6}})
	})
	mux.HandleFunc("/agent/authority", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"authority_mode": "full_project_access"})
	})
	mux.HandleFunc("/agent/chat", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"goal_id": "goal_stub", "goal_status": state.GoalStatus, "stop_reason": "stub_auto_park"})
	})
	mux.HandleFunc("/agent/runtime/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"goal": map[string]interface{}{"status": state.GoalStatus}, "task_state": state.TaskState, "continuations": state.Continuations})
	})
	mux.HandleFunc("/agent/events", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"events": events})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{"stub": true, "path": r.URL.Path})
	})

	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	fmt.Println("stub agent (" + form + ") listening on " + strings.TrimSpace(addr))
	log.Fatal(server.ListenAndServe())
}
'@

$KernelStubSource = @'
package main

import (
	"fmt"
	"log"
	"net"
)

// The smoke script only asserts that the kernel ZMQ port listens; no protocol
// exchange ever happens against the stub.
func main() {
	listener, err := net.Listen("tcp", "127.0.0.1:5555")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("stub kernel listening on 127.0.0.1:5555")
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal(err)
		}
		_ = conn
	}
}
'@

Write-Step "Build stubs"
$StubDir = Join-Path $RunRoot "stubs"
New-Item -ItemType Directory -Force -Path (Join-Path $StubDir "agent") | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $StubDir "kernel") | Out-Null
$AgentStubSourcePath = Join-Path $StubDir "agent\main.go"
$KernelStubSourcePath = Join-Path $StubDir "kernel\main.go"
[System.IO.File]::WriteAllText($AgentStubSourcePath, $AgentStubSource)
[System.IO.File]::WriteAllText($KernelStubSourcePath, $KernelStubSource)
$AgentStubExe = Join-Path $StubDir "agent_stub.exe"
$KernelStubExe = Join-Path $StubDir "kernel_stub.exe"
Push-Location (Join-Path $StubDir "agent")
try {
    & go build -o $AgentStubExe main.go
    if ($LASTEXITCODE -ne 0) { throw ("agent stub go build failed with exit code " + $LASTEXITCODE) }
}
finally { Pop-Location }
Push-Location (Join-Path $StubDir "kernel")
try {
    & go build -o $KernelStubExe main.go
    if ($LASTEXITCODE -ne 0) { throw ("kernel stub go build failed with exit code " + $LASTEXITCODE) }
}
finally { Pop-Location }
Write-Ok ("stubs built: " + $AgentStubExe)

$cases = @(
    @{ form = "no_delivery";          expectExit = 1; expectVerdict = "parked_owe_no_delivery" },
    @{ form = "parked_owe";           expectExit = 0; expectVerdict = "parked_owe" },
    @{ form = "parked_owe_silent";    expectExit = 1; expectVerdict = "parked_owe_silent" },
    @{ form = "finished";             expectExit = 0; expectVerdict = "finished" },
    @{ form = "defect_fake_complete"; expectExit = 1; expectVerdict = "defect_fake_complete" }
)

$results = New-Object System.Collections.Generic.List[string]
$failed = $false
foreach ($case in $cases) {
    $form = $case.form
    Write-Step ("Case: " + $form + " (expect verdict=" + $case.expectVerdict + " exit=" + $case.expectExit + ")")
    $caseDir = Join-Path $RunRoot ("case_" + $form)
    $smokeRunRoot = Join-Path $caseDir "smoke"
    $agentBin = Join-Path $smokeRunRoot "bin\VitAgent.d1stall.exe"
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $agentBin) | Out-Null
    Copy-Item -LiteralPath $AgentStubExe -Destination $agentBin -Force

    $output = & powershell -NoProfile -ExecutionPolicy Bypass -File $SmokeScript `
        -RepoRoot $RepoRoot `
        -RunRoot $smokeRunRoot `
        -SkipBuild `
        -KernelExe $KernelStubExe `
        -KernelDwellSeconds 0 `
        -WaitSeconds 15 `
        -ChainBudgetSeconds 40 `
        -PollSeconds 1 `
        -NoDeliveryCycles 3 *>&1
    $exitCode = $LASTEXITCODE
    $output | Out-File -FilePath (Join-Path $caseDir "console.txt") -Encoding utf8

    $verdictLine = $output | Where-Object { $_ -like "D1_STALL_VERDICT *" } | Select-Object -First 1
    $verdict = ""
    if ($verdictLine) { $verdict = ([string]$verdictLine).Substring("D1_STALL_VERDICT ".Length).Trim() }

    $problems = New-Object System.Collections.Generic.List[string]
    if ($exitCode -ne $case.expectExit) { $problems.Add(("exit=" + $exitCode + " want " + $case.expectExit)) }
    if ($verdict -ne $case.expectVerdict) { $problems.Add(("verdict=" + $verdict + " want " + $case.expectVerdict)) }
    $reportPath = Join-Path $smokeRunRoot "d1_stall_report.json"
    if (Test-Path -LiteralPath $reportPath) {
        $report = Get-Content -LiteralPath $reportPath -Raw | ConvertFrom-Json
        if ([string]$report.verdict -ne $case.expectVerdict) { $problems.Add(("report.verdict=" + $report.verdict)) }
    } else {
        $problems.Add("report json missing")
    }

    if ($problems.Count -eq 0) {
        Write-Ok ("case " + $form + ": verdict=" + $verdict + " exit=" + $exitCode)
        $results.Add(("PASS " + $form + " verdict=" + $verdict + " exit=" + $exitCode))
    } else {
        Write-Bad ("case " + $form + ": " + ($problems -join " ; "))
        $results.Add(("FAIL " + $form + " " + ($problems -join " ; ")))
        $failed = $true
    }
}

Write-Step "Summary"
$results | ForEach-Object { Write-Host $_ }
$prereq = Join-Path $RunRoot "selftest_summary.txt"
$results | Out-File -FilePath $prereq -Encoding utf8
if ($failed) { exit 1 }
Write-Ok "all classification cases behaved as declared"
exit 0
