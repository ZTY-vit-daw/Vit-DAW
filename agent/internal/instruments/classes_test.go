package instruments

import "testing"

func TestClassForQuery(t *testing.T) {
	cases := []struct {
		query string
		want  Class
	}{
		{"这曲用钢琴", ClassPiano},
		{"换个钢琴音色", ClassPiano},
		{"piano", ClassPiano},
		{"acoustic piano", ClassPiano},
		{"电钢琴", ClassEPiano},
		{"电钢", ClassEPiano},
		{"electric piano", ClassEPiano},
		{"rhodes", ClassEPiano},
		{"羽管键琴", ClassHarpsi},
		{"harpsichord", ClassHarpsi},
		{"clavinet", ClassClavinet},
		{"管风琴", ClassOrgan},
		{"hammond", ClassOrgan},
		{"贝斯", ClassBass},
		{"bass", ClassBass},
		{"低音提琴", ClassBass},
		{"bass drum", ClassDrums},
		{"底鼓", ClassDrums},
		{"鼓组", ClassDrums},
		{"用鼓", ClassDrums},
		{"drum kit", ClassDrums},
		{"closed hat", ClassDrums},
		{"吉他", ClassGuitar},
		{"弦乐", ClassStrings},
		{"violin", ClassStrings},
		{"铜管", ClassBrass},
		{"trumpet", ClassBrass},
		{"长笛", ClassWind},
		{"sax", ClassWind},
		{"马林巴", ClassMallet},
		{"marimba", ClassMallet},
		{"合唱", ClassChoir},
		{"铺底", ClassPad},
		{"pad", ClassPad},
		{"主奏", ClassLead},
		{"lead", ClassLead},
		{"拨弦", ClassPluck},
		{"序列", ClassSequence},
		{"合成器", ClassPolysynth},
		{"synth", ClassPolysynth},
		{"水果沙拉", ClassUnknown},
		{"", ClassUnknown},
	}
	for _, tc := range cases {
		class, _ := ClassForQuery(tc.query)
		if class != tc.want {
			t.Fatalf("ClassForQuery(%q) = %q, want %q", tc.query, class, tc.want)
		}
	}
}

func TestClassForGMProgram(t *testing.T) {
	cases := []struct {
		number int
		want   Class
		ok     bool
	}{
		{1, ClassPiano, true},
		{5, ClassEPiano, true},
		{8, ClassClavinet, true},
		{82, ClassLead, true},
		{90, ClassPad, true},
		{41, ClassUnknown, false},  // Violin: no confident local mapping
		{105, ClassUnknown, false}, // Sitar
		{117, ClassPercMissed, true},
		{0, ClassUnknown, false},
		{129, ClassUnknown, false},
	}
	for _, tc := range cases {
		class, ok := ClassForGMProgram(tc.number)
		if class != tc.want || ok != tc.ok {
			t.Fatalf("ClassForGMProgram(%d) = %q ok=%v, want %q ok=%v", tc.number, class, ok, tc.want, tc.ok)
		}
	}
}
