package promptruntime

import (
	"strings"
	"testing"

	"vit-daw-agent/internal/llm"
)

func TestBuildKeepsSectionAndHistoryOrder(t *testing.T) {
	assembly := Build(AssemblyInput{
		SystemSections: []Section{
			TextSection(SectionStatic, "rules", "Rules", "return json", true),
			TextSection(SectionSession, "catalog", "Catalog", "track.add", true),
		},
		History: []llm.Message{
			{Role: "user", Content: "previous question"},
			{Role: "assistant", Content: "previous answer"},
		},
		UserSections: []Section{
			TextSection(SectionRuntime, "snapshot", "Context snapshot JSON", `{"track_id":"1"}`, false),
			TextSection(SectionCurrentUser, "user", "", "create a track", false),
		},
	})

	if len(assembly.Messages) != 4 {
		t.Fatalf("messages = %+v", assembly.Messages)
	}
	if assembly.Messages[0].Role != "system" || !strings.Contains(assembly.Messages[0].Content, "Rules:\nreturn json\n\nCatalog:\ntrack.add") {
		t.Fatalf("system message not assembled in section order:\n%s", assembly.Messages[0].Content)
	}
	if assembly.Messages[1].Content != "previous question" || assembly.Messages[2].Content != "previous answer" {
		t.Fatalf("history order changed: %+v", assembly.Messages)
	}
	if assembly.Messages[3].Role != "user" || !strings.Contains(assembly.Messages[3].Content, "Context snapshot JSON:\n") || !strings.HasSuffix(assembly.Messages[3].Content, "create a track") {
		t.Fatalf("user sections not assembled in order:\n%s", assembly.Messages[3].Content)
	}
}

func TestBuildFingerprintIsStableAndChangesWithRuntime(t *testing.T) {
	input := AssemblyInput{
		SystemSections: []Section{TextSection(SectionStatic, "rules", "", "return json", true)},
		UserSections:   []Section{TextSection(SectionRuntime, "snapshot", "", `{"track_id":"1"}`, false)},
	}
	first := Build(input)
	second := Build(input)
	if first.Fingerprint == "" || first.Fingerprint != second.Fingerprint {
		t.Fatalf("fingerprint not stable: first=%q second=%q", first.Fingerprint, second.Fingerprint)
	}
	input.UserSections[0].Content = `{"track_id":"2"}`
	changed := Build(input)
	if changed.Fingerprint == first.Fingerprint {
		t.Fatalf("runtime content change did not change fingerprint: %q", changed.Fingerprint)
	}
}

func TestBuildStatsByKind(t *testing.T) {
	assembly := Build(AssemblyInput{
		SystemSections: []Section{
			TextSection(SectionStatic, "a", "", "alpha", true),
			TextSection(SectionStatic, "empty", "", " ", true),
		},
		UserSections: []Section{
			TextSection(SectionRuntime, "b", "", "bravo", false),
			TextSection(SectionCurrentUser, "c", "", "charlie", false),
		},
	})
	if assembly.Stats.SectionCount != 3 {
		t.Fatalf("section count = %d", assembly.Stats.SectionCount)
	}
	if assembly.Stats.ByKind[SectionStatic] != 1 || assembly.Stats.ByKind[SectionRuntime] != 1 || assembly.Stats.ByKind[SectionCurrentUser] != 1 {
		t.Fatalf("by kind = %+v", assembly.Stats.ByKind)
	}
	statsMap := assembly.Stats.Map()
	if statsMap["section_count"] != 3 {
		t.Fatalf("stats map = %+v", statsMap)
	}
}
