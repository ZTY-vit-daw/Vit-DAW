package main

import "testing"

func TestSignalProbeScopeFreezesExactlyOneMeasurementRange(t *testing.T) {
	clipScope, err := signalProbeScope("clip-1", "", "", "0.25")
	if err != nil || clipScope.ClipID != "clip-1" || clipScope.TapPoint != "track_post_fader" {
		t.Fatalf("clip scope=%#v err=%v", clipScope, err)
	}
	rangeScope, err := signalProbeScope("", "0", "12", "")
	if err != nil || rangeScope.StartSeconds == nil || rangeScope.EndSeconds == nil || *rangeScope.StartSeconds != 0 || *rangeScope.EndSeconds != 12 {
		t.Fatalf("range scope=%#v err=%v", rangeScope, err)
	}
	if _, err := signalProbeScope("clip-1", "0", "12", ""); err == nil {
		t.Fatal("clip and explicit range were accepted together")
	}
	if _, err := signalProbeScope("", "0", "", ""); err == nil {
		t.Fatal("incomplete range was accepted")
	}
}
