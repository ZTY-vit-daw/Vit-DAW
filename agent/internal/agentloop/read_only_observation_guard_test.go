package agentloop

import "testing"

func TestMessageLoopReadOnlyObservationRequestCatchesLookAtOverallMix(t *testing.T) {
	if !messageLoopReadOnlyObservationRequest("\u5e2e\u6211\u770b\u6574\u4f53\u6df7\u97f3") {
		t.Fatalf("look-at overall mix request was not treated as read-only observation")
	}
}

func TestMessageLoopReadOnlyObservationRequestCatchesExactChineseObservation(t *testing.T) {
	if !messageLoopReadOnlyObservationRequest("观察当前工程的频段和声像状态，不要修改。") {
		t.Fatalf("exact Chinese observation request was not treated as read-only observation")
	}
}

func TestMessageLoopReadOnlyObservationRequestKeepsExplicitActionOpen(t *testing.T) {
	if messageLoopReadOnlyObservationRequest("\u5e2e\u6211\u770b\u6574\u4f53\u6df7\u97f3\uff0c\u7136\u540e\u8c03\u6574 Track 2") {
		t.Fatalf("explicit adjustment request was incorrectly treated as read-only observation")
	}
}

func TestMessageLoopReadOnlyObservationRequestKeepsExactChinesePreflightOpen(t *testing.T) {
	if messageLoopReadOnlyObservationRequest("帮我把低频稍微收一点，但先告诉我依据。") {
		t.Fatalf("exact Chinese action preflight was incorrectly treated as read-only observation")
	}
}
