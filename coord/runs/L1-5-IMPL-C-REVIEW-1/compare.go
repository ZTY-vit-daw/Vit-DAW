package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const baseline = "4c2f75596e3571ab9f1ef3eccdaeb6fd25bbdbc8"
const reviewed = "c488b811"

type mapping struct {
	Old, New, Source, Target string
	Copy                     bool
}
type row struct {
	Mapping              mapping
	OldLine, NewLine     int
	Bytes                int
	OldSHA256, NewSHA256 string
	Equal                bool
	Wrapper              string
}
type parsed struct {
	Data      []byte
	Set       *token.FileSet
	Tree      *ast.File
	Functions map[string]*ast.FuncDecl
}

var output = "coord/runs/L1-5-IMPL-C-REVIEW-1"
var host = "agent/internal/agentloop/message_loop.go"

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func git(args ...string) []byte {
	b, err := exec.Command("git", args...).Output()
	must(err)
	return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
}
func write(name string, data []byte) { must(os.WriteFile(filepath.Join(output, name), data, 0644)) }
func jsonFile(name string, v any) {
	b, err := json.MarshalIndent(v, "", "  ")
	must(err)
	write(name, append(b, '\n'))
}
func parse(data []byte) parsed {
	s := token.NewFileSet()
	t, err := parser.ParseFile(s, "input.go", data, parser.ParseComments)
	must(err)
	p := parsed{data, s, t, map[string]*ast.FuncDecl{}}
	for _, d := range t.Decls {
		if f, ok := d.(*ast.FuncDecl); ok {
			p.Functions[f.Name.Name] = f
		}
	}
	return p
}
func (p parsed) fragment(n ast.Node) []byte {
	return p.Data[p.Set.Position(n.Pos()).Offset:p.Set.Position(n.End()).Offset]
}
func digest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

// Replace only explicitly mapped identifier tokens; preserve every other byte.
func rename(b []byte, replacements map[string]string) []byte {
	s := token.NewFileSet()
	f := s.AddFile("fragment", -1, len(b))
	var sc scanner.Scanner
	sc.Init(f, b, nil, scanner.ScanComments)
	var result bytes.Buffer
	last := 0
	for {
		pos, tok, lit := sc.Scan()
		if tok == token.EOF {
			break
		}
		if tok == token.IDENT {
			if old, ok := replacements[lit]; ok {
				offset := f.Offset(pos)
				result.Write(b[last:offset])
				result.WriteString(old)
				last = offset + len(lit)
			}
		}
	}
	result.Write(b[last:])
	return result.Bytes()
}

