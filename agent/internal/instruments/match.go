package instruments

import (
	"fmt"
	"sort"
	"strings"

	"vit-daw-agent/internal/pluginsemantics"
)

// InstalledInstrument is one installed instrument plugin as seen by the
// matching face. It is a narrowed copy of a pluginsemantics entry so this
// package stays decoupled from the index schema.
type InstalledInstrument struct {
	Name         string
	Manufacturer string
	Identifier   string
	PluginPath   string
	IsInstrument bool
}

// FromPluginSemantics narrows plugin semantic index entries into the
// matching face input. Only entries marked as instruments are kept.
func FromPluginSemantics(entries []pluginsemantics.Entry) []InstalledInstrument {
	out := make([]InstalledInstrument, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsInstrument {
			continue
		}
		out = append(out, InstalledInstrument{
			Name:         entry.Name,
			Manufacturer: entry.Manufacturer,
			Identifier:   entry.Identifier,
			PluginPath:   entry.PluginPath,
			IsInstrument: true,
		})
	}
	return out
}

// Candidate is one loadable instrument candidate for the requested class.
// Kind distinguishes dedicated timbre instruments (their init patch already
// is the requested timbre, e.g. Grand Rhapsody Piano for piano) from synth
// preset libraries (the timbre comes from choosing a preset, e.g. Surge XT
// with a factory patch).
const (
	CandidateKindDedicated = "dedicated_instrument"
	CandidateKindSynth     = "synth_preset_library"
)

type Candidate struct {
	Installed InstalledInstrument
	Kind      string
	Evidence  string
}

// PresetHint is one factory preset group that can serve the requested
// class. Approximate marks groups that only approximate the class timbre
// (for example Surge Percussion patches are chromatic synth patches, not a
// GM drum kit); the note carries the honest limitation.
type PresetHint struct {
	Source      string
	Category    string
	Class       Class
	Count       int
	Examples    []string
	Approximate bool
	Note        string
}

// Degrade is the fail-honest degradation result: a typed code plus an honest
// user-facing message. It is returned instead of an invented candidate.
type Degrade struct {
	Code    string
	Message string
}

// Degrade codes.
const (
	DegradeNoClass          = "no_instrument_class"
	DegradeNoCandidate      = "no_installed_candidate"
	DegradeNoSurgeIndex     = "surge_factory_index_unavailable"
	DegradeEmptyInstallList = "installed_instrument_list_empty"
)

// Result is the deterministic outcome of one instrument matching call.
type Result struct {
	Query       string
	Class       Class
	Evidence    string
	GMProgram   *GMProgram
	Candidates  []Candidate
	PresetHints []PresetHint
	Degrade     *Degrade
}

// dedicatedRules map an instrument class onto installed-name needles that
// identify a dedicated timbre instrument. Needles are matched against the
// normalized plugin name.
var dedicatedRules = []struct {
	class   Class
	needles []string
}{
	{ClassPiano, []string{"grand rhapsody piano"}},
	{ClassEPiano, []string{"electric88", "electric200", "electric grand 80"}},
	{ClassClavinet, []string{"clavinet"}},
	{ClassBass, []string{"bass fingers", "bass slapper"}},
}

// surgeIdentifier is the plugin name that identifies the Surge XT synth
// (the instrument, not the "Surge XT Effects" FX sibling).
const surgeName = "surge xt"

// hintLimits bound the preset hint payload.
const (
	hintExampleLimit = 5
	hintCategoryCap  = 2
)

// Match maps a free-text instrument request onto installed candidates and
// Surge factory preset hints. Resolution order: exact GM program token
// ("gm:82"), GM official-name containment, then class word rules. Pass the
// zero FactoryIndex when the Surge factory tree is unavailable. When no
// class resolves, or the class has neither a dedicated instrument nor a
// usable preset face, the result carries a typed Degrade instead of a guess.
func Match(query string, installed []InstalledInstrument, surge FactoryIndex) Result {
	result := Result{Query: query}
	trimmed := strings.TrimSpace(query)

	if trimmed == "" {
		result.Degrade = &Degrade{Code: DegradeNoClass, Message: degradeNoClassMessage(query)}
		return result
	}
	if len(installed) == 0 {
		result.Degrade = &Degrade{Code: DegradeEmptyInstallList, Message: "本机乐器清单为空（插件语义索引未建或未传入），无法给出候选，如实报告。"}
		return result
	}

	result.resolveClass(trimmed)
	if result.Degrade != nil {
		return result
	}

	surgeInstalled := findSurge(installed)
	hints, indexAvailable := presetHints(result.Class, surge)

	for _, rule := range dedicatedRules {
		if rule.class != result.Class {
			continue
		}
		for _, candidate := range installed {
			name := normalizeQuery(candidate.Name)
			for _, needle := range rule.needles {
				if name == needle || strings.Contains(name, needle) {
					result.Candidates = append(result.Candidates, Candidate{
						Installed: candidate,
						Kind:      CandidateKindDedicated,
						Evidence:  fmt.Sprintf("dedicated %s instrument: name contains %q", result.Class, needle),
					})
					break
				}
			}
		}
	}
	if len(hints) > 0 && surgeInstalled != nil {
		result.Candidates = append(result.Candidates, Candidate{
			Installed: *surgeInstalled,
			Kind:      CandidateKindSynth,
			Evidence:  fmt.Sprintf("synth preset library: Surge XT factory categories %s", hintCategories(hints)),
		})
	}
	result.PresetHints = hints

	if len(result.Candidates) == 0 {
		if !indexAvailable && needsSurgeFace(result.Class) {
			result.Degrade = &Degrade{Code: DegradeNoSurgeIndex, Message: degradeNoSurgeIndexMessage(result.Class)}
		} else {
			result.Degrade = &Degrade{Code: DegradeNoCandidate, Message: degradeNoCandidateMessage(string(result.Class))}
		}
	}
	return result
}

