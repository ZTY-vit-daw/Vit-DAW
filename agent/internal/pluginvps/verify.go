package pluginvps

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"vit-daw-agent/internal/spal"
	"vit-daw-agent/internal/vst3host"
)

type VerifierWorker interface {
	Call(context.Context, string, map[string]any) (vst3host.Response, error)
	Close() error
}

type VerifyOptions struct {
	WorkerPath  string
	Now         func() time.Time
	StartWorker func(string) (VerifierWorker, error)
}
type VerifyResult struct {
	Document   Document `json:"document"`
	Checks     int      `json:"checks"`
	Parameters int      `json:"parameters"`
	Logs       []string `json:"logs,omitempty"`
}
type workerParameter struct {
	ID         string   `json:"id"`
	Normalized float64  `json:"normalized_value"`
	Display    string   `json:"display_value"`
	Automation string   `json:"automation"`
	Choices    []string `json:"display_choices"`
}
type workerSnapshot struct {
	Identity struct {
		Name            string `json:"name"`
		Format          string `json:"format"`
		Version         string `json:"version"`
		InstallPath     string `json:"install_path"`
		FileFingerprint string `json:"file_fingerprint"`
	} `json:"identity"`
	Parameters []workerParameter `json:"parameters"`
}
type writeResult struct {
	TransactionID string         `json:"transaction_id"`
	Fresh         workerSnapshot `json:"fresh_readback"`
}
type rollbackResult struct {
	RollbackVerified bool           `json:"rollback_verified"`
	Fresh            workerSnapshot `json:"fresh_readback"`
}

