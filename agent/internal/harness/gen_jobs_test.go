package harness

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"vit-daw-agent/internal/genapi"
)

// GEN-YUE-WIRE-1 item 5: the background job lifecycle — start returns
// immediately, the yue_local conditioning + generation + ingest run on a
// background goroutine, and status/list publish progress plus the version
// archive (original MIDI notes, source score, dispatched request, replay).

// genJobsFakeKernel is a mutex-guarded fake kernel (the background goroutine
// and the test main thread both touch it).
type genJobsFakeKernel struct {
	mu       sync.Mutex
	replies  []map[string]any
	commands []map[string]any
}

func (f *genJobsFakeKernel) SendCommand(_ context.Context, cmd map[string]any) (map[string]any, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.commands = append(f.commands, cmd)
	if len(f.replies) == 0 {
		return map[string]any{"status": "ok"}, `{"status":"ok"}`, nil
	}
	reply := f.replies[0]
	f.replies = f.replies[1:]
	return reply, "", nil
}

func (f *genJobsFakeKernel) commandNames() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, 0, len(f.commands))
	for _, c := range f.commands {
		out = append(out, fmt.Sprint(c["cmd"]))
	}
	return out
}

func yueMultipart(t *testing.T, replay map[string]any, audio []byte) (string, []byte) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	pw, err := w.CreatePart(map[string][]string{"Content-Type": {"application/json"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(pw).Encode(replay); err != nil {
		t.Fatal(err)
	}
	pa, err := w.CreatePart(map[string][]string{"Content-Type": {"audio/wav"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pa.Write(audio); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
	return w.FormDataContentType(), buf.Bytes()
}

func TestGenJobsLifecycleYueLocal(t *testing.T) {
	wav := make([]byte, 2048)
	copy(wav[0:4], "RIFF")
	copy(wav[8:12], "WAVE")

	// YUE-INSTRUMENTAL-SCORE-TRANSFER-1: instrumental yue_local jobs run the
	// two-stage score-transfer route. pass1 = the conditioned request (here
	// the converted student score); its replay carries the GENERATED score
	// the fake engine "arranged" (native dialect with vocal notes — vocals
	// in pass1 are expected: they are the transfer source). pass2 = the
	// fixed transferred score re-rendered; its replay echoes exactly what
	// was submitted, so the test asserts the transferred abc byte-exact.
	genABC := "X:1\nT:\nM:4/4\nL:1/32\nQ:1/4=88\n" +
		"V: Vocal clef=treble name=\"Vocal Melody\" snm=\"Vocal\"\n" +
		"V: Ins clef=treble name=\"Ins Melody\" snm=\"Inst.\"\n" +
		"K:C\n% intro\nV: Vocal\n\"C\"c8e8g8z8|\nV: Ins\nz32|\n"

	var mu sync.Mutex
	var submits []map[string]any
	pass1Polled := false
	engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/":
			// Engine supervision readiness probe: any HTTP answer = up.
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodPost && r.URL.Path == "/synth":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("bad /synth body: %v", err)
			}
			mu.Lock()
			id := fmt.Sprintf("job%04d", len(submits)+1)
			submits = append(submits, body)
			mu.Unlock()
			_, _ = w.Write([]byte(fmt.Sprintf(`{"id":%q}`, id)))
		case r.Method == http.MethodGet && r.URL.Path == "/job" && r.URL.Query().Get("result") == "1":
			id := r.URL.Query().Get("id")
			mu.Lock()
			var echo string
			if id == "job0001" {
				echo = genABC // pass1: the generated arrangement
			} else if len(submits) > 1 {
				echo, _ = submits[1]["abc"].(string) // pass2: echo the submitted score
			}
			mu.Unlock()
			ct, body := yueMultipart(t, map[string]any{"seed": 11, "abc": echo}, wav)
			w.Header().Set("Content-Type", ct)
			_, _ = w.Write(body)
		case r.Method == http.MethodGet && r.URL.Path == "/job":
			// pass1 answers one running poll before done; pass2 finishes at once.
			mu.Lock()
			isPass1 := len(submits) == 1
			if isPass1 && !pass1Polled {
				pass1Polled = true
				mu.Unlock()
				_, _ = w.Write([]byte(`{"status":"running"}`))
				return
			}
			mu.Unlock()
			_, _ = w.Write([]byte(`{"status":"done"}`))
		default:
			t.Errorf("unexpected engine call %s %s", r.Method, r.URL.Path)
		}
	}))
	defer engine.Close()

	writeGenapiConfig(t, []genapi.ProviderConfig{{
		ID: "yue-jobs", Dialect: "yue_local", BaseURL: engine.URL,
		PollIntervalSeconds: 0.01,
		DownloadDir:        filepath.Join(t.TempDir(), "dl"),
	}})
	jobsDir := filepath.Join(t.TempDir(), "jobs")
	t.Setenv(envGenJobsDir, jobsDir)

	kernel := &genJobsFakeKernel{replies: []map[string]any{
		projectStateReply(), // render (in_project)
		{ // student score face: get_midi_clip_notes
			"status": "ok",
			"notes": []any{
				map[string]any{"id": "n1", "pitch": 60.0, "start": 0.0, "length": 1.0},
				map[string]any{"id": "n2", "pitch": 64.0, "start": 1.0, "length": 1.0},
				map[string]any{"id": "n3", "pitch": 67.0, "start": 2.0, "length": 1.0},
			},
		},
		projectStateReply(), // score music_context read
		{"status": "ok"},    // aigc_register_job
		{"status": "ok", "clip_id": "clip-9", "asset_ref": "asset:yue", "active_take_id": "take-1"},
	}}
	h := NewWithSender(kernel, nil, nil)

	start, err := h.genJobsStart(context.Background(), map[string]any{
		"provider_id":   "yue-jobs",
		"prompt":        "欢快的钢琴独奏，突出学生旋律",
		"clip_id":       "clip-melody",
		"instrumental":  true,
		"track_id":      "1009",
		"version_label": "v1",
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if start["status"] != "ok" || start["job_id"] == "" {
		t.Fatalf("start result: %#v", start)
	}
	jobID := fmt.Sprint(start["job_id"])

	// The background leg finishes through done (bounded wait).
	store, err := h.genJobStore()
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if rec := store.get(jobID); rec != nil && (rec.State == genJobStateDone || rec.State == genJobStateFailed) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	rec := store.get(jobID)
	if rec == nil || rec.State != genJobStateDone {
		t.Fatalf("job did not finish: %+v", rec)
	}

	// Two engine jobs: pass1 (conditioned request) + pass2 (fixed
	// transferred abc re-render).
	mu.Lock()
	if len(submits) != 2 {
		t.Fatalf("engine saw %d submits, want 2 (two-stage): %#v", len(submits), submits)
	}
	pass1, pass2 := submits[0], submits[1]
	mu.Unlock()

	// pass1: the conditioned request — the student dialect score (converted
	// from the 3 MIDI notes), the prompt-derived style with the instrumental
	// tag, and no free prompt.
	abc1, _ := pass1["abc"].(string)
	if !strings.Contains(abc1, "V: Ins") || !strings.Contains(abc1, "c8e8g8") {
		t.Fatalf("converted dialect score not dispatched on pass1: %q", abc1)
	}
	style1, _ := pass1["style"].(string)
	// YUE-INSTRUMENTAL-PATH-1: the style is normalized to the official
	// instrumental recipe ("Instrumental, " prefix + no-voice conditions), so
	// the check is case-insensitive and still requires the user prompt text.
	if !strings.Contains(style1, "欢快的钢琴独奏") || !strings.Contains(strings.ToLower(style1), "instrumental") {
		t.Fatalf("style mapping lost: %q", style1)
	}
	if _, ok := pass1["prompt"]; ok {
		t.Fatalf("free prompt must not reach the engine: %#v", pass1)
	}

	// pass2: the 214808 口径 — abc = the transferred score byte-exact
	// (vocal notes moved to Ins, chords/meter/tempo preserved), style kept
	// verbatim, cot re-derived from the transferred Vocal chords (full).
	transferred, receipt, terr := genapi.YueInstrumentalizeTransfer(genABC)
	if terr != nil {
		t.Fatalf("fixture transfer: %v", terr)
	}
	if receipt.VocalNotesBefore != 3 || receipt.VocalNotesAfter != 0 || receipt.OutputInsNotes != 3 {
		t.Fatalf("fixture receipt wrong: %#v", receipt)
	}
	abc2, _ := pass2["abc"].(string)
	if abc2 != transferred {
		t.Fatalf("pass2 abc is not the byte-exact transferred score:\n got %q\nwant %q", abc2, transferred)
	}
	if pass2["style"] != style1 {
		t.Fatalf("pass2 style must stay verbatim: %#v", pass2["style"])
	}
	if pass2["cot"] != "full" {
		t.Fatalf("pass2 cot must be re-derived full (chords on the transferred Vocal), got %v", pass2["cot"])
	}

	// Status publishes the full disclosure + version archive faces.
	status, err := h.genJobsStatus(map[string]any{"job_id": jobID})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status["state"] != genJobStateDone || status["task_id"] != "job0002" {
		t.Fatalf("status: %#v", status)
	}
	dispatched, _ := status["dispatched"].(map[string]any)
	if dispatched == nil || dispatched["style"] != style1 {
		t.Fatalf("dispatched summary missing: %#v", status["dispatched"])
	}
	srcScore, _ := status["source_score"].(map[string]any)
	if srcScore == nil {
		t.Fatalf("source score face missing: %#v", status)
	}
	if srcScore["notes"] != 3 {
		t.Fatalf("original MIDI note count wrong on the status face: %#v", srcScore)
	}
	if status["replay_path"] == "" || len(fmt.Sprint(status["artifacts"])) == 0 {
		t.Fatalf("artifact faces missing: %#v", status)
	}
	// The two-stage receipt face (job archive: both jobs + conservation).
	twoStage, _ := status["two_stage"].(map[string]any)
	if twoStage == nil || twoStage["pass1_task_id"] != "job0001" || twoStage["pass2_task_id"] != "job0002" || twoStage["final_version_source"] != "pass2" {
		t.Fatalf("two_stage receipt face missing/wrong: %#v", status["two_stage"])
	}
	tr, _ := twoStage["transfer"].(map[string]any)
	if tr == nil || tr["vocal_notes_before"] != float64(3) || tr["vocal_notes_after"] != float64(0) || tr["output_ins_notes"] != float64(3) {
		t.Fatalf("two_stage conservation table wrong: %#v", twoStage["transfer"])
	}

	// The version list carries the job with its archive paths.
	list, err := h.genJobsList()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	found := false
	entries, _ := list["jobs"].([]map[string]any)
	for _, e := range entries {
		if e["job_id"] == jobID {
			found = true
			if e["version_label"] != "v1" || e["replay_path"] == "" {
				t.Fatalf("list entry incomplete: %#v", e)
			}
		}
	}
	if !found {
		t.Fatalf("job not listed: %#v", list)
	}

	// The archive JSON persists the version record with the full original
	// MIDI note list (the deterministic-conversion source of truth) and the
	// two-stage receipt.
	raw, rerr := os.ReadFile(filepath.Join(jobsDir, jobID+".json"))
	if rerr != nil {
		t.Fatalf("archive json missing: %v", rerr)
	}
	var archived genJobRecord
	if jerr := json.Unmarshal(raw, &archived); jerr != nil {
		t.Fatalf("archive json corrupt: %v", jerr)
	}
	if archived.SourceScore == nil || len(archived.SourceScore.Notes) != 3 {
		t.Fatalf("full note list must archive: %+v", archived.SourceScore)
	}
	if archived.ReplayPath == "" || len(archived.Artifacts) != 2 {
		t.Fatalf("artifact archive faces: replay=%q artifacts=%d", archived.ReplayPath, len(archived.Artifacts))
	}
	if archived.TwoStage == nil || archived.TwoStage["pass2_task_id"] != "job0002" {
		t.Fatalf("two-stage receipt must archive: %+v", archived.TwoStage)
	}
}

func TestGenJobsStartRejectsUnknownProviderFast(t *testing.T) {
	writeGenapiConfig(t, []genapi.ProviderConfig{})
	t.Setenv(envGenJobsDir, filepath.Join(t.TempDir(), "jobs"))
	h := NewWithSender(nil, nil, nil)
	out, err := h.genJobsStart(context.Background(), map[string]any{"provider_id": "nope", "prompt": "x"})
	if err == nil || out["status"] != "error" {
		t.Fatalf("unknown provider must fail fast: %#v %v", out, err)
	}
}

func TestGenJobsStorePersistenceReload(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "jobs")
	t.Setenv(envGenJobsDir, dir)
	h := NewWithSender(nil, nil, nil)
	store, err := h.genJobStore()
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	now := time.Now().UTC()
	rec := &genJobRecord{
		ID: "genjob-persist", ProviderID: "yue-jobs", Dialect: "yue_local",
		State: genJobStateDone, StartedAt: now, VersionLabel: "v2",
		SourceScore: &genapi.YueScoreInput{
			Notes:     []genapi.YueNote{{Pitch: 60, Start: 0, Length: 1}},
			Numerator: 2, Denominator: 4, TempoBPM: 88, Key: "C",
		},
	}
	if err := store.add(rec); err != nil {
		t.Fatalf("add: %v", err)
	}
	// A fresh process (new harness) sees the archived version.
	h2 := NewWithSender(nil, nil, nil)
	list, err := h2.genJobsList()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	found := false
	entries, _ := list["jobs"].([]map[string]any)
	for _, e := range entries {
		if e["job_id"] == "genjob-persist" && e["source_notes"] == 1 {
			found = true
		}
	}
	if !found {
		t.Fatalf("archived job not reloaded: %#v", list)
	}
}
