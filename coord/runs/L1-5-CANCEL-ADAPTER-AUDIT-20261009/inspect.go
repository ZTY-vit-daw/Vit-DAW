package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"strings"
)

type anchor struct {
	Revision string `json:"revision"`
	Path     string `json:"path"`
	Name     string `json:"name"`
	Start    int    `json:"start"`
	End      int    `json:"end"`
}

type site struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Call string `json:"call"`
}

func git(args ...string) string {
	b, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		panic(fmt.Sprintf("git %v: %v: %s", args, err, b))
	}
	return string(b)
}

func main() {
	const a = "de1d876c"
	const c = "c488b811"
	wanted := map[string]map[string]bool{
		"agent/internal/pullharness/loop.go":                          {"GoalInput": true, "Result": true, "FastPathRouter": true, "Run": true, "finish": true},
		"agent/internal/pullharness/budget.go":                        {"ObservationBudget": true, "CycleExhausted": true, "ProbeExhausted": true},
		"agent/internal/fastpath/router.go":                           {"TryMatch": true, "SetDiagnosticOnly": true},
		"agent/internal/agentloop/runner.go":                          {"Continuation": true, "Result": true, "runState": true, "Continue": true, "ResumeAfterConfirmation": true, "executeTool": true, "checkpoint": true, "checkTurnBudget": true, "checkToolBudget": true, "complete": true, "fail": true, "pause": true, "result": true},
		"agent/internal/agentloop/message_loop.go":                    {"newFastPathRouter": true, "loop": true, "preflightClipFadeGainRead": true, "preflightClipFadeGainSet": true, "preflightProjectBlackboardStatus": true, "preflightClipRangeSplit": true, "executeMessageLoopToolCalls": true},
		"agent/internal/agentloop/static_mix_gain_staging_context.go": {"preflightStaticMixGainStagingContextPack": true},
		"agent/internal/agentloop/free_state_reasoning.go":            {"messageLoopFreeStateDiagnosticOnly": true, "messageLoopTerminalFallbackResult": true},
		"agent/internal/agentloop/exit_retain.go":                     {"retainRunObservationConclusions": true, "observationConclusionUnits": true},
		"agent/internal/agentloop/interaction_pause.go":               {"pendingInteractionPauseForResult": true},
		"agent/internal/executor/executor.go":                         {"RunToolCall": true},
		"agent/internal/harness/harness.go":                           {"Invoke": true},
		"agent/internal/executionruntime/coordinator.go":              {"Execute": true, "Reconcile": true},
		"agent/internal/executionports/staticbalance_vsp.go":          {"Apply": true},
	}
	var anchors []anchor
	var sites []site
	var statuses []site
	for _, rev := range []string{a, c} {
		paths := strings.Fields(git("ls-tree", "-r", "--name-only", rev, "agent/internal"))
		for _, path := range paths {
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				continue
			}
			isA := strings.Contains(path, "/pullharness/")
			if (rev == a) != isA || (!isA && wanted[path] == nil && !strings.Contains(path, "/agentloop/") && path != "agent/internal/runtime/runtime.go") {
				continue
			}
			src := git("show", rev+":"+path)
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, path, src, 0)
			if err != nil {
				panic(err)
			}
			for _, decl := range f.Decls {
				switch d := decl.(type) {
				case *ast.FuncDecl:
					if wanted[path][d.Name.Name] {
						anchors = append(anchors, anchor{rev, path, d.Name.Name, fset.Position(d.Pos()).Line, fset.Position(d.End()).Line})
						delete(wanted[path], d.Name.Name)
					}
				case *ast.GenDecl:
					for _, spec := range d.Specs {
						if t, ok := spec.(*ast.TypeSpec); ok && wanted[path][t.Name.Name] {
							anchors = append(anchors, anchor{rev, path, t.Name.Name, fset.Position(t.Pos()).Line, fset.Position(t.End()).Line})
							delete(wanted[path], t.Name.Name)
						}
					}
				}
			}
			if rev != c {
				continue
			}
			ast.Inspect(f, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok && strings.Contains(path, "/agentloop/") {
					if s, ok := call.Fun.(*ast.SelectorExpr); ok && (s.Sel.Name == "pause" || s.Sel.Name == "result" || s.Sel.Name == "complete" || s.Sel.Name == "fail") {
						sites = append(sites, site{path, fset.Position(call.Pos()).Line, src[fset.Position(call.Pos()).Offset:fset.Position(call.End()).Offset]})
					}
				}
				if v, ok := n.(*ast.ValueSpec); ok && path == "agent/internal/runtime/runtime.go" {
					for _, name := range v.Names {
						if strings.HasPrefix(name.Name, "Status") {
							statuses = append(statuses, site{path, fset.Position(v.Pos()).Line, src[fset.Position(v.Pos()).Offset:fset.Position(v.End()).Offset]})
						}
					}
				}
				return true
			})
		}
	}
	for path, names := range wanted {
		if len(names) > 0 {
			fmt.Fprintf(os.Stderr, "missing declarations %s: %v\n", path, names)
			os.Exit(1)
		}
	}
	out := map[string]any{"a": strings.TrimSpace(git("rev-parse", a)), "c": strings.TrimSpace(git("rev-parse", c)), "anchors": anchors, "runtime_statuses": statuses, "lifecycle_calls": sites, "anchor_count": len(anchors)}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		panic(err)
	}
}
