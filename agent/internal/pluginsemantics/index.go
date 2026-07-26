package pluginsemantics

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const SchemaVersion = 1

type Index struct {
	SchemaVersion int            `json:"schema_version"`
	BuiltAt       time.Time      `json:"built_at"`
	Source        string         `json:"source,omitempty"`
	Entries       []Entry        `json:"entries"`
	Summary       map[string]int `json:"summary,omitempty"`
	Warnings      []string       `json:"warnings,omitempty"`
}

type Entry struct {
	ID              string        `json:"id"`
	Name            string        `json:"name,omitempty"`
	DescriptiveName string        `json:"descriptive_name,omitempty"`
	Manufacturer    string        `json:"manufacturer,omitempty"`
	Format          string        `json:"format,omitempty"`
	Category        string        `json:"category,omitempty"`
	Identifier      string        `json:"identifier,omitempty"`
	PluginPath      string        `json:"plugin_path,omitempty"`
	IsInstrument    bool          `json:"is_instrument,omitempty"`
	PrimaryType     string        `json:"primary_type,omitempty"`
	SemanticTypes   []SemanticTag `json:"semantic_types,omitempty"`
	Confidence      int           `json:"confidence,omitempty"`
	Evidence        []Evidence    `json:"evidence,omitempty"`
	UpdatedAt       time.Time     `json:"updated_at"`
	SearchScore     int           `json:"search_score,omitempty"`
}

type SemanticTag struct {
	Type  string `json:"type"`
	Score int    `json:"score"`
}

type Evidence struct {
	Kind    string `json:"kind"`
	Value   string `json:"value"`
	Type    string `json:"type"`
	Score   int    `json:"score"`
	Message string `json:"message,omitempty"`
}

type SearchOptions struct {
	Query              string
	Type               string
	Manufacturer       string
	Format             string
	Limit              int
	IncludeInstruments bool
}

func DefaultPath() (string, error) {
	if override := strings.TrimSpace(os.Getenv("VIT_PLUGIN_SEMANTICS_PATH")); override != "" {
		return override, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("plugin semantics: user home: %w", err)
	}
	return filepath.Join(home, ".vit", "plugin_semantics.json"), nil
}

func Load(path string) (Index, error) {
	if strings.TrimSpace(path) == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return Index{}, err
		}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return Index{}, err
	}
	var idx Index
	if err := json.Unmarshal(b, &idx); err != nil {
		return Index{}, fmt.Errorf("plugin semantics: parse %q: %w", path, err)
	}
	if idx.SchemaVersion == 0 {
		idx.SchemaVersion = SchemaVersion
	}
	return idx, nil
}

func Save(path string, idx Index) (string, error) {
	if strings.TrimSpace(path) == "" {
		var err error
		path, err = DefaultPath()
		if err != nil {
			return "", err
		}
	}
	idx.SchemaVersion = SchemaVersion
	if idx.BuiltAt.IsZero() {
		idx.BuiltAt = time.Now().UTC()
	}
	if idx.Summary == nil {
		idx.Summary = BuildSummary(idx.Entries)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return path, fmt.Errorf("plugin semantics: mkdir %q: %w", filepath.Dir(path), err)
	}
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return path, fmt.Errorf("plugin semantics: marshal: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return path, fmt.Errorf("plugin semantics: write %q: %w", path, err)
	}
	return path, nil
}

func Build(rows []map[string]any, now time.Time) Index {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	seen := map[string]bool{}
	entries := make([]Entry, 0, len(rows))
	for _, row := range rows {
		entry := Classify(row, now)
		if entry.ID == "" {
			continue
		}
		key := strings.ToLower(entry.ID)
		if seen[key] {
			continue
		}
		seen[key] = true
		entries = append(entries, entry)
	}
	sort.SliceStable(entries, func(i, j int) bool {
		a := strings.ToLower(firstNonEmpty(entries[i].Name, entries[i].DescriptiveName, entries[i].PluginPath))
		b := strings.ToLower(firstNonEmpty(entries[j].Name, entries[j].DescriptiveName, entries[j].PluginPath))
		return a < b
	})
	return Index{
		SchemaVersion: SchemaVersion,
		BuiltAt:       now,
		Source:        "plugin_list_available",
		Entries:       entries,
		Summary:       BuildSummary(entries),
	}
}

