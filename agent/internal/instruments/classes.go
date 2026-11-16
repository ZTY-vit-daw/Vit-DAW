package instruments

import "strings"

// Class is a canonical instrument class used by the matching face. Classes
// describe the requested timbre family, not a specific plugin.
type Class string

const (
	ClassPiano      Class = "piano"
	ClassEPiano     Class = "epiano"
	ClassHarpsi     Class = "harpsichord"
	ClassClavinet   Class = "clavinet"
	ClassOrgan      Class = "organ"
	ClassBass       Class = "bass"
	ClassDrums      Class = "drums"
	ClassGuitar     Class = "guitar"
	ClassStrings    Class = "strings"
	ClassBrass      Class = "brass"
	ClassWind       Class = "wind"
	ClassMallet     Class = "mallet"
	ClassChoir      Class = "choir"
	ClassPad        Class = "pad"
	ClassLead       Class = "lead"
	ClassPluck      Class = "pluck"
	ClassPolysynth  Class = "polysynth"
	ClassSequence   Class = "sequence"
	ClassSynthFX    Class = "synth_fx"
	ClassKeysMisc   Class = "keys_misc"
	ClassPercMissed Class = "percussive_melodic"
	ClassUnknown    Class = ""
)

// classTerms are the deterministic word rules (Chinese and English) that map
// free text onto one instrument class. Rules run in declaration order and
// each class's terms in listed order, so drums outranks bass on "bass drum"
// and the specific piano-family classes outrank the generic piano class on
// "电钢琴"/"electric piano". Matching is case-insensitive substring matching
// on the normalized query.
var classTerms = []struct {
	class Class
	terms []string
}{
	{ClassDrums, []string{"bass drum", "架子鼓", "鼓组", "底鼓", "军鼓", "打击乐", "drum kit", "kick", "snare", "hi-hat", "hihat", "hi hat", "cymbal", "hat", "drums", "drum", "鼓"}},
	{ClassEPiano, []string{"电钢琴", "电钢", "电子钢琴", "electric piano", "e-piano", "epiano", "rhodes", "wurlitzer", "electric grand"}},
	{ClassHarpsi, []string{"羽管键琴", "大键琴", "harpsichord", "harpsi"}},
	{ClassClavinet, []string{"克拉维诺", "clavinet", "clav"}},
	{ClassPiano, []string{"三角钢琴", "原声钢琴", "大钢琴", "acoustic piano", "grand piano", "钢琴", "piano"}},
	{ClassOrgan, []string{"管风琴", "风琴", "hammond", "drawbar", "organ"}},
	{ClassBass, []string{"低音提琴", "贝司", "贝斯", "bass"}},
	{ClassGuitar, []string{"吉他", "guitar"}},
	{ClassStrings, []string{"弦乐", "小提琴", "大提琴", "中提琴", "violin", "viola", "cello", "strings", "contrabass"}},
	{ClassBrass, []string{"铜管", "小号", "长号", "大号", "trumpet", "trombone", "tuba", "french horn", "horn", "brass"}},
	{ClassWind, []string{"萨克斯", "单簧管", "双簧管", "长笛", "sax", "clarinet", "oboe", "flute", "piccolo", "recorder"}},
	{ClassMallet, []string{"马林巴", "木琴", "钟琴", "颤音琴", "钢片琴", "vibraphone", "marimba", "xylophone", "glockenspiel", "celesta", "music box", "mallet"}},
	{ClassChoir, []string{"合唱", "人声", "唱诗", "choir", "voice", "vocal", "aahs"}},
	{ClassPad, []string{"铺底", "氛围", "pad", "pads", "ambient"}},
	{ClassLead, []string{"主奏", "lead"}},
	{ClassPluck, []string{"拨弦", "pluck"}},
	{ClassSequence, []string{"序列", "琶音", "sequence"}},
	{ClassPolysynth, []string{"合成器", "synth", "polysynth", "poly"}},
}

// gmProgramClass maps GM program families (and selected individual programs)
// onto instrument classes. Only confident mappings are listed; unmapped GM
// programs degrade honestly instead of borrowing a neighbor class.
var gmProgramClass = map[int]Class{
	1: ClassPiano, 2: ClassPiano, 3: ClassEPiano, 4: ClassPiano,
	5: ClassEPiano, 6: ClassEPiano, 7: ClassHarpsi, 8: ClassClavinet,
	17: ClassOrgan, 18: ClassOrgan, 19: ClassOrgan, 20: ClassOrgan, 21: ClassOrgan,
	33: ClassBass, 34: ClassBass, 35: ClassBass, 36: ClassBass, 37: ClassBass, 38: ClassBass,
	39: ClassBass, 40: ClassBass,
	57: ClassBrass, 58: ClassBrass, 59: ClassBrass, 60: ClassBrass,
	61: ClassBrass, 62: ClassBrass, 63: ClassBrass, 64: ClassBrass,
	81: ClassLead, 82: ClassLead, 83: ClassLead, 84: ClassLead,
	85: ClassLead, 86: ClassLead, 87: ClassLead, 88: ClassLead,
	89: ClassPad, 90: ClassPad, 91: ClassPad, 92: ClassPad,
	93: ClassPad, 94: ClassPad, 95: ClassPad, 96: ClassPad,
	113: ClassPercMissed, 114: ClassPercMissed, 115: ClassPercMissed, 116: ClassPercMissed,
	117: ClassPercMissed, 118: ClassPercMissed, 119: ClassPercMissed, 120: ClassPercMissed,
}

// ClassForGMProgram returns the instrument class for a GM program number
// (1-128). The second return is false when the program has no confident
// local-candidate mapping.
func ClassForGMProgram(number int) (Class, bool) {
	class, ok := gmProgramClass[number]
	return class, ok
}

// ClassForQuery scores the class term rules against a free-text query. Rules
// are evaluated in declaration order and each class's terms are checked in
// order, so multi-word phrases listed first win over shorter tokens
// ("bass drum" resolves to drums before the bass rule sees "bass"). It
// returns ClassUnknown when nothing matches — the caller degrades honestly.
func ClassForQuery(query string) (Class, string) {
	lower := normalizeQuery(query)
	if lower == "" {
		return ClassUnknown, ""
	}
	for _, rule := range classTerms {
		for _, term := range rule.terms {
			normalized := normalizeQuery(term)
			if normalized != "" && strings.Contains(lower, normalized) {
				return rule.class, term
			}
		}
	}
	return ClassUnknown, ""
}
