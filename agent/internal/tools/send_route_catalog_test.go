package tools

import "testing"

// AGENT-SENDROUTE-SEMANTICS-1 验收：5 命令进 harness 命令表（catalog），
// 变更类别对齐 rack 边（写=confirm，读=direct），下划线别名可解析。

func TestSendRouteCommandRegistration(t *testing.T) {
	c := DefaultCatalog()
	cases := []struct {
		command string
		tool    string
		mutates bool
		confirm bool
	}{
		{"track.send.connect", "track.send.connect", true, true},
		{"track.send.set_amount", "track.send.set_amount", true, false},
		{"track.send.disconnect", "track.send.disconnect", true, true},
		{"track.send.list", "track.send.list", false, false},
		{"track.send.set_fx_role", "track.send.set_fx_role", true, true},
	}
	for _, tc := range cases {
		spec, ok := c.LookupCommand(tc.command)
		if !ok {
			t.Fatalf("command %s not registered", tc.command)
		}
		if spec.MutatesProject != tc.mutates {
			t.Fatalf("%s mutates = %v, want %v", tc.command, spec.MutatesProject, tc.mutates)
		}
		if spec.RequiresConfirmation != tc.confirm {
			t.Fatalf("%s confirm = %v, want %v", tc.command, spec.RequiresConfirmation, tc.confirm)
		}
		underscore := underscoreAlias(tc.command)
		if underscore != tc.command {
			if _, ok := c.LookupTool(underscore); !ok {
				t.Fatalf("underscore tool alias %s not registered", underscore)
			}
		}
	}
}

func underscoreAlias(dotted string) string {
	out := make([]byte, 0, len(dotted))
	for _, r := range dotted {
		if r == '.' {
			r = '_'
		}
		out = append(out, byte(r))
	}
	return string(out)
}
