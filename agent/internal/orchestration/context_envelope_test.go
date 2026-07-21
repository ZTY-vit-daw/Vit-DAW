package orchestration

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestContextEnvelopeBudgetsAllPromptComponents(t *testing.T) {
	session, err := NewSession("s1", "p1", "balance the project", EngineV1, CapabilityInvocation{
		CapabilityID: "static_mix.static_balance.v0", CapabilityVer: "v0", ProcessingPath: PathCapability,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := ContextEnvelopeRequest{
		System:  strings.Repeat("policy ", 80),
		Session: session,
		Bundle: ContextBundle{
			ID: "bundle1", Disclosure: `{"candidate_ids":["a","b"]}`,
			ArtifactRefs: []string{"capability-pack:bundle1"},
			Omissions:    map[string]OmissionStatus{"track_rows": OmissionBudget, "full_model": OmissionByReference},
		},
		Registry: []ContextEntry{
			{ID: "b2", Content: "static balance contract", Priority: 100},
			{ID: "b3", Content: "pan layout contract", Priority: 10},
			{ID: "b4", Content: "future contract", Priority: 1},
		},
		ToolSchemas: []ContextEntry{
			{ID: "project.state", Content: "read project state", Priority: 100},
			{ID: "mix.observe", Content: "read acoustic projection", Priority: 90},
		},
		ConversationTurns: []ContextEntry{{ID: "turn1", Content: "old"}, {ID: "turn2", Content: "new"}},
		Budget:            ContextWindowBudget{MaxWindowTokens: 800, OutputReserveTokens: 160, MaxRegistryEntries: 2, MaxToolSchemas: 2, MaxConversationTurns: 1},
	}
	envelope := BuildContextEnvelope(request)
	if !envelope.Valid || envelope.EstimatedInputTokens > envelope.MaxInputTokens {
		t.Fatalf("invalid envelope: %#v", envelope)
	}
	if len(envelope.Registry) != 2 || envelope.Registry[0].ID != "b2" {
		t.Fatalf("registry top-N was not deterministic: %#v", envelope.Registry)
	}
	if envelope.Omissions["registry.remainder"] != OmissionByReference || envelope.Omissions["conversation.older_turns"] != OmissionByReference {
		t.Fatalf("missing bounded omission manifest: %#v", envelope.Omissions)
	}
	if envelope.Omissions["bundle.track_rows"] != OmissionBudget || envelope.Omissions["bundle.full_model"] != OmissionByReference {
		t.Fatalf("bundle omission states were lost: %#v", envelope.Omissions)
	}
}

func TestContextEnvelopeLongConversationDoesNotGrowLinearly(t *testing.T) {
	session, err := NewSession("s1", "p1", "goal", EngineV1, CapabilityInvocation{CapabilityID: "c1", CapabilityVer: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	tail := []ContextEntry{
		{ID: "tail-1", Content: strings.Repeat("recent one ", 8)},
		{ID: "tail-2", Content: strings.Repeat("recent two ", 8)},
		{ID: "tail-3", Content: strings.Repeat("recent three ", 8)},
	}
	build := func(prefixCount int) ContextEnvelope {
		turns := make([]ContextEntry, 0, prefixCount+len(tail))
		for index := 0; index < prefixCount; index++ {
			turns = append(turns, ContextEntry{ID: fmt.Sprintf("old-%05d", index), Content: strings.Repeat("historical context ", 20), Reference: fmt.Sprintf("history:%d", index)})
		}
		turns = append(turns, tail...)
		return BuildContextEnvelope(ContextEnvelopeRequest{
			System: "system", Session: session,
			Bundle:            ContextBundle{ID: "b1", Disclosure: `{"decision":"compact"}`, ArtifactRefs: []string{"pack:b1"}},
			ConversationTurns: turns,
			Budget:            ContextWindowBudget{MaxWindowTokens: 1200, OutputReserveTokens: 200, MaxRegistryEntries: 2, MaxToolSchemas: 2, MaxConversationTurns: 3},
		})
	}
	short := build(10)
	long := build(10_000)
	if !short.Valid || !long.Valid || short.EstimatedInputTokens != long.EstimatedInputTokens || !reflect.DeepEqual(short.ConversationTurns, long.ConversationTurns) {
		t.Fatalf("conversation context grew with history: short=%#v long=%#v", short, long)
	}
	if len(long.Omissions) > 8 || long.Omissions["conversation.older_turns"] != OmissionByReference {
		t.Fatalf("omission manifest itself grew or lost reference state: %#v", long.Omissions)
	}
}

func TestContextEnvelopeUsesAllOmissionStates(t *testing.T) {
	session, _ := NewSession("s1", "p1", "goal", EngineV1, CapabilityInvocation{CapabilityID: "c1", CapabilityVer: "v1"})
	envelope := BuildContextEnvelope(ContextEnvelopeRequest{
		System: "system", Session: session,
		Bundle: ContextBundle{ID: "b1", ArtifactRefs: []string{"pack:b1"}, Omissions: map[string]OmissionStatus{
			"not_requested": OmissionNotRequested,
			"budget":        OmissionBudget,
			"reference":     OmissionByReference,
			"unavailable":   OmissionUnavailable,
			"stale":         OmissionStale,
			"forbidden":     OmissionForbidden,
		}},
	})
	for key, want := range map[string]OmissionStatus{
		"bundle.not_requested": OmissionNotRequested,
		"bundle.budget":        OmissionBudget, "bundle.reference": OmissionByReference,
		"bundle.unavailable": OmissionUnavailable, "bundle.stale": OmissionStale,
		"bundle.forbidden": OmissionForbidden,
	} {
		if envelope.Omissions[key] != want {
			t.Fatalf("omission %s=%q want=%q all=%#v", key, envelope.Omissions[key], want, envelope.Omissions)
		}
	}
}
