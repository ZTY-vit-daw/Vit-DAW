package policy

import "testing"

func TestClassifyDirect(t *testing.T) {
	d := Classify(map[string]any{"cmd": "get_project_state"})
	if d.Risk != RiskDirect {
		t.Fatalf("risk = %s, want %s", d.Risk, RiskDirect)
	}
}

func TestClassifyConfirmByDefault(t *testing.T) {
	d := Classify(map[string]any{"cmd": "delete_track", "track_id": "1007"})
	if d.Risk != RiskConfirm {
		t.Fatalf("risk = %s, want %s", d.Risk, RiskConfirm)
	}
}

func TestClassifyUndoableDoesNotRequireConfirmation(t *testing.T) {
	d := Classify(map[string]any{"cmd": "set_mute", "track_id": "1007", "mute": true})
	if d.Risk != RiskUndoable {
		t.Fatalf("risk = %s, want %s", d.Risk, RiskUndoable)
	}
	if NeedsConfirmation([]Decision{d}) {
		t.Fatal("undoable command should not require confirmation")
	}
}

func TestClassifyToolFormCommand(t *testing.T) {
	d := Classify(map[string]any{"tool": "track.mute", "args": map[string]any{"mute": true}})
	if d.Name != "set_mute" || d.Risk != RiskUndoable {
		t.Fatalf("decision = %+v", d)
	}
	if NeedsConfirmation([]Decision{d}) {
		t.Fatal("undoable tool call should not require confirmation")
	}
}

func TestClassifyCommandToolAlias(t *testing.T) {
	d := Classify(map[string]any{"cmd": "rack.add_macro", "name": "Macro"})
	if d.Name != "control_add_macro" || d.Risk != RiskConfirm {
		t.Fatalf("decision = %+v", d)
	}
}

func TestCommandNameFallback(t *testing.T) {
	name := CommandName(map[string]any{"action": "play"})
	if name != "play" {
		t.Fatalf("name = %q, want play", name)
	}
}

// EDU-SYMBOL-CORE-1：只读符号分析命令按 direct 级放行（spec 与
// directCommands 兜底两条路径都必须是 direct）。
func TestAnalyzeMidiTransformationsIsDirect(t *testing.T) {
	for _, cmd := range []map[string]any{
		{"cmd": "analyze_midi_transformations", "clip_id": "clipA"},
		{"tool": "midi.analyze_transformations", "args": map[string]any{"clip_id": "clipA"}},
	} {
		d := Classify(cmd)
		if d.Risk != RiskDirect {
			t.Fatalf("risk = %s, want %s (cmd=%v)", d.Risk, RiskDirect, cmd)
		}
		if NeedsConfirmation([]Decision{d}) {
			t.Fatal("read-only analysis must not require confirmation")
		}
	}
	// 未注册裸命令名兜底路径（catalog 缺行时的防线）。
	if !directCommands["analyze_midi_transformations"] {
		t.Fatal("analyze_midi_transformations missing from directCommands fallback")
	}
}