func main() {
	maps := []mapping{
		{"messageLoopText", "Text", host, "text.go", false},
		{"messageLoopTextHasAny", "TextHasAny", host, "text.go", false},
		{"messageLoopClipFadeGainRequest", "ClipFadeGainRequest", host, "clip_fade_gain.go", false},
		{"messageLoopClipFadeGainReadRequest", "ClipFadeGainReadRequest", host, "clip_fade_gain.go", false},
		{"messageLoopExecutedClipRangeSplitCuts", "ExecutedClipRangeSplitCuts", host, "clip_range_split.go", false},
		{"clipRangeSplitCutKey", "ClipRangeSplitCutKey", host, "clip_range_split.go", false},
		{"messageLoopClipRangeSplitCompletionReply", "ClipRangeSplitCompletionReply", host, "clip_range_split.go", false},
		{"messageLoopStripSilenceSuggestRequest", "StripSilenceSuggestRequest", host, "strip_silence.go", false},
		{"messageLoopA4ClipCleanupRequest", "A4ClipCleanupRequest", host, "strip_silence.go", false},
		{"messageLoopA4ClipCleanupWholeProjectRequest", "A4ClipCleanupWholeProjectRequest", host, "strip_silence.go", false},
		{"messageLoopNaturalMixRequest", "NaturalMixRequest", host, "natural_mix.go", false},
		{"messageLoopAudioObservationRequest", "AudioObservationRequest", host, "natural_mix.go", false},
		{"messageLoopGainStagingExplicitRequest", "GainStagingExplicitRequest", "agent/internal/agentloop/static_mix_gain_staging_context.go", "gain_staging_ref.go", true},
		{"messageLoopStaticBalanceIntentReference", "StaticBalanceIntentReference", "agent/internal/agentloop/static_mix_gain_staging_context.go", "gain_staging_ref.go", true},
		{"messageLoopGainStagingStrictReferenceIntent", "GainStagingStrictReferenceIntent", "agent/internal/agentloop/static_mix_gain_staging_context.go", "gain_staging_ref.go", true},
	}
	replacements := map[string]string{}
	for _, m := range maps {
		replacements[m.New] = m.Old
	}
	newHost := parse(git("show", reviewed+":"+host))
	var rows []row
	for _, m := range maps {
		old := parse(git("show", baseline+":"+m.Source))
		target := "agent/internal/fastpath/" + m.Target
		next := parse(git("show", reviewed+":"+target))
		of, nf := old.Functions[m.Old], next.Functions[m.New]
		if of == nil || nf == nil {
			panic("missing mapped function: " + m.Old)
		}
		a, b := old.fragment(of), rename(next.fragment(nf), replacements)
		wrapper := "original unchanged (copy)"
		if !m.Copy {
			wrapper = string(newHost.fragment(newHost.Functions[m.Old].Body))
		}
		rows = append(rows, row{m, old.Set.Position(of.Pos()).Line, next.Set.Position(nf.Pos()).Line, len(a), digest(a), digest(b), bytes.Equal(a, b), wrapper})
		write(m.New+".old.txt", a)
		write(m.New+".mapped.txt", b)
	}
	jsonFile("function-equivalence.json", rows)
	oldHost := parse(git("show", baseline+":"+host))
	oldLoop := string(oldHost.fragment(oldHost.Functions["loop"].Body))
	newCtor := string(newHost.fragment(newHost.Functions["newFastPathRouter"].Body))
	oldCalls := regexp.MustCompile(`l\.(preflight\w+)\(ctx, r, state\)`).FindAllStringSubmatch(oldLoop, -1)
	newCalls := regexp.MustCompile(`Name: "([a-z_]+)", Handler: l\.(preflight\w+)`).FindAllStringSubmatch(newCtor, -1)
	if len(oldCalls) != 10 || len(newCalls) != 10 {
		panic("unexpected registration count")
	}
	var chain []map[string]any
	for i, old := range oldCalls {
		chain = append(chain, map[string]any{"position": i + 1, "old_handler": old[1], "new_handler": newCalls[i][2], "entry": newCalls[i][1], "equal": old[1] == newCalls[i][2]})
	}
	jsonFile("registration-chain.json", chain)
	write("old-loop.txt", oldHost.fragment(oldHost.Functions["loop"]))
	write("new-loop.txt", newHost.fragment(newHost.Functions["loop"]))
	write("new-router-constructor.txt", newHost.fragment(newHost.Functions["newFastPathRouter"]))
	// Inventory direct selectors and helper calls of retained preflights for D.
	var dependencies []map[string]any
	for _, item := range newCalls {
		p := newHost
		source := host
		if p.Functions[item[2]] == nil {
			source = "agent/internal/agentloop/static_mix_gain_staging_context.go"
			p = parse(git("show", reviewed+":"+source))
		}
		f := p.Functions[item[2]]
		if f == nil {
			panic("unresolved preflight: " + item[2])
		}
		fields := map[string]bool{}
		calls := map[string]bool{}
		ast.Inspect(f.Body, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.SelectorExpr:
				if id, ok := v.X.(*ast.Ident); ok && (id.Name == "state" || id.Name == "r" || id.Name == "l") {
					fields[id.Name+"."+v.Sel.Name] = true
				}
			case *ast.CallExpr:
				if id, ok := v.Fun.(*ast.Ident); ok {
					calls[id.Name] = true
				}
			}
			return true
		})
		selectors, helpers := []string{}, []string{}
		for x := range fields {
			selectors = append(selectors, x)
		}
		for x := range calls {
			helpers = append(helpers, x)
		}
		sort.Strings(selectors)
		sort.Strings(helpers)
		dependencies = append(dependencies, map[string]any{"handler": item[2], "file": source, "line": p.Set.Position(f.Pos()).Line, "signature": string(p.Data[p.Set.Position(f.Pos()).Offset:p.Set.Position(f.Body.Pos()).Offset]), "direct_selectors": selectors, "direct_calls": helpers})
	}
	jsonFile("retained-handler-dependencies.json", dependencies)
	var preserved []map[string]any
	for name, oldFunction := range oldHost.Functions {
		if name == "loop" {
			continue
		}
		moved := false
		for _, m := range maps {
			if !m.Copy && name == m.Old {
				moved = true
			}
		}
		if moved {
			continue
		}
		next := newHost.Functions[name]
		if next == nil {
			panic("deleted function: " + name)
		}
		equal := bytes.Equal(oldHost.fragment(oldFunction), newHost.fragment(next))
		preserved = append(preserved, map[string]any{"function": name, "equal": equal})
		if !equal {
			panic("unexpected retained function change: " + name)
		}
	}
	sort.Slice(preserved, func(i, j int) bool { return preserved[i]["function"].(string) < preserved[j]["function"].(string) })
	jsonFile("preserved-functions.json", preserved)
	files := strings.Fields(string(git("diff", "--name-only", baseline, reviewed, "--", "*.go")))
	var formatting []map[string]any
	for _, path := range files {
		committed := git("show", reviewed+":"+path)
		c := exec.Command("gofmt")
		c.Stdin = bytes.NewReader(committed)
		formatted, err := c.Output()
		must(err)
		local, err := os.ReadFile(path)
		must(err)
		raw := exec.Command("gofmt")
		raw.Stdin = bytes.NewReader(local)
		localFormatted, err := raw.Output()
		must(err)
		formatting = append(formatting, map[string]any{"file": path, "committed_lf_clean": bytes.Equal(committed, formatted), "checkout_raw_clean": bytes.Equal(local, localFormatted), "checkout_lf_clean": bytes.Equal(bytes.ReplaceAll(local, []byte("\r\n"), []byte("\n")), localFormatted)})
	}
	jsonFile("gofmt.json", formatting)
	write("original-name-status.txt", git("diff", baseline, reviewed, "--name-status"))
	write("original-stat.txt", git("diff", baseline, reviewed, "--stat"))
	write("original-production.diff", git("diff", baseline, reviewed, "--", "agent"))
	write("integration-production.diff", git("diff", "05ed52e8", "124a643d", "--", "agent"))
	write("review-head.txt", git("rev-parse", "HEAD"))
	write("go-version.txt", mustOutput(exec.Command("go", "version")))
	equal := 0
	for _, row := range rows {
		if row.Equal {
			equal++
		}
	}
	fmt.Printf("function equivalence: %d/%d (12 moved + 3 copies)\n", equal, len(rows))
	if equal != len(rows) {
		os.Exit(1)
	}
}
func mustOutput(c *exec.Cmd) []byte { b, err := c.Output(); must(err); return b }
