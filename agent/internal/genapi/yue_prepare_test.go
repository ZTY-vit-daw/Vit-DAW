package genapi

import (
	"strings"
	"testing"
)

// GEN-YUE-WIRE-1 items 2-4: prompt->style mapping, instrumental conditioning,
// extra_params hygiene, dispatched summary.

func TestYuePrepareStyleFromPrompt(t *testing.T) {
	// User changes the prompt -> the style channel actually changes, and the
	// dispatched summary shows what really leaves the building.
	res, err := YuePrepareRequest(GenerationRequest{
		Prompt:       "rendered genctx blocks...",
		ABC:          "X:1\nK:C\nV: Ins\nc4|",
		DurationSeconds: 60,
	}, YuePrepareOptions{UserPrompt: "欢快的钢琴独奏，突出学生旋律"})
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if res.Request.StyleTags != "欢快的钢琴独奏，突出学生旋律" {
		t.Fatalf("style must map the raw user prompt verbatim, got %q", res.Request.StyleTags)
	}
	if res.Dispatched["style"] != res.Request.StyleTags {
		t.Fatalf("dispatched summary must carry the style in full: %#v", res.Dispatched)
	}
	found := false
	for _, a := range res.Actions {
		if strings.HasPrefix(a, "style: derived verbatim") {
			found = true
		}
	}
	if !found {
		t.Fatalf("prompt-derived style must be disclosed in actions: %#v", res.Actions)
	}
	// Explicit style_tags win over the prompt.
	res2, err := YuePrepareRequest(GenerationRequest{ABC: "X:1", StyleTags: "solo piano"}, YuePrepareOptions{UserPrompt: "ignored"})
	if err != nil {
		t.Fatalf("prepare2: %v", err)
	}
	if res2.Request.StyleTags != "solo piano" {
		t.Fatalf("explicit style must win, got %q", res2.Request.StyleTags)
	}
}

func TestYuePrepareInstrumentalConditioning(t *testing.T) {
	abc := "X:1\nM:4/4\nL:1/32\nK:C\nV: Vocal\nV: Ins\nV: Vocal\nc8d8e8f8|\nV: Ins\nz32|\n"
	res, err := YuePrepareRequest(GenerationRequest{
		ABC:          abc,
		Instrumental: true,
		StyleTags:    "gentle strings",
	}, YuePrepareOptions{})
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if res.Request.StyleTags != "gentle strings, instrumental" {
		t.Fatalf("instrumental tag missing: %q", res.Request.StyleTags)
	}
	// Vocal voice rested, Ins untouched.
	if !strings.Contains(res.Request.ABC, "z32|") || strings.Contains(res.Request.ABC, "c8d8e8f8|\nV: Ins\nz32|") {
		t.Fatalf("vocal not rested properly:\n%s", res.Request.ABC)
	}
	vocal, perr := ParseYueABC(res.Request.ABC, "Vocal")
	if perr != nil || len(vocal) != 0 {
		t.Fatalf("vocal must parse silent after conditioning: %v %#v", perr, vocal)
	}
	// Count>1 collapses to one job per version with disclosure.
	res2, err := YuePrepareRequest(GenerationRequest{ABC: abc, Count: 2, StyleTags: "x"}, YuePrepareOptions{})
	if err != nil {
		t.Fatalf("prepare2: %v", err)
	}
	if res2.Request.Count != 1 {
		t.Fatalf("count must collapse to 1, got %d", res2.Request.Count)
	}
}

func TestYuePrepareScoreConversionWired(t *testing.T) {
	res, err := YuePrepareRequest(GenerationRequest{Prompt: "x"}, YuePrepareOptions{
		UserPrompt: "bright marimba",
		Score: &YueScoreInput{
			Notes:       []YueNote{{Pitch: 60, Start: 0, Length: 4}},
			Numerator:   4,
			Denominator: 4,
		},
	})
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if !strings.Contains(res.Request.ABC, "c32|") {
		t.Fatalf("score not converted:\n%s", res.Request.ABC)
	}
	if res.Request.StyleTags != "bright marimba" {
		t.Fatalf("style mapping lost: %q", res.Request.StyleTags)
	}
}

func TestYueBuilderExtraParamsHygiene(t *testing.T) {
	d := dialectRegistry["yue_local"]
	cfg := ProviderConfig{
		ID:          "yue-local",
		Dialect:     "yue_local",
		BaseURL:     "http://127.0.0.1:8087",
		ExtraParams: map[string]any{"abc": "HACKED", "style": "fixed-style", "lyrics": "fixed-lyrics", "seed": 11, "steps": 24},
	}
	req := GenerationRequest{ABC: "X:1\nK:C\nV: Ins\nc4|", StyleTags: "student style", Lyrics: "student lyrics"}
	body := d.buildRequest(d, req, cfg)
	// Student fields win over the fixed overlay.
	if body["abc"] != req.ABC || body["style"] != "student style" || body["lyrics"] != "student lyrics" {
		t.Fatalf("student fields must not be overridden: %#v", body)
	}
	// Engine knobs remain overridable.
	if body["seed"] != 11 || body["steps"] != 24 {
		t.Fatalf("engine knobs must remain overridable: %#v", body)
	}
	// Limitations disclose the dropped protected keys.
	lims := cfg.Limitations(req)
	sawABC := false
	for _, l := range lims {
		if strings.HasPrefix(l, "extra_params.abc:") {
			sawABC = true
		}
	}
	if !sawABC {
		t.Fatalf("protected extra_params must be disclosed: %#v", lims)
	}
	// The dispatched summary lists them too.
	sum := YueDispatchedSummary(req, cfg)
	dropped, _ := sum["extra_params_dropped_protected"].([]string)
	if len(dropped) != 3 {
		t.Fatalf("summary must list the three protected keys: %#v", sum["extra_params_dropped_protected"])
	}
}