func Classify(row map[string]any, now time.Time) Entry {
	entry := Entry{
		Name:            firstString(row, "name"),
		DescriptiveName: firstString(row, "descriptive_name", "display_name"),
		Manufacturer:    firstString(row, "manufacturer", "vendor"),
		Format:          firstString(row, "format"),
		Category:        firstString(row, "category", "sub_category", "subcategory"),
		Identifier:      firstString(row, "identifier", "uid", "file_or_identifier"),
		PluginPath:      firstString(row, "plugin_path", "path", "file_path", "file_or_identifier"),
		IsInstrument:    boolValue(row["is_instrument"]) || categoryHas(entryCategory(row), "instrument"),
		UpdatedAt:       now,
	}
	entry.ID = stableID(entry)
	scores := map[string]int{}
	var evidence []Evidence
	add := func(kind, value, typ string, score int, message string) {
		if typ == "" || score <= 0 {
			return
		}
		scores[typ] += score
		evidence = append(evidence, Evidence{Kind: kind, Value: value, Type: typ, Score: score, Message: message})
	}
	classifyCategory(entry.Category, add)
	classifyText(entry, add)
	if entry.IsInstrument {
		add("metadata", "is_instrument", "instrument", 70, "plugin metadata marks it as an instrument")
	}
	entry.SemanticTypes = sortedTags(scores)
	if len(entry.SemanticTypes) > 0 {
		entry.PrimaryType = entry.SemanticTypes[0].Type
		entry.Confidence = entry.SemanticTypes[0].Score
		if entry.Confidence > 100 {
			entry.Confidence = 100
		}
	}
	entry.Evidence = evidence
	return entry
}

func Search(idx Index, opts SearchOptions) []Entry {
	limit := opts.Limit
	if limit <= 0 || limit > 50 {
		limit = 12
	}
	query := normalize(opts.Query)
	wantTypes := queryTypes(query)
	explicitType := normalize(opts.Type)
	if explicitType != "" {
		wantTypes[canonicalType(explicitType)] = true
	}
	manufacturer := normalize(opts.Manufacturer)
	format := normalize(opts.Format)
	results := make([]Entry, 0, len(idx.Entries))
	for _, entry := range idx.Entries {
		if !opts.IncludeInstruments && entry.IsInstrument && !wantTypes["instrument"] {
			continue
		}
		if manufacturer != "" && !strings.Contains(normalize(entry.Manufacturer), manufacturer) {
			continue
		}
		if format != "" && normalize(entry.Format) != format {
			continue
		}
		score := scoreEntry(entry, query, wantTypes)
		if score <= 0 {
			continue
		}
		entry.SearchScore = score
		results = append(results, entry)
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].SearchScore != results[j].SearchScore {
			return results[i].SearchScore > results[j].SearchScore
		}
		if results[i].Confidence != results[j].Confidence {
			return results[i].Confidence > results[j].Confidence
		}
		return strings.ToLower(results[i].Name) < strings.ToLower(results[j].Name)
	})
	if len(results) > limit {
		results = results[:limit]
	}
	return results
}

func Get(idx Index, idOrQuery string) (Entry, bool) {
	key := normalize(idOrQuery)
	for _, entry := range idx.Entries {
		if normalize(entry.ID) == key || normalize(entry.PluginPath) == key || normalize(entry.Identifier) == key {
			return entry, true
		}
	}
	results := Search(idx, SearchOptions{Query: idOrQuery, Limit: 1, IncludeInstruments: true})
	if len(results) == 0 {
		return Entry{}, false
	}
	return results[0], true
}

func BuildSummary(entries []Entry) map[string]int {
	out := map[string]int{}
	for _, entry := range entries {
		key := entry.PrimaryType
		if key == "" {
			key = "unknown"
		}
		out[key]++
	}
	return out
}

