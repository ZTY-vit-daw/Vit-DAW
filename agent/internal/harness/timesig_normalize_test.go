package harness

import (
	"testing"

	"vit-daw-agent/internal/tools"
)

// KERNEL-TIMESIG-1: arg normalization for the project meter command. Models
// commonly emit the W01 wire form ("2/4") as a single field — the split fills
// the kernel's integer pair. Everything not integer-parseable stays absent so
// the kernel's fail-closed validation rejects it (no coercion, no guessing).
func TestNormalizeCommandArgsSetTimeSignature(t *testing.T) {
	spec := tools.CommandSpec{CommandName: "set_time_signature"}

	t.Run("wire form split into integer pair", func(t *testing.T) {
		cmd := map[string]any{"time_signature": "2/4"}
		normalizeCommandArgs(spec, cmd, nil)
		if got := cmd["numerator"]; got != 2 {
			t.Fatalf("numerator = %v (%T), want 2", got, got)
		}
		if got := cmd["denominator"]; got != 4 {
			t.Fatalf("denominator = %v (%T), want 4", got, got)
		}
	})

	t.Run("explicit integer pair untouched", func(t *testing.T) {
		cmd := map[string]any{"numerator": 3, "denominator": 4}
		normalizeCommandArgs(spec, cmd, nil)
		if cmd["numerator"] != 3 || cmd["denominator"] != 4 {
			t.Fatalf("pair changed: %+v", cmd)
		}
	})

	t.Run("aliases copied", func(t *testing.T) {
		cmd := map[string]any{"beats_per_bar": 6, "note_value": 8}
		normalizeCommandArgs(spec, cmd, nil)
		if cmd["numerator"] != 6 || cmd["denominator"] != 8 {
			t.Fatalf("aliases not copied: %+v", cmd)
		}
	})

	t.Run("junk wire form leaves pair absent for kernel fail-closed", func(t *testing.T) {
		cmd := map[string]any{"time_signature": "maybe"}
		normalizeCommandArgs(spec, cmd, nil)
		if _, exists := cmd["numerator"]; exists {
			t.Fatalf("numerator fabricated from junk: %+v", cmd)
		}
		if _, exists := cmd["denominator"]; exists {
			t.Fatalf("denominator fabricated from junk: %+v", cmd)
		}
	})

	t.Run("double slash junk left absent", func(t *testing.T) {
		cmd := map[string]any{"time_signature": "2/4/8"}
		normalizeCommandArgs(spec, cmd, nil)
		if _, exists := cmd["numerator"]; exists {
			t.Fatalf("numerator fabricated from 2/4/8: %+v", cmd)
		}
	})

	t.Run("fractional parts left absent", func(t *testing.T) {
		cmd := map[string]any{"time_signature": "2.5/4"}
		normalizeCommandArgs(spec, cmd, nil)
		if _, exists := cmd["numerator"]; exists {
			t.Fatalf("numerator fabricated from 2.5/4: %+v", cmd)
		}
	})
}
