package capabilitycontext

import (
	"testing"
	"time"

	"vit-daw-agent/internal/mixstyle"
)

func TestOrchestrationBundleKeepsFullB2RowsOutOfDisclosure(t *testing.T) {
	pack := BuildStaticBalancePack(StaticBalanceInput{
		UserIntent: "inspect static balance",
		ProjectState: map[string]any{"tracks": []any{
			map[string]any{"id": "t1", "name": "Vocal", "volume_db": -2.0},
			map[string]any{"id": "t2", "name": "Bass", "volume_db": -4.0},
		}},
		Style:       mixstyle.Default(),
		GeneratedAt: time.Unix(1, 0),
		Budget:      Budget{MaxDisclosedTracks: 1, MaxRankingRows: 2},
	})
	bundle := OrchestrationBundle(pack, "cut-1")
	if bundle.ProjectCutHash != "cut-1" || len(bundle.ArtifactRefs) != 1 {
		t.Fatalf("unexpected bundle metadata: %#v", bundle)
	}
	if bundle.Disclosure == "" {
		t.Fatal("expected compact decision disclosure")
	}
	if len(bundle.OmissionReasons) == 0 {
		t.Fatal("expected budget omission to be explicit")
	}
	if contains(bundle.Disclosure, "Vocal") || contains(bundle.Disclosure, "Bass") {
		t.Fatal("full track rows must remain out of the default disclosure")
	}
}

func contains(value, needle string) bool {
	for i := 0; i+len(needle) <= len(value); i++ {
		if value[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