func Verify(ctx context.Context, d Document, options VerifyOptions) (VerifyResult, error) {
	d.Verified = false
	d.Verification = nil
	if err := d.Validate(false); err != nil {
		return VerifyResult{}, err
	}
	startWorker := options.StartWorker
	if startWorker == nil {
		startWorker = func(path string) (VerifierWorker, error) { return vst3host.Start(path) }
	}
	worker, err := startWorker(options.WorkerPath)
	if err != nil {
		return VerifyResult{}, err
	}
	defer worker.Close()
	loaded, err := worker.Call(ctx, "load", map[string]any{"plugin_path": d.Plugin.InstallPath, "sample_rate": 48000, "block_size": 512})
	if err != nil {
		return VerifyResult{}, fmt.Errorf("load plugin: %w", err)
	}
	var snap workerSnapshot
	if err = json.Unmarshal(loaded.Result, &snap); err != nil {
		return VerifyResult{}, fmt.Errorf("decode load snapshot: %w", err)
	}
	if !strings.EqualFold(strings.TrimSpace(snap.Identity.Name), strings.TrimSpace(d.Plugin.Name)) {
		return VerifyResult{}, fmt.Errorf("loaded plugin name %q does not match VPS %q", snap.Identity.Name, d.Plugin.Name)
	}
	if !strings.EqualFold(strings.TrimSpace(snap.Identity.FileFingerprint), strings.TrimSpace(d.Plugin.InstallationHash)) {
		return VerifyResult{}, fmt.Errorf("installation hash mismatch: live %s file %s", snap.Identity.FileFingerprint, d.Plugin.InstallationHash)
	}
	index := map[string]workerParameter{}
	ids := make([]string, 0, len(snap.Parameters))
	for _, p := range snap.Parameters {
		index[p.ID] = p
		ids = append(ids, p.ID)
	}
	for _, id := range BindingParameterIDs(d.Bindings) {
		if _, ok := index[id]; !ok {
			return VerifyResult{}, fmt.Errorf("binding parameter %s is absent from fresh surface", id)
		}
	}
	checks := 0
	logs := append([]string(nil), loaded.Logs...)
	verifyP := func(slot string, b spal.ParameterBinding) error {
		points := []float64{0, 0.5, 1}
		if isToggleUnit(b.Unit) {
			points = []float64{0, 1}
		}
		for _, n := range points {
			expected := b.Min + (b.Max-b.Min)*n
			if strings.EqualFold(b.Scale, "log") && b.Min > 0 && b.Max > b.Min {
				expected = b.Min * math.Pow(b.Max/b.Min, n)
			}
			display, callLogs, callErr := writeReadbackRollback(ctx, worker, b.ParameterID, n)
			logs = append(logs, callLogs...)
			if callErr != nil {
				return fmt.Errorf("%s: %w", slot, callErr)
			}
			if err = compareDisplayValue(display, expected, n, b.Unit); err != nil {
				return fmt.Errorf("%s at %.3f: %w", slot, n, err)
			}
			checks++
		}
		return nil
	}
	verifyE := func(slot string, b spal.EnumParameterBinding) error {
		labels := make([]string, 0, len(b.Values))
		for l := range b.Values {
			labels = append(labels, l)
		}
		sort.Strings(labels)
		for _, label := range labels {
			displayLabel := enumDisplayLabel(b, label)
			display, callLogs, callErr := writeReadbackRollback(ctx, worker, b.ParameterID, b.Values[label])
			logs = append(logs, callLogs...)
			if callErr != nil {
				return fmt.Errorf("%s: %w", slot, callErr)
			}
			if !enumTextMatches(display, displayLabel) {
				return fmt.Errorf("%s label %q (display %q) read back as %q", slot, label, displayLabel, display)
			}
			checks++
		}
		return nil
	}
	keys := make([]string, 0, len(d.Bindings.Bands))
	for k := range d.Bindings.Bands {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		b := d.Bindings.Bands[k]
		if b.Allocated != nil {
			if err = verifyP(k+".allocated", *b.Allocated); err != nil {
				return VerifyResult{}, err
			}
		}
		if err = verifyP(k+".enabled", b.Enabled); err != nil {
			return VerifyResult{}, err
		}
		if err = verifyE(k+".response_shape", b.ResponseShape); err != nil {
			return VerifyResult{}, err
		}
		if err = verifyP(k+".frequency_hz", b.FrequencyHz); err != nil {
			return VerifyResult{}, err
		}
		if err = verifyP(k+".gain_db", b.GainDB); err != nil {
			return VerifyResult{}, err
		}
		if err = verifyP(k+".q", b.Q); err != nil {
			return VerifyResult{}, err
		}
	}
	for name, p := range map[string]*spal.EQV2PassFilterBinding{"highpass": d.Bindings.HighPass, "lowpass": d.Bindings.LowPass} {
		if p == nil {
			continue
		}
		if p.Allocated != nil {
			if err = verifyP(name+".allocated", *p.Allocated); err != nil {
				return VerifyResult{}, err
			}
		}
		if p.ResponseShape != nil {
			if err = verifyE(name+".response_shape", *p.ResponseShape); err != nil {
				return VerifyResult{}, err
			}
		}
		if err = verifyP(name+".enabled", p.Enabled); err != nil {
			return VerifyResult{}, err
		}
		if err = verifyP(name+".cutoff_frequency_hz", p.CutoffFrequencyHz); err != nil {
			return VerifyResult{}, err
		}
		if err = verifyE(name+".slope_db_per_octave", p.SlopeDBPerOctave); err != nil {
			return VerifyResult{}, err
		}
	}
	if p := d.Bindings.Output; p != nil {
		if p.Bypass != nil {
			if err = verifyP("output.bypass", *p.Bypass); err != nil {
				return VerifyResult{}, err
			}
		}
		if p.DryMix != nil {
			if err = verifyP("output.dry_mix_percent", *p.DryMix); err != nil {
				return VerifyResult{}, err
			}
		}
		if p.OutputGainDB != nil {
			if err = verifyP("output.output_gain_db", *p.OutputGainDB); err != nil {
				return VerifyResult{}, err
			}
		}
	}
	now := time.Now().UTC()
	if options.Now != nil {
		now = options.Now().UTC()
	}
	d.Verified = true
	d.Verification = &Verification{InstallationHash: d.Plugin.InstallationHash, ParameterSurfaceHash: HashSurface(ids), VerifiedAt: now, WorkerProtocol: vst3host.WorkerProtocol, Checks: checks}
	if err = d.Validate(true); err != nil {
		return VerifyResult{}, err
	}
	return VerifyResult{Document: d, Checks: checks, Parameters: len(ids), Logs: logs}, nil
}

