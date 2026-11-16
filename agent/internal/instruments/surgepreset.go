package instruments

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FactoryPatch is one read-only Surge XT factory preset discovered on disk.
// HeaderOK records whether the file starts with the VST chunk preset magic
// (CcnK / FPCh); files without it are kept but flagged instead of silently
// dropped.
type FactoryPatch struct {
	Category string
	Name     string
	Path     string
	HeaderOK bool
}

// CategoryCount summarizes one factory category directory.
type CategoryCount struct {
	Category  string
	Count     int
	Invalid   int
	Semantics []Class
}

// FactoryIndex is a deterministic, read-only index of one Surge XT factory
// patch tree (root/<category>/*.fxp). It carries no write authority and no
// preset content beyond names and paths.
type FactoryIndex struct {
	Root         string
	Patches      []FactoryPatch
	InvalidCount int
}

const factoryRootEnv = "VIT_SURGE_FACTORY_ROOT"

// DefaultFactoryRoot returns the Surge XT factory patch root. The
// VIT_SURGE_FACTORY_ROOT environment variable overrides it (used by tests
// and by machines with a non-standard install); the default follows the
// Surge XT Windows installer layout under ProgramData.
func DefaultFactoryRoot() string {
	if override := strings.TrimSpace(os.Getenv(factoryRootEnv)); override != "" {
		return filepath.Clean(override)
	}
	programData := os.Getenv("ProgramData")
	if programData == "" {
		return ""
	}
	return filepath.Join(programData, "Surge XT", "patches_factory")
}

// BuildFactoryIndex walks one level of category directories under root and
// indexes every .fxp file directly inside them. Nested subdirectories are
// deliberately not indexed (the Surge XT install keeps tutorial content
// there, e.g. Tutorials/Formula Modulator); the excluded remainder is
// visible by comparing len(Patches) with the on-disk file count. It errors
// when the root is missing so callers can degrade honestly instead of
// matching against a phantom preset library.
func BuildFactoryIndex(root string) (FactoryIndex, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return FactoryIndex{}, fmt.Errorf("surge factory root is empty")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return FactoryIndex{Root: root}, fmt.Errorf("surge factory root %q unavailable: %w", root, err)
	}
	index := FactoryIndex{Root: root}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		category := entry.Name()
		files, err := os.ReadDir(filepath.Join(root, category))
		if err != nil {
			continue
		}
		for _, file := range files {
			if file.IsDir() || !strings.EqualFold(filepath.Ext(file.Name()), ".fxp") {
				continue
			}
			full := filepath.Join(root, category, file.Name())
			patch := FactoryPatch{
				Category: category,
				Name:     strings.TrimSuffix(file.Name(), filepath.Ext(file.Name())),
				Path:     full,
				HeaderOK: hasChunkPresetHeader(full),
			}
			if !patch.HeaderOK {
				index.InvalidCount++
			}
			index.Patches = append(index.Patches, patch)
		}
	}
	sort.Slice(index.Patches, func(i, j int) bool {
		if index.Patches[i].Category != index.Patches[j].Category {
			return index.Patches[i].Category < index.Patches[j].Category
		}
		return index.Patches[i].Name < index.Patches[j].Name
	})
	return index, nil
}

// CategoryCounts returns per-category summaries in category order.
func (ix FactoryIndex) CategoryCounts() []CategoryCount {
	type tally struct{ count, invalid int }
	byCategory := map[string]tally{}
	for _, patch := range ix.Patches {
		row := byCategory[patch.Category]
		row.count++
		if !patch.HeaderOK {
			row.invalid++
		}
		byCategory[patch.Category] = row
	}
	out := make([]CategoryCount, 0, len(byCategory))
	for category, row := range byCategory {
		out = append(out, CategoryCount{
			Category:  category,
			Count:     row.count,
			Invalid:   row.invalid,
			Semantics: CategorySemantics(category),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Category < out[j].Category })
	return out
}

// CategorySemantics maps a Surge XT factory category onto instrument
// classes. The Keys category is mixed and resolved per patch name by
// PatchSemantics; here it reports the classes it can contribute.
func CategorySemantics(category string) []Class {
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "keys":
		return []Class{ClassEPiano, ClassOrgan, ClassHarpsi, ClassKeysMisc}
	case "basses":
		return []Class{ClassBass}
	case "percussion":
		return []Class{ClassDrums}
	case "pads":
		return []Class{ClassPad}
	case "leads":
		return []Class{ClassLead}
	case "plucks":
		return []Class{ClassPluck}
	case "polysynths":
		return []Class{ClassPolysynth}
	case "sequences":
		return []Class{ClassSequence}
	case "brass":
		return []Class{ClassBrass}
	case "winds":
		return []Class{ClassWind}
	case "fx":
		return []Class{ClassSynthFX}
	default:
		return nil
	}
}

// PatchSemantics resolves one factory patch onto an instrument class. Keys
// patches are split by name (EP/organ/harpsi); everything else follows the
// category map. The bool is false for categories without an instrument-class
// mapping (Templates, Tutorials, Splits, MPE, Vocoder, Chords, unknown).
func PatchSemantics(patch FactoryPatch) (Class, bool) {
	category := strings.ToLower(strings.TrimSpace(patch.Category))
	if category == "keys" {
		name := strings.ToLower(patch.Name)
		switch {
		case strings.Contains(name, "ep"), strings.Contains(name, "suitcase"):
			return ClassEPiano, true
		case strings.Contains(name, "organ"):
			return ClassOrgan, true
		case strings.Contains(name, "harpsi"):
			return ClassHarpsi, true
		default:
			return ClassKeysMisc, true
		}
	}
	classes := CategorySemantics(category)
	if len(classes) == 0 {
		return ClassUnknown, false
	}
	return classes[0], true
}

// FilterByClass returns the valid patches whose semantics match the class,
// in index order.
func (ix FactoryIndex) FilterByClass(class Class) []FactoryPatch {
	var out []FactoryPatch
	for _, patch := range ix.Patches {
		if !patch.HeaderOK {
			continue
		}
		if patchClass, ok := PatchSemantics(patch); ok && patchClass == class {
			out = append(out, patch)
		}
	}
	return out
}

// hasChunkPresetHeader reads the first 12 bytes and checks the VST chunk
// preset magic: "CcnK" at offset 0 and "FPCh" at offset 8.
func hasChunkPresetHeader(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	header := make([]byte, 12)
	read, err := file.Read(header)
	if err != nil || read < 12 {
		return false
	}
	return string(header[0:4]) == "CcnK" && string(header[8:12]) == "FPCh"
}