func IsNotExist(err error) bool {
	return errors.Is(err, os.ErrNotExist)
}

func classifyCategory(category string, add func(kind, value, typ string, score int, message string)) {
	lower := normalize(category)
	if lower == "" {
		return
	}
	categoryRules := []struct {
		needle string
		typ    string
		score  int
	}{
		{"fx|reverb", "reverb", 95},
		{"reverb", "reverb", 92},
		{"fx|delay", "delay", 95},
		{"delay", "delay", 90},
		{"fx|eq", "eq", 96},
		{"equalizer", "eq", 92},
		{"fx|dynamics", "dynamics", 92},
		{"dynamics", "dynamics", 88},
		{"fx|analyzer", "analyzer", 96},
		{"analyzer", "analyzer", 92},
		{"fx|distortion", "distortion", 90},
		{"distortion", "distortion", 88},
		{"fx|modulation", "modulation", 88},
		{"modulation", "modulation", 84},
		{"fx|filter", "filter", 90},
		{"filter", "filter", 86},
		{"pitch", "pitch", 86},
		{"instrument|synth", "synth", 96},
		{"instrument", "instrument", 90},
		{"synth", "synth", 88},
	}
	for _, rule := range categoryRules {
		if strings.Contains(lower, rule.needle) {
			add("category", category, rule.typ, rule.score, "matched plugin category")
		}
	}
}

func classifyText(entry Entry, add func(kind, value, typ string, score int, message string)) {
	text := normalize(strings.Join([]string{
		entry.Name,
		entry.DescriptiveName,
		entry.Manufacturer,
		filepath.Base(entry.PluginPath),
		entry.Identifier,
	}, " "))
	name := firstNonEmpty(entry.Name, entry.DescriptiveName, filepath.Base(entry.PluginPath))
	textRules := []struct {
		needle string
		typ    string
		score  int
	}{
		{"valhalla", "reverb", 84},
		{"supermassive", "reverb", 88},
		{"supermassive", "delay", 72},
		{"vintageverb", "reverb", 96},
		{"room", "reverb", 70},
		{"plate", "reverb", 70},
		{"hall", "reverb", 70},
		{"shimmer", "reverb", 72},
		{"verb", "reverb", 86},
		{"reverb", "reverb", 92},
		{"delay", "delay", 92},
		{"echo", "delay", 78},
		{"eq", "eq", 78},
		{"equalizer", "eq", 92},
		{"nova", "eq", 80},
		{"nova", "dynamics", 58},
		{"pro-q", "eq", 92},
		{"compressor", "compressor", 92},
		{"comp", "compressor", 66},
		{"limiter", "limiter", 90},
		{"maximizer", "limiter", 76},
		{"gate", "gate", 78},
		{"expander", "gate", 74},
		{"span", "analyzer", 94},
		{"analyzer", "analyzer", 92},
		{"spectrum", "analyzer", 78},
		{"meter", "meter", 72},
		{"loudness", "meter", 76},
		{"saturat", "distortion", 76},
		{"distortion", "distortion", 88},
		{"overdrive", "distortion", 78},
		{"chorus", "modulation", 82},
		{"flanger", "modulation", 82},
		{"phaser", "modulation", 82},
		{"tremolo", "modulation", 76},
		{"filter", "filter", 82},
		{"pitch", "pitch", 78},
		{"autotune", "pitch", 86},
		{"synth", "synth", 88},
		{"surge", "synth", 82},
		{"sampler", "instrument", 78},
		{"piano", "instrument", 78},
		{"drum", "instrument", 74},
	}
	for _, rule := range textRules {
		if strings.Contains(text, rule.needle) {
			add("name", name, rule.typ, rule.score, "matched plugin name/manufacturer text")
		}
	}
}

