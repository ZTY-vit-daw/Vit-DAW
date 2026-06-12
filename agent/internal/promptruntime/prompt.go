package promptruntime

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	"vit-daw-agent/internal/llm"
)

type SectionKind string

const (
	SectionStatic      SectionKind = "static"
	SectionSession     SectionKind = "session"
	SectionDelta       SectionKind = "delta"
	SectionRuntime     SectionKind = "runtime"
	SectionCurrentUser SectionKind = "current_user"
)

type Section struct {
	ID       string
	Kind     SectionKind
	Title    string
	Content  string
	CacheKey string
	Stable   bool
}

type AssemblyInput struct {
	SystemSections []Section
	History        []llm.Message
	UserSections   []Section
}

type Assembly struct {
	Messages    []llm.Message
	Fingerprint string
	Stats       AssemblyStats
}

type AssemblyStats struct {
	SectionCount int
	CharCount    int
	ByKind       map[SectionKind]int
}

func TextSection(kind SectionKind, id, title, content string, stable bool) Section {
	return Section{
		ID:      strings.TrimSpace(id),
		Kind:    kind,
		Title:   strings.TrimSpace(title),
		Content: strings.TrimSpace(content),
		Stable:  stable,
	}
}

func Build(input AssemblyInput) Assembly {
	messages := make([]llm.Message, 0, 2+len(input.History))
	if content := renderSections(input.SystemSections); content != "" {
		messages = append(messages, llm.Message{Role: "system", Content: content})
	}
	messages = append(messages, normalizedHistory(input.History)...)
	if content := renderSections(input.UserSections); content != "" {
		messages = append(messages, llm.Message{Role: "user", Content: content})
	}
	return Assembly{
		Messages:    messages,
		Fingerprint: fingerprint(input),
		Stats:       stats(input),
	}
}

func (s AssemblyStats) Map() map[string]any {
	byKind := make(map[string]int, len(s.ByKind))
	for kind, count := range s.ByKind {
		byKind[string(kind)] = count
	}
	return map[string]any{
		"section_count": s.SectionCount,
		"char_count":    s.CharCount,
		"by_kind":       byKind,
	}
}

func renderSections(sections []Section) string {
	parts := make([]string, 0, len(sections))
	for _, section := range sections {
		content := strings.TrimSpace(section.Content)
		if content == "" {
			continue
		}
		title := strings.TrimSpace(section.Title)
		if title != "" {
			content = title + ":\n" + content
		}
		parts = append(parts, content)
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func normalizedHistory(history []llm.Message) []llm.Message {
	out := make([]llm.Message, 0, len(history))
	for _, msg := range history {
		role := strings.TrimSpace(msg.Role)
		content := strings.TrimSpace(msg.Content)
		if role == "" || content == "" {
			continue
		}
		out = append(out, llm.Message{Role: role, Content: content})
	}
	return out
}

func stats(input AssemblyInput) AssemblyStats {
	out := AssemblyStats{ByKind: map[SectionKind]int{}}
	for _, section := range append(append([]Section{}, input.SystemSections...), input.UserSections...) {
		content := strings.TrimSpace(section.Content)
		if content == "" {
			continue
		}
		out.SectionCount++
		out.CharCount += len([]rune(content))
		out.ByKind[section.Kind]++
	}
	return out
}

func fingerprint(input AssemblyInput) string {
	type fingerprintSection struct {
		ID       string      `json:"id,omitempty"`
		Kind     SectionKind `json:"kind,omitempty"`
		Title    string      `json:"title,omitempty"`
		Content  string      `json:"content,omitempty"`
		CacheKey string      `json:"cache_key,omitempty"`
		Stable   bool        `json:"stable,omitempty"`
	}
	type fingerprintInput struct {
		SystemSections []fingerprintSection `json:"system_sections,omitempty"`
		History        []llm.Message        `json:"history,omitempty"`
		UserSections   []fingerprintSection `json:"user_sections,omitempty"`
	}
	normalizeSections := func(sections []Section) []fingerprintSection {
		out := make([]fingerprintSection, 0, len(sections))
		for _, section := range sections {
			out = append(out, fingerprintSection{
				ID:       strings.TrimSpace(section.ID),
				Kind:     section.Kind,
				Title:    strings.TrimSpace(section.Title),
				Content:  strings.TrimSpace(section.Content),
				CacheKey: strings.TrimSpace(section.CacheKey),
				Stable:   section.Stable,
			})
		}
		return out
	}
	payload, _ := json.Marshal(fingerprintInput{
		SystemSections: normalizeSections(input.SystemSections),
		History:        normalizedHistory(input.History),
		UserSections:   normalizeSections(input.UserSections),
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}
