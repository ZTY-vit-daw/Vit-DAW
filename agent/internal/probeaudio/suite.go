// Package probeaudio generates deterministic audio assets for plugin probing,
// behavior and FXM counterfactual measurements.
package probeaudio

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const (
	SuiteID       = "vit.pluginprobe.audio.v1@48000"
	SchemaVersion = "vit.pluginprobe.audio_manifest.v1"
	SampleRate    = 48000
	Channels      = 2
	BitsPerSample = 24
)

type Manifest struct {
	SchemaVersion string    `json:"schema_version"`
	SuiteID       string    `json:"suite_id"`
	SampleRate    int       `json:"sample_rate"`
	Channels      int       `json:"channels"`
	BitsPerSample int       `json:"bits_per_sample"`
	GeneratedAt   time.Time `json:"generated_at"`
	Assets        []Asset   `json:"assets"`
	UsagePolicy   []string  `json:"usage_policy"`
}

type Asset struct {
	ID          string   `json:"id"`
	File        string   `json:"file"`
	DurationSec float64  `json:"duration_seconds"`
	Purpose     []string `json:"purpose"`
	SHA256      string   `json:"sha256"`
}

type signalSpec struct {
	id       string
	file     string
	duration float64
	purpose  []string
	render   func(frame int, t float64) (float64, float64)
}

func Generate(root string, now time.Time) (Manifest, error) {
	root = filepath.Clean(root)
	if root == "" || root == "." {
		return Manifest{}, fmt.Errorf("probe-audio output directory is required")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return Manifest{}, err
	}
	specs := suiteSpecs()
	manifest := Manifest{SchemaVersion: SchemaVersion, SuiteID: SuiteID, SampleRate: SampleRate, Channels: Channels, BitsPerSample: BitsPerSample, GeneratedAt: now.UTC(), UsagePolicy: []string{"Use the same source bytes, window, sample rate, routing, gain and automation for bypass_chain and processed_chain renders.", "Probe assets are engineering evidence, not musical references.", "Dynamic/nonlinear results remain conditional on input level and measurement window."}}
	for _, spec := range specs {
		path := filepath.Join(root, spec.file)
		if err := writePCM24(path, spec.duration, spec.render); err != nil {
			return Manifest{}, err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return Manifest{}, err
		}
		sum := sha256.Sum256(data)
		manifest.Assets = append(manifest.Assets, Asset{ID: spec.id, File: spec.file, DurationSec: spec.duration, Purpose: spec.purpose, SHA256: "sha256:" + hex.EncodeToString(sum[:])})
	}
	sort.Slice(manifest.Assets, func(i, j int) bool { return manifest.Assets[i].ID < manifest.Assets[j].ID })
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return Manifest{}, err
	}
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), append(data, '\n'), 0o644); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func suiteSpecs() []signalSpec {
	return []signalSpec{
		{id: "impulse_stereo", file: "impulse_stereo_48k_2s.wav", duration: 2, purpose: []string{"impulse_response", "latency", "phase"}, render: func(frame int, _ float64) (float64, float64) {
			if frame == 256 {
				return .5, .5
			}
			return 0, 0
		}},
		{id: "log_sweep_20hz_20khz", file: "log_sweep_20hz_20khz_48k_10s.wav", duration: 10, purpose: []string{"static_frequency_response", "filter_shape", "harmonic_observation"}, render: logSweep(20, 20000, 10, dbAmplitude(-12))},
		{id: "multitone_31band", file: "multitone_31band_48k_10s.wav", duration: 10, purpose: []string{"band_energy", "static_chain_delta", "nonlinear_intermodulation_observation"}, render: multitone()},
		{id: "stepped_sine_levels", file: "stepped_sine_1khz_levels_48k_12s.wav", duration: 12, purpose: []string{"compressor_threshold", "ratio", "level_dependent_behavior"}, render: steppedSine()},
		{id: "transient_burst", file: "transient_burst_48k_10s.wav", duration: 10, purpose: []string{"attack", "release", "transient_shaping"}, render: transientBurst()},
		{id: "stereo_phase_probe", file: "stereo_phase_probe_48k_10s.wav", duration: 10, purpose: []string{"stereo_width", "phase", "channel_link"}, render: stereoProbe()},
		{id: "tail_probe", file: "tail_probe_48k_12s.wav", duration: 12, purpose: []string{"reverb_tail", "delay_feedback", "gate_release"}, render: tailProbe()},
	}
}

func logSweep(startHz, endHz, duration, amplitude float64) func(int, float64) (float64, float64) {
	logRatio := math.Log(endHz / startHz)
	phaseScale := 2 * math.Pi * startHz * duration / logRatio
	return func(_ int, t float64) (float64, float64) {
		fade := edgeFade(t, duration, .05)
		phase := phaseScale * (math.Exp(t*logRatio/duration) - 1)
		value := amplitude * fade * math.Sin(phase)
		return value, value
	}
}

