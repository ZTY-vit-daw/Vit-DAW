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
	// Official instrumental recipe style normalization (YUE-INSTRUMENTAL-PATH-1):
	// prefix + the four no-voice conditions + trailing period. The bare
	// "instrumental" tail word of the pre-390 path showed no suppression
	// effect (run_20261116_000009) and is replaced by the recipe form.
	want := "Instrumental, gentle strings, no vocals, no singing, no choir, no spoken words."
	if res.Request.StyleTags != want {
		t.Fatalf("instrumental style not normalized to the official recipe:\n got %q\nwant %q", res.Request.StyleTags, want)
	}
	// Vocal voice rested, Ins untouched.
	if !strings.Contains(res.Request.ABC, "z32|") || strings.Contains(res.Request.ABC, "c8d8e8f8|\nV: Ins\nz32|") {
		t.Fatalf("vocal not rested properly:\n%s", res.Request.ABC)
	}
	vocal, perr := ParseYueABC(res.Request.ABC, "Vocal")
	if perr != nil || len(vocal) != 0 {
		t.Fatalf("vocal must parse silent after conditioning: %v %#v", perr, vocal)
	}
	// No % sections in this score -> lyrics stay empty (official equivalent).
	if res.Request.Lyrics != "" {
		t.Fatalf("sectionless score must not grow lyrics: %q", res.Request.Lyrics)
	}
	// No chords on the resting Vocal -> cot=melody (official derivation).
	if res.Request.COT != "melody" {
		t.Fatalf("cot must derive melody for a chordless Vocal voice, got %q", res.Request.COT)
	}
	// YUE-VOCAL-FORENSICS-1: the disclosure must not oversell request-side
	// conditioning — the engine has no instrumental switch and may still
	// sing. Guard the wording so a future edit cannot reintroduce
	// "validated instrumental" claims (372 receipt oversell root cause).
	joined := strings.Join(res.Actions, "\n")
	if !strings.Contains(joined, "engine-side muting not guaranteed") {
		t.Fatalf("instrumental actions must disclose that engine-side muting is not guaranteed: %#v", res.Actions)
	}
	if strings.Contains(joined, "validated instrumental") {
		t.Fatalf("actions must not claim a validated instrumental path: %#v", res.Actions)
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

// YUE-INSTRUMENTAL-PATH-1 S1: official recipe style normalization edges.
func TestYueInstrumentalRecipeStyleNormalization(t *testing.T) {
	cases := []struct {
		name  string
		in    string
		final string
	}{
		{
			name: "empty style takes the official default (which still gains the prefix)",
			in:   "",
			// Official prepare: default text "Expressive instrumental music"
			// does not open with the word instrumental, so the prefix is
			// added on top — faithful to instrumental.py.
			final: "Instrumental, Expressive instrumental music, no vocals, no singing, no choir, no spoken words.",
		},
		{
			name:  "plain style gains prefix, conditions and period",
			in:    "warm solo piano",
			final: "Instrumental, warm solo piano, no vocals, no singing, no choir, no spoken words.",
		},
		{
			name:  "style already opening with Instrumental keeps its prefix",
			in:    "Instrumental, chamber strings",
			final: "Instrumental, chamber strings, no vocals, no singing, no choir, no spoken words.",
		},
		{
			name:  "lowercase instrumental opener is honored case-insensitively",
			in:    "instrumental jazz trio",
			final: "instrumental jazz trio, no vocals, no singing, no choir, no spoken words.",
		},
		{
			name:  "existing conditions are not duplicated",
			in:    "warm solo piano, no vocals",
			final: "Instrumental, warm solo piano, no vocals, no singing, no choir, no spoken words.",
		},
		{
			name:  "trailing punctuation is stripped before prefixing",
			in:    "gentle harp.,",
			final: "Instrumental, gentle harp, no vocals, no singing, no choir, no spoken words.",
		},
	}
	for _, tc := range cases {
		res, err := YuePrepareRequest(GenerationRequest{
			Instrumental: true,
			StyleTags:    tc.in,
			ABC:          "X:1\nK:C\nV: Ins\nc4|",
		}, YuePrepareOptions{})
		if err != nil {
			t.Fatalf("%s: prepare: %v", tc.name, err)
		}
		if res.Request.StyleTags != tc.final {
			t.Fatalf("%s:\n got %q\nwant %q", tc.name, res.Request.StyleTags, tc.final)
		}
	}
}

// YUE-INSTRUMENTAL-PATH-1 S2/S4: section-tag lyrics and cot derivation.
func TestYueInstrumentalRecipeLyricsAndCot(t *testing.T) {
	// Sections + chords on the resting Vocal voice (the official score form):
	// lyrics = exact section tags, cot = full.
	officialForm := "X:1\nM:4/4\nL:1/32\nQ:1/4=96\nV: Vocal clef=treble name=\"Vocal Melody\" snm=\"Vocal\"\n" +
		"V: Ins clef=treble name=\"Ins Melody\" snm=\"Inst.\"\nK:G\n% verse\nV: Vocal\n\"G\"z32|\"D7\"z32|\n" +
		"V: Ins\nd8e6d2B8z8|F8F8=F8z8|\n% outro\nV: Vocal\n\"Em\"z16z4|\nV: Ins\nE8G8B4-|B4G8D8|\n"
	res, err := YuePrepareRequest(GenerationRequest{
		ABC:          officialForm,
		Instrumental: true,
		StyleTags:    "chamber strings",
	}, YuePrepareOptions{})
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	wantTags := "[Verse]\n\n[Outro]\n"
	if res.Request.Lyrics != wantTags {
		t.Fatalf("section tags mismatch:\n got %q\nwant %q", res.Request.Lyrics, wantTags)
	}
	if res.Request.COT != "full" {
		t.Fatalf("chord-annotated Vocal voice must derive cot=full, got %q", res.Request.COT)
	}
	joined := strings.Join(res.Actions, "\n")
	if !strings.Contains(joined, "lyrics: section tags generated") || !strings.Contains(joined, "cot: derived from the score's Vocal chords (full") {
		t.Fatalf("lyrics/cot actions missing: %#v", res.Actions)
	}
	// Explicit lyrics are kept (never silently replaced) with a disclosure.
	res2, err := YuePrepareRequest(GenerationRequest{
		ABC:          officialForm,
		Instrumental: true,
		StyleTags:    "s",
		Lyrics:       "student wrote these words",
	}, YuePrepareOptions{})
	if err != nil {
		t.Fatalf("prepare2: %v", err)
	}
	if res2.Request.Lyrics != "student wrote these words" {
		t.Fatalf("explicit lyrics must be kept, got %q", res2.Request.Lyrics)
	}
	if !strings.Contains(strings.Join(res2.Actions, "\n"), "lyrics: request lyrics kept as provided") {
		t.Fatalf("kept-lyrics disclosure missing: %#v", res2.Actions)
	}
	// Our converter's form (chords ride the Ins melody) derives melody.
	converted, cerr := ConvertMidiToYueABC(YueScoreInput{
		Notes:     []YueNote{{Pitch: 60, Start: 0, Length: 4}},
		Sections:  []YueSection{{Name: "intro", Bars: 1}},
		Chords:    []YueChord{{Bar: 0, Name: "C"}},
		TempoBPM:  120,
	})
	if cerr != nil {
		t.Fatalf("convert: %v", cerr)
	}
	res3, err := YuePrepareRequest(GenerationRequest{ABC: converted, Instrumental: true}, YuePrepareOptions{})
	if err != nil {
		t.Fatalf("prepare3: %v", err)
	}
	if res3.Request.COT != "melody" {
		t.Fatalf("converter form (chords on Ins) must derive cot=melody, got %q", res3.Request.COT)
	}
	if res3.Request.Lyrics != "[Intro]\n" {
		t.Fatalf("converted score sections must become tags, got %q", res3.Request.Lyrics)
	}
}

// YUE-INSTRUMENTAL-PATH-1 zero-generation dry-run leg: the dispatched body of
// a fully conditioned instrumental request must (a) carry the official
// recipe fields (style/lyrics/abc/cot) and (b) stay inside the field set the
// local engine has actually accepted in prior runs — no invented parameters.
// Evidence anchors: YUE2-PC-DEPLOY-1 smoke1 (style/lyrics/cot/duration/seed/
// steps/output_format accepted), run_20261116_000009 engine echo (style/abc/
// duration/lm_seed/seed/output_format accepted) + "[Prompt] cot=full"
// consumption line, dialect replay knobs (synth_batch_size/semantic_tokens).
// This proves the construction face only; engine-side acceptance of
// cot=melody and the abc+lyrics combination remains a real-stack question.
func TestYueInstrumentalDispatchShapeDryRun(t *testing.T) {
	res, err := YuePrepareRequest(GenerationRequest{
		Instrumental:    true,
		StyleTags:       "warm solo piano",
		DurationSeconds: 60,
		ABC: "X:1\nM:4/4\nL:1/32\nQ:1/4=96\nV: Vocal clef=treble name=\"Vocal Melody\" snm=\"Vocal\"\n" +
			"V: Ins clef=treble name=\"Ins Melody\" snm=\"Inst.\"\nK:C\n% intro\nV: Vocal\nZ4|\nV: Ins\nc8e8g8c'8|b8g8e8g8|a8f8d8f8|e8c8d8e8|\n",
	}, YuePrepareOptions{})
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	d := dialectRegistry["yue_local"]
	cfg := ProviderConfig{ID: "yue-local", Dialect: "yue_local", BaseURL: "http://127.0.0.1:8087", ExtraParams: map[string]any{"seed": 11}}
	body := d.buildRequest(d, res.Request, cfg)

	// The recipe face is fully present.
	if body["style"] != "Instrumental, warm solo piano, no vocals, no singing, no choir, no spoken words." {
		t.Fatalf("dispatched style not recipe-normalized: %q", body["style"])
	}
	if body["lyrics"] != "[Intro]\n" {
		t.Fatalf("dispatched lyrics must be the exact section tags, got %q", body["lyrics"])
	}
	if body["cot"] != "melody" {
		t.Fatalf("dispatched cot must be the derived melody, got %v", body["cot"])
	}
	abc, _ := body["abc"].(string)
	vocal, perr := ParseYueABC(abc, "Vocal")
	if perr != nil || len(vocal) != 0 {
		t.Fatalf("dispatched abc must keep the Vocal voice silent: %v %#v", perr, vocal)
	}
	if body["duration"] != 60.0 || body["seed"] != 11 || body["steps"] != 32 || body["output_format"] != "wav16" {
		t.Fatalf("baseline knobs not intact: %#v", body)
	}

	// No invented parameters: every key is inside the engine-evidenced set.
	evidenced := map[string]bool{
		"style": true, "lyrics": true, "abc": true, "cot": true, "duration": true,
		"seed": true, "steps": true, "output_format": true, "lm_seed": true,
		"synth_batch_size": true, "semantic_tokens": true,
	}
	for k := range body {
		if !evidenced[k] {
			t.Fatalf("dispatched key %q is outside the engine-evidenced accepted set: %#v", k, body)
		}
	}

	// The dispatched summary mirrors the effective cot.
	sum := YueDispatchedSummary(res.Request, cfg)
	if sum["cot"] != "melody" {
		t.Fatalf("summary must show the derived cot, got %#v", sum["cot"])
	}
	if sum["lyrics_chars"] != len("[Intro]\n") {
		t.Fatalf("summary must reflect the section-tag lyrics: %#v", sum)
	}
}

// Non-instrumental yue_local requests keep the validated cot=full baseline
// (no derivation happens outside the instrumental branch).
func TestYueNonInstrumentalKeepsFullCot(t *testing.T) {
	res, err := YuePrepareRequest(GenerationRequest{
		ABC:       "X:1\nK:C\nV: Ins\nc4|",
		StyleTags: "solo piano",
	}, YuePrepareOptions{})
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if res.Request.COT != "" {
		t.Fatalf("non-instrumental requests must not grow a cot value, got %q", res.Request.COT)
	}
	d := dialectRegistry["yue_local"]
	body := d.buildRequest(d, res.Request, ProviderConfig{ID: "yue-local", Dialect: "yue_local", BaseURL: "http://127.0.0.1:8087"})
	if body["cot"] != "full" {
		t.Fatalf("builder baseline cot must stay full, got %v", body["cot"])
	}
}
