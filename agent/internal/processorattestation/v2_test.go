package processorattestation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestV2CoverageVocabularyIsIdentityFreeAndFamilyBound(t *testing.T) {
	for _, family := range []string{FamilyLimiter, FamilyGateExpander, FamilyDeEsser, FamilyTransient, FamilyMultiband} {
		axes := V2CoverageAxes(family)
		if len(axes) == 0 {
			t.Fatalf("family %s has no axes", family)
		}
		for _, axis := range axes {
			if err := ValidateV2Coverage(family, Coverage{Action: "adjust", Axis: axis}); err != nil {
				t.Fatalf("family=%s axis=%s err=%v", family, axis, err)
			}
		}
		if err := ValidateV2Coverage(family, Coverage{Action: "upsert", Axis: axes[0]}); err == nil {
			t.Fatalf("family %s accepted non-adjust action", family)
		}
	}
	roles := V2CoverageForRoles(FamilyDeEsser, []string{"threshold", "reduction_range", "focus_frequency", "release"})
	if len(roles) != 4 || roles[0].Action != "adjust" {
		t.Fatalf("roles=%+v", roles)
	}
	encoded, _ := json.Marshal(roles)
	for _, forbidden := range []string{"param_id", "topology_generation", "profile", "vps", "spal"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("coverage leaked %s: %s", forbidden, encoded)
		}
	}
}

func TestV2StoreQueryRequiresCurrentFingerprintAndCoverage(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "attestations.v2.json")
	store, err := NewStoreV2(path)
	if err != nil {
		t.Fatal(err)
	}
	pluginPath := filepath.Join(root, "Limiter.vst3")
	if err := os.WriteFile(pluginPath, []byte("limiter-v1"), 0600); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := FingerprintPath(pluginPath)
	if err != nil {
		t.Fatal(err)
	}
	spec := IssueSpecV2{Subject: Subject{Name: "Limiter", Manufacturer: "Vendor", Format: "VST3", Identifier: "limiter-v1", InstalledPath: pluginPath}, BinaryFingerprint: fingerprint, ProcessorFamily: FamilyLimiter, Coverage: []Coverage{{Action: "adjust", Axis: "output_ceiling"}}, Evidence: []EvidenceRef{{ReceiptID: "receipt-v2", Kind: "processor.v2", SHA256: "sha256:" + strings.Repeat("a", 64), ObservedAt: time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC)}}}
	issued, err := store.PromoteCurrent(spec, "test")
	if err != nil {
		t.Fatal(err)
	}
	if issued.Status != StatusPromoted || !strings.HasPrefix(issued.AttestationID, "pca2_") {
		t.Fatalf("issued=%+v", issued)
	}
	key := issued.Subject.SubjectKey
	result, err := store.Query(QueryV2{SubjectKey: key, BinaryFingerprint: fingerprint, ProcessorFamily: FamilyLimiter, RequiredCoverage: []Coverage{{Action: "adjust", Axis: "output_ceiling"}}})
	if err != nil || !result.Eligible {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if _, err := store.Query(QueryV2{SubjectKey: key, BinaryFingerprint: fingerprint, ProcessorFamily: FamilyLimiter, RequiredCoverage: []Coverage{{Action: "adjust", Axis: "peak_mode"}}}); err != nil {
		t.Fatal(err)
	} else if result, _ := store.Query(QueryV2{SubjectKey: key, BinaryFingerprint: fingerprint, ProcessorFamily: FamilyLimiter, RequiredCoverage: []Coverage{{Action: "adjust", Axis: "peak_mode"}}}); result.Eligible {
		t.Fatal("missing axis was eligible")
	}
}