// resolveClass fills Class/Evidence/GMProgram or Degrade for a non-empty
// query using the documented resolution order.
func (r *Result) resolveClass(query string) {
	lower := strings.ToLower(query)
	if strings.HasPrefix(lower, "gm:") {
		var number int
		if _, err := fmt.Sscanf(lower, "gm:%d", &number); err == nil {
			program, ok := LookupGMProgram(number)
			if !ok {
				r.Degrade = &Degrade{Code: DegradeNoClass, Message: fmt.Sprintf("GM 编号 %d 超出 1-128，无法对应标准乐器名。", number)}
				return
			}
			r.GMProgram = &program
			class, ok := ClassForGMProgram(program.Number)
			if !ok {
				r.Degrade = &Degrade{Code: DegradeNoCandidate, Message: degradeNoCandidateMessage(program.Name)}
				return
			}
			r.Class = class
			r.Evidence = fmt.Sprintf("gm program %d %q", program.Number, program.Name)
			return
		}
	}
	if program, ok := LookupGMProgramByName(query); ok {
		r.GMProgram = &program
		if class, ok := ClassForGMProgram(program.Number); ok {
			r.Class = class
			r.Evidence = fmt.Sprintf("gm program name %q", program.Name)
			return
		}
		r.Degrade = &Degrade{Code: DegradeNoCandidate, Message: degradeNoCandidateMessage(program.Name)}
		return
	}
	class, term := ClassForQuery(query)
	if class == ClassUnknown {
		r.Degrade = &Degrade{Code: DegradeNoClass, Message: degradeNoClassMessage(r.Query)}
		return
	}
	r.Class = class
	r.Evidence = fmt.Sprintf("term %q", term)
}

// presetHints builds the factory preset hints for a class. indexAvailable
// reports whether the Surge factory index was supplied at all (Root set),
// independent of whether the class has matching categories.
func presetHints(class Class, surge FactoryIndex) ([]PresetHint, bool) {
	if surge.Root == "" {
		return nil, false
	}
	var hints []PresetHint
	byCategory := map[string][]FactoryPatch{}
	for _, patch := range surge.FilterByClass(class) {
		byCategory[patch.Category] = append(byCategory[patch.Category], patch)
	}
	categories := make([]string, 0, len(byCategory))
	for category := range byCategory {
		categories = append(categories, category)
	}
	sort.Strings(categories)
	for _, category := range categories {
		if len(hints) >= hintCategoryCap {
			break
		}
		patches := byCategory[category]
		hint := PresetHint{
			Source:      "surge_xt_factory",
			Category:    category,
			Class:       class,
			Count:       len(patches),
			Approximate: categoryApproximate(class, category),
			Note:        categoryNote(class, category),
		}
		for i, patch := range patches {
			if i >= hintExampleLimit {
				break
			}
			hint.Examples = append(hint.Examples, patch.Name)
		}
		hints = append(hints, hint)
	}
	return hints, true
}

// findSurge locates the Surge XT instrument among the installed list,
// explicitly excluding the "Surge XT Effects" FX sibling.
func findSurge(installed []InstalledInstrument) *InstalledInstrument {
	for i := range installed {
		name := normalizeQuery(installed[i].Name)
		if name == surgeName && installed[i].IsInstrument {
			return &installed[i]
		}
	}
	return nil
}

// needsSurgeFace reports whether a class can only be served through the
// Surge factory preset face (no dedicated instrument rule exists for it).
func needsSurgeFace(class Class) bool {
	for _, rule := range dedicatedRules {
		if rule.class == class {
			return false
		}
	}
	return true
}

func hintCategories(hints []PresetHint) string {
	names := make([]string, 0, len(hints))
	for _, hint := range hints {
		names = append(names, hint.Category)
	}
	return strings.Join(names, ", ")
}

// categoryApproximate marks preset groups that only approximate the class.
func categoryApproximate(class Class, category string) bool {
	switch class {
	case ClassDrums:
		// Surge Percussion patches are chromatic synth percussion, not a GM
		// keyed drum kit.
		return true
	case ClassBrass, ClassWind:
		// Synth patches standing in for acoustic instrument families.
		return true
	default:
		return false
	}
}

func categoryNote(class Class, category string) string {
	switch class {
	case ClassDrums:
		return "Surge XT 打击类预设是 chromatic 合成打击（音高随键位），不是 GM 键位鼓组；GM 36/38/42 键位语义不成立，仅作打击音色近似。"
	case ClassBrass, ClassWind:
		return "Surge XT 该分类是合成器音色，仅作原声乐器的合成近似，不是采样还原。"
	default:
		return ""
	}
}

func degradeNoClassMessage(query string) string {
	return fmt.Sprintf("没能在「%s」里识别出乐器语义（支持钢琴/电钢/风琴/贝斯/鼓/弦乐/铜管/合成器等词与 GM 标准名）。请指明具体乐器，我不会随意挑一个。", strings.TrimSpace(query))
}

func degradeNoCandidateMessage(subject string) string {
	return fmt.Sprintf("本机音源库没有「%s」的可用候选：没有对应专用音源，Surge XT 出厂预设库也没有该乐器分类。如实报告，不硬造候选。", subject)
}

func degradeNoSurgeIndexMessage(class Class) string {
	return fmt.Sprintf("「%s」只能经 Surge XT 出厂预设面满足，但出厂预设库当前不可读（安装缺失或路径不可用）。无法给出预设候选，如实报告。", string(class))
}

func normalizeQuery(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
