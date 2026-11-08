package agentloop

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

// PLUGIN-CONTROL-EVAL-BR1-1 (card 317) thin-prompt seam, B4/C4/305/312
// ablation-hook test pattern: default unset keeps the ordinary-family system
// skeleton byte-for-byte; VIT_PLUGIN_CONTROL_EVAL_SEAM_BR1_PROMPT=1 (exact
// value only) appends the frozen B-R1 guidance segment as the skeleton's
// final block, after the S4 trim and the verified-entry trim. The segment
// text is frozen verbatim from the card face; this file pins the exact bytes
// (independent literal + SHA-256) so the injected guidance cannot drift.

// seamBR1FrozenSegment repeats the card-face text as an INDEPENDENT literal:
// if either copy is edited the equality test fails. Card 317, verbatim:
// 「你有 web.search/web.fetch 可查插件手册与参数文档；遇到不确定的参数编码
// （param_id/量纲/取值范围）先查再写，查过的不重复查」
const seamBR1FrozenSegment = "你有 web.search/web.fetch 可查插件手册与参数文档；遇到不确定的参数编码（param_id/量纲/取值范围）先查再写，查过的不重复查"

// seamBR1FrozenSegmentSHA256 is the SHA-256 of the UTF-8 frozen segment,
// recorded in the card-317 RUNBOOK (byte-freeze anchor; computed once from
// the card text and pinned here).
const seamBR1FrozenSegmentSHA256 = "51c403d8f8901d586ae1180cdc699ff308dbbc5a870e3fe3db7c2cf68c5a202c"

const seamBR1LastSkeletonRule = "- User-facing reply text must be Chinese when the user writes Chinese. Do not expose JSON, tool names, tool IDs, or internal IDs unless the user explicitly asks for technical details."

func TestSeamBR1PromptSegmentBytesFrozen(t *testing.T) {
	if pluginControlEvalSeamBR1PromptSegment != seamBR1FrozenSegment {
		t.Fatalf("segment constant drifted from the frozen card text:\nconst: %q\ncard:  %q",
			pluginControlEvalSeamBR1PromptSegment, seamBR1FrozenSegment)
	}
	sum := sha256.Sum256([]byte(seamBR1FrozenSegment))
	if got := hex.EncodeToString(sum[:]); got != seamBR1FrozenSegmentSHA256 {
		t.Fatalf("frozen segment SHA-256 mismatch: got %s want %s", got, seamBR1FrozenSegmentSHA256)
	}
}

func TestSeamBR1PromptClosedKeepsSkeletonByteForByte(t *testing.T) {
	closed := messageLoopOrdinaryFamilySystemSkeleton(nil)
	if strings.Contains(closed, pluginControlEvalSeamBR1PromptSegment) {
		t.Fatal("closed seam must NOT contain the B-R1 segment")
	}
	if !strings.HasSuffix(closed, seamBR1LastSkeletonRule) {
		t.Fatal("closed seam skeleton must end with the last ordinary rule (byte-identical tail anchor)")
	}
}

func TestSeamBR1PromptOpenAppendsFrozenSegmentAsFinalBlock(t *testing.T) {
	t.Setenv(pluginControlEvalSeamBR1PromptEnv, "1")
	skeleton := messageLoopOrdinaryFamilySystemSkeleton(nil)
	if n := strings.Count(skeleton, pluginControlEvalSeamBR1PromptSegment); n != 1 {
		t.Fatalf("open seam must contain the segment exactly once, got %d", n)
	}
	wantTail := seamBR1LastSkeletonRule + "\n" + pluginControlEvalSeamBR1PromptSegment
	if !strings.HasSuffix(skeleton, wantTail) {
		t.Fatal("open seam must append the segment as the final block directly after the last rule")
	}
}

func TestSeamBR1PromptOnlyExactOneArmsTheSeam(t *testing.T) {
	closed := messageLoopOrdinaryFamilySystemSkeleton(nil)
	for _, value := range []string{"", "0", "true", "off", "1x"} {
		t.Setenv(pluginControlEvalSeamBR1PromptEnv, value)
		if pluginControlEvalSeamBR1PromptOpen() {
			t.Fatalf("env value %q must not open the B-R1 seam", value)
		}
		if got := messageLoopOrdinaryFamilySystemSkeleton(nil); got != closed {
			t.Fatalf("env value %q must keep the skeleton byte-for-byte", value)
		}
	}
}

// Composition with the S4 master seam (the mode B-R1 actually runs in):
// S4 strip + segment append must coexist, the segment landing after the trim
// so the injected guidance is always the final block.
func TestSeamBR1PromptOpenWithMasterSeamComposition(t *testing.T) {
	t.Setenv(pluginControlEvalSeamEnv, "1")
	t.Setenv(pluginControlEvalSeamBR1PromptEnv, "1")
	skeleton := messageLoopOrdinaryFamilySystemSkeleton(seamS4VerifiedEntryState())
	for _, marker := range seamS4TypedRuleMarkers {
		if strings.Contains(skeleton, marker) {
			t.Fatalf("composed seams must still strip typed rule %q", marker)
		}
	}
	wantTail := seamBR1LastSkeletonRule + "\n" + pluginControlEvalSeamBR1PromptSegment
	if !strings.HasSuffix(skeleton, wantTail) {
		t.Fatal("composed seams must append the B-R1 segment as the final block")
	}
}

// Verified-entry trim + BR1 (master seam closed): the segment lands after the
// verified-entry-trimmed skeleton too -- position is trim-order independent.
func TestSeamBR1PromptOpenWithVerifiedEntryState(t *testing.T) {
	t.Setenv(pluginControlEvalSeamBR1PromptEnv, "1")
	skeleton := messageLoopOrdinaryFamilySystemSkeleton(seamS4VerifiedEntryState())
	if !strings.HasSuffix(skeleton, pluginControlEvalSeamBR1PromptSegment) {
		t.Fatal("BR1 seam with verified-entry state must still end with the segment")
	}
}
