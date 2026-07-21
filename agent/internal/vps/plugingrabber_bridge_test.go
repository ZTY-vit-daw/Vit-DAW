package vps

import (
	"strings"
	"testing"

	plugingrabber "vit-daw-agent/internal/workflows/plugingrabber"
)

func TestBuildPluginFingerprintFromDigestUsesCurrentSurfaceFacts(t *testing.T) {
	minimumGain, maximumGain := -18.0, 18.0
	minimumType, maximumType := 0.0, 2.0
	digest := plugingrabber.ParameterDigest{Parameters: []plugingrabber.ParameterInfo{
		{
			ID:  "gain",
			Min: minimumGain,
			Max: maximumGain,
			DisplayDomainCandidate: &plugingrabber.PluginDisplayDomain{
				Text: "-18..18 dB", Unit: "dB", Scale: "linear",
			},
		},
		{
			ID:         "type",
			IsDiscrete: true,
			Min:        minimumType,
			Max:        maximumType,
			DisplayProbe: &plugingrabber.ParameterDisplayProbe{DiscreteLabels: []plugingrabber.ParameterDisplayProbeLabel{
				{Index: 0, Label: "Low shelf"}, {Index: 1, Label: "Bell"}, {Index: 2, Label: "High shelf"},
			}},
			DisplayDomainCandidate: &plugingrabber.PluginDisplayDomain{Text: "filter type", Scale: "enum"},
		},
	}}
	fingerprint, err := BuildPluginFingerprintFromDigest("binary:learned-eq-1", digest)
	if err != nil {
		t.Fatalf("BuildPluginFingerprintFromDigest: %v", err)
	}
	if !fingerprint.Complete() || !strings.HasPrefix(fingerprint.ParameterSurface, "sha256:") || !strings.HasPrefix(fingerprint.DisplaySurface, "sha256:") {
		t.Fatalf("unexpected fingerprint: %#v", fingerprint)
	}
}

func TestBuildPluginFingerprintFromDigestFailsClosedWithoutDisplayDomain(t *testing.T) {
	digest := plugingrabber.ParameterDigest{Parameters: []plugingrabber.ParameterInfo{{ID: "gain", Min: -1.0, Max: 1.0}}}
	if _, err := BuildPluginFingerprintFromDigest("binary:example", digest); err == nil || !strings.Contains(err.Error(), "display-domain") {
		t.Fatalf("missing display-domain error = %v", err)
	}
}

func TestBuildPluginFingerprintFromDigestUsesRawHostDisplayForOpaqueControl(t *testing.T) {
	digest := plugingrabber.ParameterDigest{Parameters: []plugingrabber.ParameterInfo{{
		ID:  "ui_state",
		Min: 0.0,
		Max: 1.0,
		DisplayProbe: &plugingrabber.ParameterDisplayProbe{
			CurrentText: "On",
			Samples: []plugingrabber.ParameterDisplayProbeSample{
				{NormalizedValue: 0, Text: "Off"},
				{NormalizedValue: 1, Text: "On"},
			},
		},
	}}}
	fingerprint, err := BuildPluginFingerprintFromDigest("binary:host-surface", digest)
	if err != nil {
		t.Fatalf("BuildPluginFingerprintFromDigest: %v", err)
	}
	if !fingerprint.Complete() {
		t.Fatalf("fingerprint = %#v", fingerprint)
	}
}

func TestBuildPluginFingerprintFromDigestIgnoresMutableCurrentHostText(t *testing.T) {
	build := func(current string) plugingrabber.ParameterDigest {
		return plugingrabber.ParameterDigest{Parameters: []plugingrabber.ParameterInfo{{
			ID:  "filter_type",
			Min: 0.0,
			Max: 1.0,
			DisplayProbe: &plugingrabber.ParameterDisplayProbe{
				CurrentText: current,
				Samples: []plugingrabber.ParameterDisplayProbeSample{
					{NormalizedValue: 0, Text: "Low S"},
					{NormalizedValue: .25, Text: "Bell"},
					{NormalizedValue: .5, Text: "Bell"},
					{NormalizedValue: .75, Text: "High S"},
					{NormalizedValue: 1, Text: "High S"},
				},
			},
		}}}
	}
	learned, err := BuildPluginFingerprintFromDigest("binary:host-surface", build("Bell"))
	if err != nil {
		t.Fatalf("learned fingerprint: %v", err)
	}
	current, err := BuildPluginFingerprintFromDigest("binary:host-surface", build("Low S"))
	if err != nil {
		t.Fatalf("current fingerprint: %v", err)
	}
	if !learned.Equal(current) {
		t.Fatalf("mutable current display text changed the VPS fingerprint: learned=%#v current=%#v", learned, current)
	}
}

func TestBuildPluginFingerprintFromDigestFailsClosedWithOnlyMutableCurrentHostText(t *testing.T) {
	digest := plugingrabber.ParameterDigest{Parameters: []plugingrabber.ParameterInfo{{
		ID:  "filter_type",
		Min: 0.0,
		Max: 1.0,
		DisplayProbe: &plugingrabber.ParameterDisplayProbe{
			CurrentText: "Bell",
		},
	}}}
	if _, err := BuildPluginFingerprintFromDigest("binary:host-surface", digest); err == nil || !strings.Contains(err.Error(), "display-domain") {
		t.Fatalf("current value alone must not qualify a stable display surface, got %v", err)
	}
}
