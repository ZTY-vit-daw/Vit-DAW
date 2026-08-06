package plugingrabber

import (
	"reflect"
	"testing"
)

func TestCompressorAxesForRolesUsesSemanticContract(t *testing.T) {
	axes := CompressorAxesForRoles([]string{"threshold", "ratio", "mix", "threshold"})
	want := []string{"activation_intensity", "transfer_severity", "parallel_balance"}
	if !reflect.DeepEqual(axes, want) {
		t.Fatalf("axes=%v want=%v", axes, want)
	}
}