func writeReadbackRollback(ctx context.Context, w VerifierWorker, id string, normalized float64) (string, []string, error) {
	resp, err := w.Call(ctx, "write", map[string]any{"changes": []map[string]any{{"id": id, "normalized": normalized}}})
	if err != nil {
		return "", nil, err
	}
	var wr writeResult
	if err = json.Unmarshal(resp.Result, &wr); err != nil {
		return "", resp.Logs, err
	}
	if wr.TransactionID == "" {
		return "", resp.Logs, fmt.Errorf("write omitted transaction_id")
	}
	var found *workerParameter
	for i := range wr.Fresh.Parameters {
		if wr.Fresh.Parameters[i].ID == id {
			found = &wr.Fresh.Parameters[i]
			break
		}
	}
	if found == nil {
		return "", resp.Logs, fmt.Errorf("fresh readback omitted parameter %s", id)
	}
	if math.Abs(found.Normalized-normalized) > 1e-4 {
		return "", resp.Logs, fmt.Errorf("fresh normalized readback %.9f != %.9f", found.Normalized, normalized)
	}
	rb, rbErr := w.Call(ctx, "rollback", map[string]any{"transaction_id": wr.TransactionID})
	logs := append(resp.Logs, rb.Logs...)
	if rbErr != nil {
		return "", logs, fmt.Errorf("rollback: %w", rbErr)
	}
	var rr rollbackResult
	if err = json.Unmarshal(rb.Result, &rr); err != nil {
		return "", logs, err
	}
	if !rr.RollbackVerified {
		return "", logs, fmt.Errorf("rollback verification failed")
	}
	return strings.TrimSpace(found.Display), logs, nil
}

var numberPattern = regexp.MustCompile(`[-+]?\d+(?:[.,]\d+)?(?:[eE][-+]?\d+)?`)

func isToggleUnit(unit string) bool {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "toggle", "bool", "boolean", "switch":
		return true
	default:
		return false
	}
}

func compareDisplayValue(display string, expected, normalized float64, unit string) error {
	if isToggleUnit(unit) {
		label := normalizeLabel(display)
		if normalized <= 0 && (label == "off" || label == "disabled" || label == "unused" || label == "false" || label == "0") {
			return nil
		}
		if normalized >= 1 && (label == "on" || label == "enabled" || label == "used" || label == "true" || label == "1") {
			return nil
		}
	}
	m := numberPattern.FindString(display)
	if m == "" {
		return fmt.Errorf("display text %q has no numeric value", display)
	}
	actual, err := strconv.ParseFloat(strings.ReplaceAll(m, ",", "."), 64)
	if err != nil {
		return err
	}
	tol := math.Max(0.02*math.Max(1, math.Abs(expected)), 0.05)
	if math.Abs(actual-expected) > tol {
		return fmt.Errorf("display text %q numeric %.6f does not match expected %.6f", display, actual, expected)
	}
	return nil
}
func enumDisplayLabel(binding spal.EnumParameterBinding, semanticLabel string) string {
	if label := strings.TrimSpace(binding.DisplayLabels[semanticLabel]); label != "" {
		return label
	}
	return semanticLabel
}

func enumTextMatches(display, label string) bool {
	a := normalizeLabel(display)
	b := normalizeLabel(label)
	return a == b || strings.Contains(a, b) || strings.Contains(b, a)
}
func normalizeLabel(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	r := strings.NewReplacer(" ", "", "_", "", "-", "", "/", "")
	return r.Replace(s)
}
