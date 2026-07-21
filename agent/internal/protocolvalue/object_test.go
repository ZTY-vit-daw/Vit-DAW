package protocolvalue

import (
	"testing"

	"vit-daw-agent/internal/mom"
)

func TestObjectNormalizesTypedMOMProjection(t *testing.T) {
	projection := &mom.Projection{
		MOMVersion: mom.Version,
		Intent:     mom.IntentProjectMultitrackObservation,
		MultitrackRelation: mom.MultitrackRelation{
			Status: mom.StatusReady, TrackCount: 3,
		},
	}
	object := Object(projection)
	relation, ok := object["multitrack_relation"].(map[string]any)
	if !ok || relation["status"] != mom.StatusReady || relation["track_count"] != float64(3) {
		t.Fatalf("typed projection was not normalized: %#v", object)
	}
}

func TestObjectRejectsNonObjectPayload(t *testing.T) {
	if Object(nil) != nil || Object("not-an-object") != nil {
		t.Fatal("non-object payload must not be fabricated as an object")
	}
}
