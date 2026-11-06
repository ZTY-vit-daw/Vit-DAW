package plugingrabber

import "testing"

// ARR-SRCLOAD-1（277 段 2，主管派发令授权的域外微补丁）：装载工作流的
// instrument 词表补 钢琴/piano 单独词——此前「加个钢琴」不含「音源」时
// intent kind 判空，装载候选搜索不带 include_instruments/type=synth，
// 钢琴类 VSTi 搜不到。本测试钉词表补丁防回退。
func TestInstrumentSubjectIncludesPiano(t *testing.T) {
	for _, text := range []string{"加个钢琴", "给这条轨装一个钢琴", "load a piano on this track", "钢琴音源"} {
		if PluginIntentKind(text) != "instrument" {
			t.Fatalf("PluginIntentKind(%q) = %q, want instrument", text, PluginIntentKind(text))
		}
	}
	// 效果器词不受补丁影响（负例钉）。
	for _, text := range []string{"加一个混响", "load a reverb"} {
		if PluginIntentKind(text) != "effect" {
			t.Fatalf("PluginIntentKind(%q) = %q, want effect", text, PluginIntentKind(text))
		}
	}
}
