package policy

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"vit-daw-agent/internal/tools"
)

type Risk string

const (
	RiskDirect   Risk = "direct"
	RiskUndoable Risk = "undoable"
	RiskConfirm  Risk = "confirm"
)

type Decision struct {
	Command map[string]any `json:"command"`
	Name    string         `json:"name"`
	Risk    Risk           `json:"risk"`
	Reason  string         `json:"reason"`
}

var directCommands = map[string]bool{
	"ping":                                  true,
	"get_project_state":                     true,
	"list_tracks":                           true,
	"get_recent_projects":                   true,
	"play":                                  true,
	"stop":                                  true,
	"return_to_zero":                        true,
	"project_health_check":                  true,
	"get_audio_device_types":                true,
	"get_audio_devices":                     true,
	"get_wave_input_devices":                true,
	"get_midi_clip_notes":                   true,
	"get_midi_clip_data":                    true,
	"get_plugin_parameters":                 true,
	"plugin_grabber_explain_controls":       true,
	"plugin_grabber_inspect_compressor":     true,
	"plugin_list_available":                 true,
	"plugin_search":                         true,
	"artifact_list":                         true,
	"artifact_read":                         true,
	"artifact_extract":                      true,
	"browser_fetch":                         true,
	"browser_search":                        true,
	"transport_option_stop_return_to_start": true,
}

func CommandName(cmd map[string]any) string {
	return tools.CommandName(cmd)
}

func Classify(cmd map[string]any) Decision {
	name := CommandName(cmd)
	catalog := tools.DefaultCatalog()
	if name == "" {
		if toolName := strings.TrimSpace(fmt.Sprint(cmd["tool"])); toolName != "" && toolName != "<nil>" {
			if spec, ok := catalog.LookupTool(toolName); ok {
				return decisionForSpec(cmd, spec)
			}
			return Decision{
				Command: cmd,
				Name:    toolName,
				Risk:    RiskConfirm,
				Reason:  "未知工具需要先预览并确认",
			}
		}
	}
	if name == "" {
		return Decision{
			Command: cmd,
			Name:    "",
			Risk:    RiskConfirm,
			Reason:  "缺少 cmd/action/command 字段",
		}
	}
	if spec, ok := catalog.LookupCommand(name); ok {
		return decisionForSpec(cmd, spec)
	}
	if spec, ok := catalog.LookupTool(name); ok {
		return decisionForSpec(cmd, spec)
	}
	if directCommands[name] {
		return Decision{
			Command: cmd,
			Name:    name,
			Risk:    RiskDirect,
			Reason:  "只读或低风险操作",
		}
	}
	return Decision{
		Command: cmd,
		Name:    name,
		Risk:    RiskConfirm,
		Reason:  "该命令可能修改 DAW 工程或风险未知",
	}
}

func decisionForSpec(cmd map[string]any, spec tools.CommandSpec) Decision {
	risk := RiskDirect
	reason := "只读或低风险操作"
	if spec.RequiresConfirmation || spec.RiskLevel == tools.RiskConfirm {
		risk = RiskConfirm
		reason = "该命令需要先预览并确认"
	} else if spec.RiskLevel == tools.RiskUndoable {
		risk = RiskUndoable
		reason = "小型工程编辑，可直接执行并支持撤销"
	}
	return Decision{
		Command: cmd,
		Name:    spec.CommandName,
		Risk:    risk,
		Reason:  reason,
	}
}

func Analyze(commands []map[string]any) []Decision {
	out := make([]Decision, 0, len(commands))
	for _, cmd := range commands {
		out = append(out, Classify(cmd))
	}
	return out
}

func NeedsConfirmation(decisions []Decision) bool {
	for _, d := range decisions {
		if d.Risk == RiskConfirm {
			return true
		}
	}
	return false
}

func DirectCommandNames() []string {
	catalog := tools.DefaultCatalog()
	names := catalog.DirectCommandNames()
	seen := make(map[string]bool, len(names)+len(directCommands))
	for _, name := range names {
		seen[name] = true
	}
	for name := range directCommands {
		if !seen[name] {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func Preview(decisions []Decision) string {
	if len(decisions) == 0 {
		return ""
	}
	var b strings.Builder
	for i, d := range decisions {
		raw, _ := json.Marshal(d.Command)
		fmt.Fprintf(&b, "%d. %s [%s] %s\n   %s", i+1, fallbackName(d.Name), d.Risk, d.Reason, string(raw))
		if i != len(decisions)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func fallbackName(name string) string {
	if strings.TrimSpace(name) == "" {
		return "<unknown>"
	}
	return name
}