func multitone() func(int, float64) (float64, float64) {
	frequencies := []float64{20, 25, 31.5, 40, 50, 63, 80, 100, 125, 160, 200, 250, 315, 400, 500, 630, 800, 1000, 1250, 1600, 2000, 2500, 3150, 4000, 5000, 6300, 8000, 10000, 12500, 16000, 20000}
	scale := dbAmplitude(-15) / math.Sqrt(float64(len(frequencies)))
	return func(_ int, t float64) (float64, float64) {
		value := 0.0
		for i, frequency := range frequencies {
			phase := -math.Pi * float64(i*(i-1)) / float64(len(frequencies))
			value += scale * math.Sin(2*math.Pi*frequency*t+phase)
		}
		value = clamp(value)
		return value, value
	}
}

func steppedSine() func(int, float64) (float64, float64) {
	levels := []float64{-36, -30, -24, -18, -12, -6}
	return func(_ int, t float64) (float64, float64) {
		index := int(t / 2)
		if index >= len(levels) {
			index = len(levels) - 1
		}
		local := t - float64(index)*2
		value := dbAmplitude(levels[index]) * edgeFade(local, 2, .02) * math.Sin(2*math.Pi*1000*t)
		return value, value
	}
}

func transientBurst() func(int, float64) (float64, float64) {
	return func(frame int, t float64) (float64, float64) {
		local := math.Mod(t, 1)
		if local > .12 {
			return 0, 0
		}
		noise := deterministicNoise(uint32(frame))
		envelope := math.Exp(-local * 45)
		value := .65 * envelope * (.65*noise + .35*math.Sin(2*math.Pi*120*t))
		return value, value
	}
}

func stereoProbe() func(int, float64) (float64, float64) {
	return func(_ int, t float64) (float64, float64) {
		value := dbAmplitude(-12) * math.Sin(2*math.Pi*440*t)
		switch int(t / 2) {
		case 0:
			return value, value
		case 1:
			return value, -value
		case 2:
			return value, 0
		case 3:
			return 0, value
		default:
			return value, dbAmplitude(-6) * math.Sin(2*math.Pi*660*t)
		}
	}
}

func tailProbe() func(int, float64) (float64, float64) {
	return func(frame int, t float64) (float64, float64) {
		if t > .25 {
			return 0, 0
		}
		envelope := math.Sin(math.Pi * t / .25)
		value := .5 * envelope * deterministicNoise(uint32(frame+17))
		return value, value
	}
}

func writePCM24(path string, duration float64, render func(int, float64) (float64, float64)) error {
	frames := int(math.Round(duration * SampleRate))
	bytesPerSample := BitsPerSample / 8
	dataSize := frames * Channels * bytesPerSample
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	write := func(value any) error { return binary.Write(file, binary.LittleEndian, value) }
	if _, err := file.Write([]byte("RIFF")); err != nil {
		return err
	}
	if err := write(uint32(36 + dataSize)); err != nil {
		return err
	}
	if _, err := file.Write([]byte("WAVEfmt ")); err != nil {
		return err
	}
	if err := write(uint32(16)); err != nil {
		return err
	}
	if err := write(uint16(1)); err != nil {
		return err
	}
	if err := write(uint16(Channels)); err != nil {
		return err
	}
	if err := write(uint32(SampleRate)); err != nil {
		return err
	}
	if err := write(uint32(SampleRate * Channels * bytesPerSample)); err != nil {
		return err
	}
	if err := write(uint16(Channels * bytesPerSample)); err != nil {
		return err
	}
	if err := write(uint16(BitsPerSample)); err != nil {
		return err
	}
	if _, err := file.Write([]byte("data")); err != nil {
		return err
	}
	if err := write(uint32(dataSize)); err != nil {
		return err
	}
	buffer := make([]byte, Channels*bytesPerSample)
	for frame := 0; frame < frames; frame++ {
		left, right := render(frame, float64(frame)/SampleRate)
		for channel, sample := range []float64{left, right} {
			value := int32(math.Round(clamp(sample) * 8388607))
			offset := channel * bytesPerSample
			buffer[offset] = byte(value)
			buffer[offset+1] = byte(value >> 8)
			buffer[offset+2] = byte(value >> 16)
		}
		if _, err := file.Write(buffer); err != nil {
			return err
		}
	}
	return file.Sync()
}

func edgeFade(t, duration, fade float64) float64 {
	if t < fade {
		return .5 - .5*math.Cos(math.Pi*t/fade)
	}
	if t > duration-fade {
		return .5 - .5*math.Cos(math.Pi*(duration-t)/fade)
	}
	return 1
}
func dbAmplitude(db float64) float64 { return math.Pow(10, db/20) }
func clamp(value float64) float64 {
	if value > .98 {
		return .98
	}
	if value < -.98 {
		return -.98
	}
	return value
}
func deterministicNoise(seed uint32) float64 {
	value := seed*1664525 + 1013904223
	return float64(value)/float64(math.MaxUint32)*2 - 1
}