func scoreEntry(entry Entry, query string, wantTypes map[string]bool) int {
	score := 0
	if query == "" && len(wantTypes) == 0 {
		score = entry.Confidence
	}
	for _, tag := range entry.SemanticTypes {
		if wantTypes[tag.Type] {
			score += 1000 + tag.Score*2
		}
	}
	haystack := normalize(strings.Join([]string{
		entry.Name,
		entry.DescriptiveName,
		entry.Manufacturer,
		entry.Category,
		entry.Format,
		entry.PluginPath,
		entry.Identifier,
		entry.PrimaryType,
	}, " "))
	if query != "" {
		if strings.Contains(haystack, query) {
			score += 700
		}
		for _, token := range strings.Fields(query) {
			if len(token) < 2 {
				continue
			}
			if strings.Contains(haystack, token) {
				score += 130
			}
		}
	}
	if score > 0 {
		score += entry.Confidence
		if !entry.IsInstrument {
			score += 60
		}
	}
	return score
}

func queryTypes(query string) map[string]bool {
	out := map[string]bool{}
	if query == "" {
		return out
	}
	rules := []struct {
		typ   string
		terms []string
	}{
		{"reverb", []string{"reverb", "verb", "space", "hall", "room", "\u6df7\u54cd"}},
		{"delay", []string{"delay", "echo", "\u5ef6\u8fdf"}},
		{"eq", []string{"eq", "equalizer", "\u5747\u8861"}},
		{"compressor", []string{"compressor", "compression", "comp", "\u538b\u7f29"}},
		{"dynamics", []string{"dynamics", "\u52a8\u6001"}},
		{"analyzer", []string{"analyzer", "spectrum", "meter", "\u5206\u6790"}},
		{"distortion", []string{"distortion", "saturation", "overdrive"}},
		{"modulation", []string{"modulation", "chorus", "flanger", "phaser"}},
		{"filter", []string{"filter"}},
		{"pitch", []string{"pitch", "tune"}},
		{"synth", []string{"synth", "instrument", "\u5408\u6210\u5668", "\u4e50\u5668"}},
	}
	for _, rule := range rules {
		for _, term := range rule.terms {
			if strings.Contains(query, term) {
				out[rule.typ] = true
				break
			}
		}
	}
	return out
}

func sortedTags(scores map[string]int) []SemanticTag {
	out := make([]SemanticTag, 0, len(scores))
	for typ, score := range scores {
		if score <= 0 {
			continue
		}
		out = append(out, SemanticTag{Type: typ, Score: score})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Type < out[j].Type
	})
	for i := range out {
		if out[i].Score > 100 {
			out[i].Score = 100
		}
	}
	return out
}

func stableID(entry Entry) string {
	// Shell formats such as Waves expose many distinct plugins from one file.
	// JUCE's identifier names the member; the file path only names the shell.
	for _, value := range []string{entry.Identifier, entry.PluginPath, entry.Format + ":" + entry.Manufacturer + ":" + entry.Name} {
		value = strings.TrimSpace(value)
		if value != "" && value != "::" {
			return value
		}
	}
	return ""
}

func canonicalType(value string) string {
	value = normalize(value)
	aliases := map[string]string{
		"verb":        "reverb",
		"space":       "reverb",
		"equalizer":   "eq",
		"compression": "compressor",
		"comp":        "compressor",
		"instrument":  "instrument",
	}
	if mapped := aliases[value]; mapped != "" {
		return mapped
	}
	return value
}

func categoryHas(category, term string) bool {
	return strings.Contains(normalize(category), normalize(term))
}

func entryCategory(row map[string]any) string {
	return firstString(row, "category", "sub_category", "subcategory")
}

func firstString(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := row[key]; ok {
			s := strings.TrimSpace(fmt.Sprint(v))
			if s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return ""
}

func boolValue(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		switch strings.ToLower(strings.TrimSpace(x)) {
		case "true", "1", "yes", "on", "instrument":
			return true
		}
	case int:
		return x != 0
	case int64:
		return x != 0
	case float64:
		return x != 0
	case json.Number:
		n, _ := strconv.Atoi(x.String())
		return n != 0
	}
	return false
}

func normalize(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
