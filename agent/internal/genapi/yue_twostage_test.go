package genapi

import (
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// fakeWav is the minimal sniffable WAV payload the fake engine returns.
var fakeWav = append([]byte("RIFF\x24\x00\x00\x00WAVEfmt "), make([]byte, 32)...)

// twoStageFakeEngine is the dry-run stand-in for yue2.cpp (372 engine
// chain): POST /synth -> {"id"}, GET /job?id= -> {"status":"done"},
// GET /job?id=&result=1 -> multipart (JSON replay echo + audio). pass1 =
// no abc in the request body (free generation, echoes genABC); pass2 =
// abc present (echoes what it was sent, so the test can assert the
// transferred score arrived byte-exact).
type twoStageFakeEngine struct {
	mu     sync.Mutex
	bodies []map[string]any
	genABC string
}

func (e *twoStageFakeEngine) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/synth", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		e.mu.Lock()
		e.bodies = append(e.bodies, body)
		id := "job-pass1"
		if _, ok := body["abc"]; ok {
			id = "job-pass2"
		}
		e.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fmt.Sprintf(`{"id":%q}`, id)))
	})
	mux.HandleFunc("/job", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("result") != "1" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"done"}`))
			return
		}
		id := r.URL.Query().Get("id")
		e.mu.Lock()
		echo := e.genABC
		if id == "job-pass2" && len(e.bodies) > 1 {
			if a, ok := e.bodies[1]["abc"].(string); ok {
				echo = a
			}
		}
		e.mu.Unlock()
		w.Header().Set("Content-Type", "multipart/mixed; boundary=vitfake")
		mw := multipart.NewWriter(w)
		_ = mw.SetBoundary("vitfake")
		replayPart, _ := mw.CreatePart(map[string][]string{"Content-Type": {"application/json"}})
		replayJSON, _ := json.Marshal(map[string]any{"abc": echo, "semantic_tokens": "11,22,33"})
		_, _ = replayPart.Write(replayJSON)
		audioPart, _ := mw.CreatePart(map[string][]string{"Content-Type": {"audio/wav"}})
		_, _ = audioPart.Write(fakeWav)
		_ = mw.Close()
	})
	return mux
}

// TestYueTwoStageDryRunFakeEngine covers both stages and the receipt on a
// fake engine (zero real stack): pass1 free generation -> Go Vocal->Ins
// score transfer -> pass2 fixed-abc re-render, pass2 request body exactly
// the 214808 口径 (abc=transferred score, cot re-derived, style verbatim,
// fixed seeds from extra_params), final WAV from pass2, pass1 artifacts
// kept on disk as evidence.
func TestYueTwoStageDryRunFakeEngine(t *testing.T) {
	// Generated pass1 score: 5 vocal notes + 2 chord annotations; the Ins
	// voice overlaps the vocal at the verse (vocal g8 vs ins c8) so the
	// vocal-priority overlap policy is exercised on the dry run too.
	genABC := "X:1\nT:\nM:4/4\nL:1/32\nQ:1/4=96\n" +
		"V: Vocal clef=treble name=\"Vocal Melody\" snm=\"Vocal\"\n" +
		"V: Ins clef=treble name=\"Ins Melody\" snm=\"Inst.\"\n" +
		"K:C\n% intro\nV: Vocal\n\"C\"c8d8e8f8|\nV: Ins\nz32|\n% verse\nV: Vocal\n\"D\"g8z24|\nV: Ins\nc8d8e8f8|\n"
	wantTransferred, wantReceipt, terr := YueInstrumentalizeTransfer(genABC)
	if terr != nil {
		t.Fatalf("fixture transfer: %v", terr)
	}
	if wantReceipt.VocalNotesBefore != 5 || wantReceipt.VocalNotesAfter != 0 || wantReceipt.OriginalInsNotes != 4 || wantReceipt.OutputInsNotes != 8 {
		t.Fatalf("fixture receipt wrong: %#v", wantReceipt)
	}
	if len(wantReceipt.AffectedInsNotes) != 1 {
		t.Fatalf("fixture must exercise the overlap policy: %#v", wantReceipt.AffectedInsNotes)
	}

	eng := &twoStageFakeEngine{genABC: genABC}
	srv := httptest.NewServer(eng.handler())
	defer srv.Close()

	cfg := ProviderConfig{
		ID:          "yue-two-stage-fake",
		Dialect:     "yue_local",
		BaseURL:     srv.URL,
		DownloadDir: t.TempDir(),
		ExtraParams: map[string]any{"seed": 23, "lm_seed": 39020261120},
	}
	req := GenerationRequest{
		Prompt:          "rendered genctx blocks (not forwarded by yue_local; the score/style/lyrics are the input face)",
		Instrumental:    true,
		StyleTags:       "Instrumental, warm string quartet, no vocals, no singing, no choir, no spoken words.",
		DurationSeconds: 60,
	}
	res, err := YueGenerateInstrumentalTwoStage(context.Background(), cfg, req, YueTwoStageOptions{})
	if err != nil {
		t.Fatalf("two-stage: %v", err)
	}

	// Two jobs hit the engine; pass1 without abc, pass2 with the exact
	// transferred score.
	eng.mu.Lock()
	defer eng.mu.Unlock()
	if len(eng.bodies) != 2 {
		t.Fatalf("engine saw %d submits, want 2 (two jobs): %#v", len(eng.bodies), eng.bodies)
	}
	if _, has := eng.bodies[0]["abc"]; has {
		t.Fatalf("pass1 must be free generation (no abc): %#v", eng.bodies[0])
	}
	if eng.bodies[1]["abc"] != wantTransferred {
		t.Fatalf("pass2 abc is not the byte-exact transferred score:\n got %q\nwant %q", eng.bodies[1]["abc"], wantTransferred)
	}
	// 214808 口径: style verbatim (recipe-normalized), cot re-derived from
	// the transferred score's Vocal chords (chords ride Vocal -> full).
	if eng.bodies[1]["style"] != req.StyleTags {
		t.Fatalf("pass2 style must stay verbatim: %#v", eng.bodies[1]["style"])
	}
	if eng.bodies[1]["cot"] != "full" {
		t.Fatalf("pass2 cot must be re-derived full, got %v", eng.bodies[1]["cot"])
	}
	if eng.bodies[1]["duration"] != float64(60) || eng.bodies[1]["seed"] != float64(23) || eng.bodies[1]["lm_seed"] != float64(39020261120) {
		t.Fatalf("pass2 knobs/seeds wrong: %#v", eng.bodies[1])
	}

	// Receipt face: both jobs, structural counts, final version = pass2.
	if res.Pass1 == nil || res.Pass2 == nil || res.Pass1.TaskID != "job-pass1" || res.Pass2.TaskID != "job-pass2" {
		t.Fatalf("job ids wrong: %v / %v", res.Pass1, res.Pass2)
	}
	if res.Final() != res.Pass2 {
		t.Fatal("final version must be pass2")
	}
	if res.Transfer == nil || res.Transfer.VocalNotesBefore != 5 || res.Transfer.VocalNotesAfter != 0 || res.Transfer.OutputInsNotes != 8 {
		t.Fatalf("receipt conservation broken: %#v", res.Transfer)
	}
	if len(res.Pass2.Artifacts) == 0 {
		t.Fatal("pass2 must deliver the artifacts")
	}
	for _, a := range res.Pass2.Artifacts {
		if a.Kind == "audio" && !strings.HasSuffix(a.LocalPath, ".wav") {
			t.Fatalf("final audio artifact wrong: %#v", a)
		}
	}
	if len(res.Pass1.Artifacts) == 0 {
		t.Fatal("pass1 artifacts must stay on disk as evidence")
	}

	// Per-note structural check on what pass2 actually received.
	parsed, perr := yueParseNativeScore(res.TransferredABC)
	if perr != nil {
		t.Fatalf("transferred score must parse: %v", perr)
	}
	if len(parsed.Voices["Vocal"].Notes) != 0 {
		t.Fatalf("transferred Vocal must be silent: %#v", parsed.Voices["Vocal"].Notes)
	}
	if len(parsed.Voices["Ins"].Notes) != 8 {
		t.Fatalf("transferred Ins notes = %d, want 8", len(parsed.Voices["Ins"].Notes))
	}

	receipt := res.ReceiptMap(cfg)
	if receipt["mode"] != "score_transfer_two_stage" || receipt["pass1_task_id"] != "job-pass1" || receipt["pass2_task_id"] != "job-pass2" {
		t.Fatalf("receipt archive face wrong: %#v", receipt)
	}
	if receipt["final_version_source"] != "pass2" {
		t.Fatalf("receipt must name pass2 as the final version source: %#v", receipt)
	}
	seeds, _ := receipt["fixed_seeds"].(map[string]any)
	if seeds["seed"] != 23 || seeds["lm_seed"] != 39020261120 {
		t.Fatalf("fixed seeds must be recorded: %#v", receipt["fixed_seeds"])
	}
	joined := strings.Join(res.Actions, "\n")
	if !strings.Contains(joined, "two-stage pass1") || !strings.Contains(joined, "two-stage pass2") || !strings.Contains(joined, "double generation cost") {
		t.Fatalf("two-stage actions missing: %#v", res.Actions)
	}
}

// No replay abc in the pass1 result = classified parse failure; the pass1
// artifacts are not lost (they are already on disk) and no pass2 job fires.
func TestYueTwoStageNoReplayABCClassified(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/synth" {
			_, _ = w.Write([]byte(`{"id":"job-x"}`))
			return
		}
		if r.URL.Query().Get("result") == "1" {
			w.Header().Set("Content-Type", "multipart/mixed; boundary=vitfake")
			mw := multipart.NewWriter(w)
			_ = mw.SetBoundary("vitfake")
			part, _ := mw.CreatePart(map[string][]string{"Content-Type": {"application/json"}})
			_, _ = part.Write([]byte(`{"semantic_tokens":"1"}`)) // no abc field
			_ = mw.Close()
			return
		}
		_, _ = w.Write([]byte(`{"status":"done"}`))
	}))
	defer srv.Close()
	cfg := ProviderConfig{ID: "yue-noreplay", Dialect: "yue_local", BaseURL: srv.URL, DownloadDir: t.TempDir()}
	req := GenerationRequest{Prompt: "rendered genctx blocks", Instrumental: true, StyleTags: "Instrumental, solo piano, no vocals, no singing, no choir, no spoken words."}
	_, err := YueGenerateInstrumentalTwoStage(context.Background(), cfg, req, YueTwoStageOptions{})
	if err == nil {
		t.Fatal("missing replay abc must fail")
	}
	ge := AsError(err)
	if ge.Class != ClassParseError {
		t.Fatalf("expected response_parse_error, got %s: %v", ge.Class, err)
	}
	if !strings.Contains(ge.ProviderMessage, "no replay abc") {
		t.Fatalf("failure must name the missing replay abc: %v", err)
	}
}

// Unparseable generated abc = classified transfer failure ("invalid
// generated abc" class), no pass2 job submitted.
func TestYueTwoStageUnparseableABCClassified(t *testing.T) {
	eng := &twoStageFakeEngine{genABC: "X:1\nT:\nnot a native score\n"}
	srv := httptest.NewServer(eng.handler())
	defer srv.Close()
	cfg := ProviderConfig{ID: "yue-badabc", Dialect: "yue_local", BaseURL: srv.URL, DownloadDir: t.TempDir()}
	req := GenerationRequest{Prompt: "rendered genctx blocks", Instrumental: true, StyleTags: "Instrumental, solo piano, no vocals, no singing, no choir, no spoken words."}
	_, err := YueGenerateInstrumentalTwoStage(context.Background(), cfg, req, YueTwoStageOptions{})
	if err == nil {
		t.Fatal("unparseable generated abc must fail")
	}
	ge := AsError(err)
	if ge.Class != ClassParseError {
		t.Fatalf("expected response_parse_error, got %s: %v", ge.Class, err)
	}
	eng.mu.Lock()
	defer eng.mu.Unlock()
	if len(eng.bodies) != 1 {
		t.Fatalf("pass2 must not fire after a failed transfer: %d submits", len(eng.bodies))
	}
}
