package agentloop

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/config"
	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/mixboard"
	"vit-daw-agent/internal/planner"
	agentruntime "vit-daw-agent/internal/runtime"
)

type fakeMessageCompleter struct {
	responses []string
	errors    []error
	calls     [][]llm.Message
}

func (f *fakeMessageCompleter) Complete(_ context.Context, _ config.EngineConfig, messages []llm.Message) (string, error) {
	f.calls = append(f.calls, append([]llm.Message(nil), messages...))
	if len(f.errors) > 0 {
		err := f.errors[0]
		f.errors = f.errors[1:]
		if err != nil {
			return "", err
		}
	}
	if len(f.responses) == 0 {
		return `{"final":true,"reply":"done"}`, nil
	}
	out := f.responses[0]
	f.responses = f.responses[1:]
	return out, nil
}

type fakeMessageExecutor struct {
	calls                       []planner.ToolCall
	confirmed                   []bool
	omitNoteWriteObservedNotes  bool
	mixObservationResult        map[string]any
	projectSampleRateHz         int
	preflightSampleRateDecision map[string]any
	audioAnalysisStatusResults  []map[string]any
	projectState                map[string]any
}

func isTestMixObservationTool(tool string) bool {
	switch strings.TrimSpace(tool) {
	case "mix.observe", "mix.request_observation":
		return true
	default:
		return false
	}
}

func (f *fakeMessageExecutor) currentProjectSampleRateHz() int {
	if f.projectSampleRateHz > 0 {
		return f.projectSampleRateHz
	}
	return 48000
}

func (f *fakeMessageExecutor) currentSourceSampleRateHz() int {
	if sr := firstPositiveMapInt(f.preflightSampleRateDecision, "source_sample_rate_hz"); sr > 0 {
		return sr
	}
	return f.currentProjectSampleRateHz()
}

func (f *fakeMessageExecutor) currentSampleRateMismatchCount() int {
	if len(f.preflightSampleRateDecision) == 0 {
		return 0
	}
	if f.currentSourceSampleRateHz() != f.currentProjectSampleRateHz() {
		return 3
	}
	return 0
}

func testReadyImportTIMProjection() map[string]any {
	return map[string]any{
		"status": "ready",
		"technical_summary": map[string]any{
			"track_count":                3,
			"clip_count":                 3,
			"source_present_count":       3,
			"acoustic_ready_track_count": 3,
			"pcm_source_count":           3,
			"sample_rate_counts":         map[string]any{"48000_hz": 3},
			"bit_depth_counts":           map[string]any{"24_bit": 3},
			"channel_count_counts":       map[string]any{"2_ch": 3},
		},
		"coverage": map[string]any{
			"source_path":       map[string]any{"status": "ready", "known_count": 3, "total_count": 3},
			"playback_validity": map[string]any{"status": "missing", "known_count": 0, "total_count": 3, "missing_count": 3},
			"format_family":     map[string]any{"status": "ready", "known_count": 3, "total_count": 3},
			"sample_rate":       map[string]any{"status": "ready", "known_count": 3, "total_count": 3},
			"bit_depth":         map[string]any{"status": "ready", "known_count": 3, "total_count": 3},
			"channel_count":     map[string]any{"status": "ready", "known_count": 3, "total_count": 3},
			"acoustic_package":  map[string]any{"status": "ready", "known_count": 3, "total_count": 3},
		},
		"risk_summary": map[string]any{
			"overall_risk":  "none",
			"issue_count":   0,
			"primary_codes": []string{},
		},
	}
}

func (f *fakeMessageExecutor) setProjectStateClipGain(clipID string, gainDB float64) {
	clipID = strings.TrimSpace(clipID)
	if f == nil || clipID == "" || len(f.projectState) == 0 {
		return
	}
	setClip := func(clip map[string]any) bool {
		if firstMapText(clip, "clip_id", "id", "item_id") != clipID {
			return false
		}
		clip["clip_gain_db"] = gainDB
		clip["gain_db"] = gainDB
		return true
	}
	for _, clip := range messageLoopMapRows(f.projectState["clips"]) {
		if setClip(clip) {
			return
		}
	}
	if tracks, ok := f.projectState["tracks"].([]map[string]any); ok {
		for _, track := range tracks {
			for _, key := range []string{"clips", "clip_summaries", "audio_clips"} {
				if clips, ok := track[key].([]map[string]any); ok {
					for _, clip := range clips {
						if setClip(clip) {
							return
						}
					}
				}
				if clips, ok := track[key].([]any); ok {
					for _, raw := range clips {
						if clip, ok := raw.(map[string]any); ok && setClip(clip) {
							return
						}
					}
				}
			}
		}
	}
	for _, track := range messageLoopMapRows(f.projectState["tracks"]) {
		for _, key := range []string{"clips", "clip_summaries", "audio_clips"} {
			for _, clip := range messageLoopMapRows(track[key]) {
				if setClip(clip) {
					return
				}
			}
		}
	}
}

func (f *fakeMessageExecutor) setProjectStateTrackVolume(trackID string, volumeDB float64) {
	trackID = strings.TrimSpace(trackID)
	if f == nil || trackID == "" || len(f.projectState) == 0 {
		return
	}
	setTrack := func(track map[string]any) bool {
		if firstMapText(track, "track_id", "id") != trackID {
			return false
		}
		track["volume_db"] = volumeDB
		track["fader_db"] = volumeDB
		return true
	}
	for _, track := range messageLoopMapRows(f.projectState["tracks"]) {
		if setTrack(track) {
			return
		}
	}
	for _, track := range messageLoopMapRows(f.projectState["visible_tracks"]) {
		if setTrack(track) {
			return
		}
	}
	daw := messageLoopMapValue(f.projectState["daw_state_summary"])
	for _, track := range messageLoopMapRows(daw["tracks"]) {
		if setTrack(track) {
			return
		}
	}
}

func messageLoopB1TrackVolumeFromProjectState(projectState map[string]any, trackID string) (float64, bool) {
	trackID = strings.TrimSpace(trackID)
	if trackID == "" {
		return 0, false
	}
	for _, track := range messageLoopMapRows(projectState["tracks"]) {
		if firstMapText(track, "track_id", "id") == trackID {
			return firstNumericMapValue(track, "volume_db", "fader_db", "track_gain_db", "gain_db", "db")
		}
	}
	for _, track := range messageLoopMapRows(projectState["visible_tracks"]) {
		if firstMapText(track, "track_id", "id") == trackID {
			return firstNumericMapValue(track, "volume_db", "fader_db", "track_gain_db", "gain_db", "db")
		}
	}
	daw := messageLoopMapValue(projectState["daw_state_summary"])
	for _, track := range messageLoopMapRows(daw["tracks"]) {
		if firstMapText(track, "track_id", "id") == trackID {
			return firstNumericMapValue(track, "volume_db", "fader_db", "track_gain_db", "gain_db", "db")
		}
	}
	return 0, false
}

func (f *fakeMessageExecutor) RunToolCall(_ context.Context, in executorpkg.Input) (executorpkg.Result, error) {
	f.calls = append(f.calls, in.ToolCall)
	f.confirmed = append(f.confirmed, in.Confirmed)
	switch strings.TrimSpace(in.ToolCall.Tool) {
	case "project.state":
		state := f.projectState
		if len(state) == 0 {
			state = map[string]any{
				"status":           "ok",
				"track_count":      2,
				"clip_count":       2,
				"duration_seconds": 12.5,
				"sample_rate_hz":   48000,
				"bit_depth":        24,
				"markers": []map[string]any{{
					"marker_id":     "marker_1",
					"name":          "Verse",
					"kind":          "range",
					"start_seconds": 0.0,
					"end_seconds":   12.5,
				}},
				"tracks": []map[string]any{{
					"track_id":       "track_drums",
					"track_name":     "Drums",
					"is_audio_track": true,
					"clips": []map[string]any{{
						"clip_id":        "clip_drums",
						"start_seconds":  0.0,
						"length_seconds": 12.5,
					}},
				}, {
					"track_id":       "track_vocal",
					"track_name":     "Vocal",
					"is_audio_track": true,
					"clips": []map[string]any{{
						"clip_id":        "clip_vocal",
						"start_seconds":  0.0,
						"length_seconds": 10.0,
					}},
				}},
			}
		}
		return executorpkg.Result{
			ToolCallID:  in.ToolCall.ID,
			Tool:        in.ToolCall.Tool,
			CommandName: "get_project_state",
			Status:      "ok",
			Result:      state,
		}, nil
	case "track.add":
		return executorpkg.Result{
			ToolCallID:    in.ToolCall.ID,
			Tool:          in.ToolCall.Tool,
			CommandName:   "track.add",
			Status:        "ok",
			Result:        map[string]any{"track_id": "1007", "track_name": "Track 1"},
			ObservedState: map[string]any{"tracks": []map[string]any{{"track_id": "1007", "track_name": "Track 1"}}},
		}, nil
	case "track.folder.create", "folder_track.create":
		return executorpkg.Result{
			ToolCallID:  in.ToolCall.ID,
			Tool:        in.ToolCall.Tool,
			CommandName: "folder_track.create",
			Status:      "ok",
			Result: map[string]any{
				"track_id":            "1322",
				"folder_track_id":     "1322",
				"track_name":          firstNonEmpty(firstMapText(in.ToolCall.Args, "folder_name"), "Folder"),
				"name":                firstNonEmpty(firstMapText(in.ToolCall.Args, "folder_name"), "Folder"),
				"is_folder_track":     true,
				"routing_bus_enabled": false,
				"child_track_count":   0,
				"folder_behavior":     "container",
			},
			ObservedState: map[string]any{"tracks": []map[string]any{{"track_id": "1322", "track_name": "Folder", "track_type": "folder"}}},
		}, nil
	case "track.move_to_folder":
		return executorpkg.Result{
			ToolCallID:  in.ToolCall.ID,
			Tool:        in.ToolCall.Tool,
			CommandName: "track.move_to_folder",
			Status:      "ok",
			Result: map[string]any{
				"track_id":        in.ToolCall.Args["track_id"],
				"folder_track_id": in.ToolCall.Args["folder_track_id"],
				"status":          "ok",
			},
		}, nil
	case "midi.create_clip":
		return executorpkg.Result{
			ToolCallID:  in.ToolCall.ID,
			Tool:        in.ToolCall.Tool,
			CommandName: "create_midi_clip",
			Status:      "ok",
			Result: map[string]any{
				"track_id":    "1007",
				"clip_id":     "clip_1",
				"new_clip_id": "clip_1",
			},
			ObservedState: map[string]any{"tracks": []map[string]any{{
				"track_id": "1007",
				"clips": []map[string]any{{
					"id":    "clip_1",
					"type":  "midi",
					"notes": []map[string]any{},
				}},
			}}},
		}, nil
	case "midi.apply_note_patch":
		notes := []map[string]any{
			{"id": "note_1", "pitch": 60, "start": 0, "length": 1},
			{"id": "note_2", "pitch": 64, "start": 1, "length": 1},
		}
		observedNotes := notes
		if f.omitNoteWriteObservedNotes {
			observedNotes = []map[string]any{}
		}
		return executorpkg.Result{
			ToolCallID:  in.ToolCall.ID,
			Tool:        in.ToolCall.Tool,
			CommandName: "apply_midi_note_patch",
			Status:      "ok",
			Result: map[string]any{
				"track_id":          "1007",
				"clip_id":           "clip_1",
				"inserted_count":    2,
				"inserted_note_ids": []string{"note_1", "note_2"},
				"notes":             notes,
			},
			ObservedState: map[string]any{"tracks": []map[string]any{{
				"track_id": "1007",
				"clips": []map[string]any{{
					"id":    "clip_1",
					"notes": observedNotes,
				}},
			}}},
		}, nil
	case "midi.read_notes", "midi.read_clip_notes":
		return executorpkg.Result{
			ToolCallID:  in.ToolCall.ID,
			Tool:        in.ToolCall.Tool,
			CommandName: "get_midi_clip_notes",
			Status:      "ok",
			Result: map[string]any{
				"track_id": "1007",
				"clip_id":  "clip_1",
				"notes": []map[string]any{
					{"id": "note_1", "pitch": 60, "start": 0, "length": 1},
					{"id": "note_2", "pitch": 64, "start": 1, "length": 1},
				},
			},
		}, nil
	case "clip.import_media_to_track":
		return executorpkg.Result{
			ToolCallID:  in.ToolCall.ID,
			Tool:        in.ToolCall.Tool,
			CommandName: "import_media_to_track",
			Status:      "ok",
			Result: map[string]any{
				"track_id":   "1007",
				"track_name": "Lead Vocal",
				"clip_id":    "1011",
				"clip_name":  "lead_vocal.wav",
			},
			ObservedState: map[string]any{"tracks": []map[string]any{{
				"track_id":   "1007",
				"track_name": "Lead Vocal",
				"clips": []map[string]any{{
					"id":   "1011",
					"name": "lead_vocal.wav",
					"type": "audio",
				}},
			}}},
		}, nil
	case "project.import_preflight":
		folderPath := firstMapText(in.ToolCall.Args, "folder_path")
		if folderPath == "" {
			folderPath = `E:\BaiduNetdiskDownload\sattelites`
		}
		projectSampleRateHz := f.currentProjectSampleRateHz()
		result := map[string]any{
			"status":  "ok",
			"command": "project.import_preflight",
			"summary": map[string]any{
				"discovered_audio_file_count": 3,
				"readable_file_count":         3,
				"tracks_to_create":            3,
			},
			"audio_settings_snapshot": map[string]any{
				"sample_rate_hz":   projectSampleRateHz,
				"record_bit_depth": 24,
				"pcm_format":       "int24",
			},
			"import_plan": map[string]any{
				"folder_path":                folderPath,
				"intended_mode":              "stems_folder",
				"target_policy":              "create_tracks",
				"start_time_seconds":         0.0,
				"tracks_to_create":           3,
				"track_plan_preview":         []map[string]any{{"file_name": "drums.wav"}, {"file_name": "bass.wav"}, {"file_name": "vocal.wav"}},
				"command_timeout_ms":         120000,
				"requires_user_confirmation": true,
			},
		}
		if len(f.preflightSampleRateDecision) > 0 {
			decision := cloneMap(f.preflightSampleRateDecision)
			result["sample_rate_decision"] = decision
			if patch := messageLoopMapValue(decision["project_audio_settings_patch"]); len(patch) > 0 {
				result["project_audio_settings_patch"] = patch
			}
			summary := result["summary"].(map[string]any)
			summary["sample_rate_decision"] = decision
			if patch := messageLoopMapValue(decision["project_audio_settings_patch"]); len(patch) > 0 {
				summary["project_audio_settings_patch"] = patch
			}
			plan := result["import_plan"].(map[string]any)
			plan["sample_rate_decision"] = decision
			if patch := messageLoopMapValue(decision["project_audio_settings_patch"]); len(patch) > 0 {
				plan["project_audio_settings_patch"] = patch
			}
		}
		return executorpkg.Result{
			ToolCallID:  in.ToolCall.ID,
			Tool:        in.ToolCall.Tool,
			CommandName: "project.import_preflight",
			Status:      "ok",
			Result:      result,
		}, nil
	case "project.set_audio_settings":
		settings := messageLoopMapValue(in.ToolCall.Args["audio_settings"])
		if sr := firstPositiveMapInt(settings, "sample_rate_hz"); sr > 0 {
			f.projectSampleRateHz = sr
		}
		return executorpkg.Result{
			ToolCallID:  in.ToolCall.ID,
			Tool:        in.ToolCall.Tool,
			CommandName: "project.set_audio_settings",
			Status:      "ok",
			Result: map[string]any{
				"status":  "ok",
				"command": "project.set_audio_settings",
				"audio_settings": map[string]any{
					"sample_rate_hz":   f.currentProjectSampleRateHz(),
					"record_bit_depth": 24,
					"pcm_format":       "int24",
				},
			},
		}, nil
	case "project.audio_analysis_status":
		result := map[string]any{}
		if len(f.audioAnalysisStatusResults) > 0 {
			result = cloneMap(f.audioAnalysisStatusResults[0])
			f.audioAnalysisStatusResults = f.audioAnalysisStatusResults[1:]
		}
		if len(result) == 0 {
			result = map[string]any{
				"status":                 "ok",
				"command":                "project.audio_analysis_status",
				"analysis_job_id":        "audio_analysis_test_1",
				"job_id":                 "audio_analysis_test_1",
				"analysis_queue_status":  "submitted",
				"total_clips":            3,
				"submitted_clips":        3,
				"total_feature_jobs":     9,
				"submitted_feature_jobs": 9,
				"tim_projection":         testReadyImportTIMProjection(),
			}
		}
		return executorpkg.Result{
			ToolCallID:  in.ToolCall.ID,
			Tool:        in.ToolCall.Tool,
			CommandName: "project.audio_analysis_status",
			Status:      "ok",
			Result:      result,
		}, nil
	case "project.audio_analysis_start":
		return executorpkg.Result{
			ToolCallID:  in.ToolCall.ID,
			Tool:        in.ToolCall.Tool,
			CommandName: "project.audio_analysis_start",
			Status:      "ok",
			Result: map[string]any{
				"status":          "ok",
				"analysis_job_id": firstNonEmpty(firstMapText(in.ToolCall.Args, "analysis_job_id"), "audio_analysis_test_1"),
				"retry_missing":   true,
			},
		}, nil
	case "project.import_folder_as_stems":
		if !in.Confirmed {
			preview := "Import 3 readable files as stems"
			decision := messageLoopMapValue(in.ToolCall.Args["sample_rate_decision"])
			if patch := messageLoopMapValue(in.ToolCall.Args["project_audio_settings_patch"]); len(patch) > 0 {
				preview += fmt.Sprintf("; project sample rate %d Hz -> %d Hz before import",
					firstPositiveMapInt(decision, "project_sample_rate_hz"),
					firstPositiveMapInt(patch, "sample_rate_hz"))
			} else if strings.EqualFold(firstMapText(decision, "status"), "mismatch") {
				preview += fmt.Sprintf("; sample-rate mismatch source %d Hz, project %d Hz; keep project rate",
					firstPositiveMapInt(decision, "source_sample_rate_hz"),
					firstPositiveMapInt(decision, "project_sample_rate_hz"))
			}
			return executorpkg.Result{
				ToolCallID:           in.ToolCall.ID,
				Tool:                 in.ToolCall.Tool,
				CommandName:          "project.import_folder_as_stems",
				Status:               "needs_confirmation",
				RequiresConfirmation: true,
				Preview:              preview,
				UndoLabel:            "Import stems folder",
				Result: map[string]any{
					"status":                "needs_confirmation",
					"requires_confirmation": true,
				},
			}, nil
		}
		projectSampleRateHz := f.currentProjectSampleRateHz()
		sourceSampleRateHz := f.currentSourceSampleRateHz()
		sampleRateMismatchCount := f.currentSampleRateMismatchCount()
		return executorpkg.Result{
			ToolCallID:  in.ToolCall.ID,
			Tool:        in.ToolCall.Tool,
			CommandName: "project.import_folder_as_stems",
			Status:      "ok",
			Result: map[string]any{
				"status": "ok",
				"summary": map[string]any{
					"discovered_audio_file_count":        3,
					"readable_file_count":                3,
					"unreadable_file_count":              0,
					"tracks_created":                     3,
					"clips_created":                      3,
					"sample_rate_mismatch_count":         sampleRateMismatchCount,
					"bit_depth_or_format_mismatch_count": 0,
					"copy_policy":                        "reference_original",
					"start_time_seconds":                 0.0,
					"edit_length_seconds":                228.57,
					"analysis_deferred":                  true,
					"analysis_jobs_queued":               3,
					"analysis_job_id":                    "audio_analysis_test_1",
					"analysis_queue_status":              "queued",
				},
				"audio_settings_snapshot": map[string]any{
					"sample_rate_hz":            projectSampleRateHz,
					"record_bit_depth":          24,
					"pcm_format":                "int24",
					"media_copy_policy":         "reference_original",
					"channel_import_policy":     "preserve_interleaved",
					"import_sample_rate_policy": "ask",
				},
				"created_track_ids": []string{"track_drums", "track_bass", "track_vocal"},
				"created_clip_ids":  []string{"clip_drums", "clip_bass", "clip_vocal"},
				"imported_tracks_preview": []map[string]any{
					{
						"track_id":          "track_drums",
						"track_name":        "drums",
						"clip_id":           "clip_drums",
						"clip_name":         "drums.wav",
						"source_file_path":  `E:\BaiduNetdiskDownload\sattelites\drums.wav`,
						"start_seconds":     0.0,
						"duration_seconds":  228.57,
						"sample_rate_hz":    sourceSampleRateHz,
						"bit_depth":         24,
						"pcm_format":        "int24",
						"channel_count":     2,
						"media_copy_policy": "reference_original",
					},
					{
						"track_id":          "track_bass",
						"track_name":        "bass",
						"clip_id":           "clip_bass",
						"clip_name":         "bass.wav",
						"source_file_path":  `E:\BaiduNetdiskDownload\sattelites\bass.wav`,
						"start_seconds":     0.0,
						"duration_seconds":  228.57,
						"sample_rate_hz":    sourceSampleRateHz,
						"bit_depth":         24,
						"pcm_format":        "int24",
						"channel_count":     2,
						"media_copy_policy": "reference_original",
					},
					{
						"track_id":          "track_vocal",
						"track_name":        "vocal",
						"clip_id":           "clip_vocal",
						"clip_name":         "vocal.wav",
						"source_file_path":  `E:\BaiduNetdiskDownload\sattelites\vocal.wav`,
						"start_seconds":     0.0,
						"duration_seconds":  228.57,
						"sample_rate_hz":    sourceSampleRateHz,
						"bit_depth":         24,
						"pcm_format":        "int24",
						"channel_count":     2,
						"media_copy_policy": "reference_original",
					},
				},
			},
			ObservedState: map[string]any{"tracks": []map[string]any{{
				"track_id": "track_drums",
				"clips":    []map[string]any{{"id": "clip_drums", "type": "audio"}},
			}, {
				"track_id": "track_bass",
				"clips":    []map[string]any{{"id": "clip_bass", "type": "audio"}},
			}, {
				"track_id": "track_vocal",
				"clips":    []map[string]any{{"id": "clip_vocal", "type": "audio"}},
			}}},
		}, nil
	case "project.markers.apply_section_markers":
		if !in.Confirmed {
			return executorpkg.Result{
				ToolCallID:           in.ToolCall.ID,
				Tool:                 in.ToolCall.Tool,
				CommandName:          "project.markers.apply_section_markers",
				Status:               "needs_confirmation",
				RequiresConfirmation: true,
				Preview:              "Write A5 section map as project markers",
				UndoLabel:            "Write section markers",
				Result: map[string]any{
					"status":                "needs_confirmation",
					"requires_confirmation": true,
				},
			}, nil
		}
		sections := messageLoopMapRows(in.ToolCall.Args["sections"])
		markers := make([]map[string]any, 0, len(sections))
		for i, section := range sections {
			name := firstNonEmpty(firstMapText(section, "name", "label"), fmt.Sprintf("Section %d", i+1))
			markers = append(markers, map[string]any{
				"marker_id":     fmt.Sprintf("marker_%02d", i+1),
				"name":          name,
				"kind":          "range",
				"start_seconds": firstPresentNumber(section, "start_seconds", "start"),
				"end_seconds":   firstPresentNumber(section, "end_seconds", "end"),
				"source":        firstNonEmpty(firstMapText(in.ToolCall.Args, "source"), "epm_a5"),
			})
		}
		return executorpkg.Result{
			ToolCallID:  in.ToolCall.ID,
			Tool:        in.ToolCall.Tool,
			CommandName: "project.markers.apply_section_markers",
			Status:      "ok",
			Result: map[string]any{
				"command":       "project.markers.apply_section_markers",
				"written_count": len(markers),
				"markers":       markers,
				"all_markers":   markers,
			},
		}, nil
	case "plugin.load_to_rack":
		return executorpkg.Result{
			ToolCallID:  in.ToolCall.ID,
			Tool:        in.ToolCall.Tool,
			CommandName: "plugin.load_to_rack",
			Status:      "ok",
			Result: map[string]any{
				"track_id":    in.ToolCall.Args["track_id"],
				"plugin_id":   "plugin_1",
				"plugin_name": "TDR Nova",
			},
			ObservedState: map[string]any{"tracks": []map[string]any{{
				"track_id": "1007",
				"rack": map[string]any{"nodes": []map[string]any{{
					"plugin_id":                       "plugin_1",
					"plugin_name":                     "TDR Nova",
					"track_id":                        "1007",
					"audio_reachable_from_rack_input": true,
					"vit_effective_in_output_path":    true,
				}}},
			}}},
		}, nil
	case "plugin.get_parameters":
		rows := make([]any, 0, 8)
		for i := 0; i < 8; i++ {
			row := map[string]any{"id": "param_verbose"}
			for j := 0; j < 40; j++ {
				row[fmt.Sprintf("field_%02d", j)] = "verbose parameter field " + strings.Repeat("x", 120)
			}
			rows = append(rows, row)
		}
		return executorpkg.Result{
			ToolCallID:  in.ToolCall.ID,
			Tool:        in.ToolCall.Tool,
			CommandName: "get_plugin_parameters",
			Status:      "ok",
			Result: map[string]any{
				"track_id":         "1007",
				"plugin_id":        "plugin_1",
				"plugin_name":      "TDR Nova",
				"parameter_count":  len(rows),
				"parameter_values": rows,
			},
		}, nil
	case "mix.read":
		return executorpkg.Result{
			ToolCallID:  in.ToolCall.ID,
			Tool:        in.ToolCall.Tool,
			CommandName: "mix_read",
			Status:      "ok",
			Result: map[string]any{
				"status": "ok",
				"entries": []map[string]any{{
					"key":   "observation.digest",
					"value": map[string]any{"peak_dbfs": -6.0, "rms_dbfs": -12.0, "headroom_db": 6.0},
				}},
			},
		}, nil
	case "mix.derive":
		return executorpkg.Result{
			ToolCallID:  in.ToolCall.ID,
			Tool:        in.ToolCall.Tool,
			CommandName: "mix_derive",
			Status:      "ok",
			Result: map[string]any{
				"status":         "ready",
				"observation_id": firstMapText(in.ToolCall.Args, "observation_id"),
				"mix_session_id": firstMapText(in.ToolCall.Args, "mix_session_id"),
				"relationship": map[string]any{
					"type":   firstMapText(in.ToolCall.Args, "type"),
					"status": "ready",
				},
			},
		}, nil
	case "mix.request_observation", "mix.observe":
		result := f.mixObservationResult
		if len(result) == 0 {
			result = map[string]any{
				"status":         "ok",
				"track_id":       firstMapText(in.ToolCall.Args, "track_id"),
				"artifact_id":    "obs_test",
				"observation_id": "obs_test",
				"mix_session_id": "mix_test",
				"summary":        "observation ready",
				"observation": map[string]any{
					"target_ref": map[string]any{"kind": "track", "id": firstMapText(in.ToolCall.Args, "track_id"), "label": "target"},
				},
			}
		}
		return executorpkg.Result{
			ToolCallID:  in.ToolCall.ID,
			Tool:        in.ToolCall.Tool,
			CommandName: "mix_request_observation",
			Status:      "ok",
			Result:      result,
		}, nil
	case "mix.propose_tick":
		return executorpkg.Result{
			ToolCallID:           in.ToolCall.ID,
			Tool:                 in.ToolCall.Tool,
			CommandName:          "mix_propose_tick",
			Status:               "ok",
			RequiresConfirmation: true,
			Result: map[string]any{
				"status":                "ok",
				"requires_confirmation": true,
				"tick_id":               "mix_tick_test",
				"track_id":              firstMapText(in.ToolCall.Args, "track_id"),
				"delta_db":              in.ToolCall.Args["delta_db"],
				"preview":               "track_gain_adjust 1007 -1 dB",
			},
		}, nil
	case "mix.apply_tick":
		return executorpkg.Result{
			ToolCallID:  in.ToolCall.ID,
			Tool:        in.ToolCall.Tool,
			CommandName: "mix_apply_tick",
			Status:      "ok",
			Result: map[string]any{
				"status":   "ok",
				"tick_id":  firstMapText(in.ToolCall.Args, "tick_id"),
				"track_id": firstMapText(in.ToolCall.Args, "track_id"),
				"after_db": -1,
			},
			ObservedState: map[string]any{"tracks": []map[string]any{{
				"track_id":  "1007",
				"volume_db": -1,
			}}},
		}, nil
	case "clip.fade.read":
		clipID := firstMapText(in.ToolCall.Args, "clip_id")
		return executorpkg.Result{
			ToolCallID:  in.ToolCall.ID,
			Tool:        in.ToolCall.Tool,
			CommandName: "clip.fade.read",
			Status:      "ok",
			Result: map[string]any{
				"status":           "ok",
				"clip_id":          clipID,
				"fade_in_seconds":  0.25,
				"fade_out_seconds": 0.50,
			},
		}, nil
	case "clip.gain.read":
		clipID := firstMapText(in.ToolCall.Args, "clip_id")
		return executorpkg.Result{
			ToolCallID:  in.ToolCall.ID,
			Tool:        in.ToolCall.Tool,
			CommandName: "clip.gain.read",
			Status:      "ok",
			Result: map[string]any{
				"status":       "ok",
				"clip_id":      clipID,
				"clip_gain_db": -2.75,
				"gain_db":      -2.75,
			},
		}, nil
	case "clip.strip_silence.suggest":
		clipID := firstNonEmpty(firstMapText(in.ToolCall.Args, "clip_id"), "1011")
		trackID := firstNonEmpty(firstMapText(in.ToolCall.Args, "track_id"), "1007")
		scope := firstNonEmpty(firstMapText(in.ToolCall.Args, "scope"), "selected_clip")
		actions := []any{
			map[string]any{
				"tool_name": "clip.strip_silence.apply",
				"args": map[string]any{
					"clip_id":     clipID,
					"track_id":    trackID,
					"analysis_id": "analysis_test",
					"strip_regions": []any{map[string]any{
						"clip_id":       clipID,
						"track_id":      trackID,
						"start_seconds": 0.0,
						"end_seconds":   0.35,
					}},
				},
			},
		}
		if scope == "selected_track" || scope == "all_project" {
			actions = append(actions, map[string]any{
				"tool_name": "clip.strip_silence.apply",
				"args": map[string]any{
					"clip_id":     "clip_b",
					"track_id":    "1010",
					"analysis_id": "analysis_test_b",
					"strip_regions": []any{map[string]any{
						"clip_id":       "clip_b",
						"track_id":      "1010",
						"start_seconds": 1.0,
						"end_seconds":   1.25,
					}},
				},
			})
		}
		result := map[string]any{
			"status":               "ok",
			"scope":                scope,
			"confidence":           "high",
			"pending_action_count": len(actions),
			"recommended_params": map[string]any{
				"threshold_dbfs":    -48.0,
				"min_silence_ms":    120.0,
				"clip_start_pad_ms": 15.0,
				"clip_end_pad_ms":   50.0,
			},
			"analysis": map[string]any{
				"strip_region_count": len(actions),
				"strip_seconds":      0.35,
				"analyzed_seconds":   2.0,
			},
			"risks":           []any{"preview before applying"},
			"pending_actions": actions,
		}
		if len(actions) == 1 {
			result["pending_action"] = actions[0]
		}
		return executorpkg.Result{
			ToolCallID:           in.ToolCall.ID,
			Tool:                 in.ToolCall.Tool,
			CommandName:          "clip.strip_silence.suggest",
			Status:               "ok",
			RequiresConfirmation: true,
			Result:               result,
		}, nil
	case "clip.strip_silence.apply":
		regions := messageLoopMapRows(in.ToolCall.Args["strip_regions"])
		return executorpkg.Result{
			ToolCallID:  in.ToolCall.ID,
			Tool:        in.ToolCall.Tool,
			CommandName: "clip.strip_silence.apply",
			Status:      "ok",
			Result: map[string]any{
				"status":               "ok",
				"clip_id":              firstMapText(in.ToolCall.Args, "clip_id"),
				"track_id":             firstMapText(in.ToolCall.Args, "track_id"),
				"deleted_region_count": len(regions),
			},
			ObservedState: map[string]any{"tracks": []map[string]any{{
				"track_id": firstNonEmpty(firstMapText(in.ToolCall.Args, "track_id"), "1007"),
				"clips": []map[string]any{{
					"id":   firstNonEmpty(firstMapText(in.ToolCall.Args, "clip_id"), "1011"),
					"type": "audio",
				}},
			}}},
		}, nil
	case "clip.strip_silence.apply_batch":
		actions := messageLoopMapRows(in.ToolCall.Args["pending_actions"])
		regionCount := 0
		for _, action := range actions {
			args := messageLoopMapValue(action["args"])
			if len(args) == 0 {
				args = action
			}
			regionCount += len(messageLoopMapRows(args["strip_regions"]))
		}
		return executorpkg.Result{
			ToolCallID:  in.ToolCall.ID,
			Tool:        in.ToolCall.Tool,
			CommandName: "clip.strip_silence.apply_batch",
			Status:      "ok",
			Result: map[string]any{
				"status":               "ok",
				"applied_clip_count":   len(actions),
				"applied_region_count": regionCount,
				"pending_action_count": len(actions),
			},
			ObservedState: map[string]any{"tracks": []map[string]any{{
				"track_id": "1007",
				"clips": []map[string]any{{
					"id":   "1011",
					"type": "audio",
				}},
			}}},
		}, nil
	case "clip.fade.set":
		clipID := firstMapText(in.ToolCall.Args, "clip_id")
		fadeIn, _ := firstNumericMapValue(in.ToolCall.Args, "fade_in_seconds", "fade_in", "fadeInSeconds")
		fadeOut, _ := firstNumericMapValue(in.ToolCall.Args, "fade_out_seconds", "fade_out", "fadeOutSeconds")
		if !in.Confirmed {
			return executorpkg.Result{
				ToolCallID:           in.ToolCall.ID,
				Tool:                 in.ToolCall.Tool,
				CommandName:          "clip.fade.set",
				Status:               "needs_confirmation",
				RequiresConfirmation: true,
				Preview:              fmt.Sprintf("Set clip %s fade in %.3fs fade out %.3fs", clipID, fadeIn, fadeOut),
				UndoLabel:            "Set clip fade",
				Result: map[string]any{
					"status":                "needs_confirmation",
					"clip_id":               clipID,
					"fade_in_seconds":       fadeIn,
					"fade_out_seconds":      fadeOut,
					"requires_confirmation": true,
				},
			}, nil
		}
		return executorpkg.Result{
			ToolCallID:  in.ToolCall.ID,
			Tool:        in.ToolCall.Tool,
			CommandName: "clip.fade.set",
			Status:      "ok",
			Result: map[string]any{
				"status":           "ok",
				"clip_id":          clipID,
				"fade_in_seconds":  fadeIn,
				"fade_out_seconds": fadeOut,
			},
		}, nil
	case "clip.gain.set":
		clipID := firstMapText(in.ToolCall.Args, "clip_id")
		gainDB, _ := firstNumericMapValue(in.ToolCall.Args, "gain_db", "clip_gain_db", "db")
		if !in.Confirmed {
			return executorpkg.Result{
				ToolCallID:           in.ToolCall.ID,
				Tool:                 in.ToolCall.Tool,
				CommandName:          "clip.gain.set",
				Status:               "needs_confirmation",
				RequiresConfirmation: true,
				Preview:              fmt.Sprintf("Set clip %s gain to %+.2f dB", clipID, gainDB),
				UndoLabel:            "Set clip gain",
				Result: map[string]any{
					"status":                "needs_confirmation",
					"clip_id":               clipID,
					"gain_db":               gainDB,
					"requires_confirmation": true,
				},
			}, nil
		}
		f.setProjectStateClipGain(clipID, gainDB)
		return executorpkg.Result{
			ToolCallID:  in.ToolCall.ID,
			Tool:        in.ToolCall.Tool,
			CommandName: "clip.gain.set",
			Status:      "ok",
			Result: map[string]any{
				"status":       "ok",
				"clip_id":      clipID,
				"clip_gain_db": gainDB,
				"gain_db":      gainDB,
			},
		}, nil
	case "clip.gain.set_batch":
		actions := messageLoopMapRows(in.ToolCall.Args["pending_actions"])
		if !in.Confirmed {
			return executorpkg.Result{
				ToolCallID:           in.ToolCall.ID,
				Tool:                 in.ToolCall.Tool,
				CommandName:          "clip.gain.set_batch",
				Status:               "needs_confirmation",
				RequiresConfirmation: true,
				Preview:              fmt.Sprintf("Set %d clip gains", len(actions)),
				UndoLabel:            "Set clip gain batch",
				Result: map[string]any{
					"status":                "needs_confirmation",
					"pending_action_count":  len(actions),
					"requires_confirmation": true,
				},
			}, nil
		}
		resultRows := make([]map[string]any, 0, len(actions))
		for _, action := range actions {
			args := messageLoopMapValue(action["args"])
			if len(args) == 0 {
				args = action
			}
			clipID := firstMapText(args, "clip_id")
			gainDB, _ := firstNumericMapValue(args, "gain_db", "clip_gain_db", "db")
			f.setProjectStateClipGain(clipID, gainDB)
			resultRows = append(resultRows, map[string]any{
				"status":       "ok",
				"clip_id":      clipID,
				"track_id":     firstMapText(args, "track_id"),
				"clip_gain_db": gainDB,
				"gain_db":      gainDB,
			})
		}
		return executorpkg.Result{
			ToolCallID:  in.ToolCall.ID,
			Tool:        in.ToolCall.Tool,
			CommandName: "clip.gain.set_batch",
			Status:      "ok",
			Result: map[string]any{
				"status":        "ok",
				"applied_count": len(resultRows),
				"failed_count":  0,
				"actions":       resultRows,
			},
		}, nil
	case "track.group.apply_control":
		groupID := firstMapText(in.ToolCall.Args, "group_id", "id")
		trackIDs := messageLoopStringSlice(in.ToolCall.Args["track_ids"])
		if len(trackIDs) == 0 {
			trackIDs = messageLoopStringSlice(in.ToolCall.Args["member_track_ids"])
		}
		targetDB, _ := firstNumericMapValue(in.ToolCall.Args, "db", "target_db", "volume_db")
		if !in.Confirmed {
			return executorpkg.Result{
				ToolCallID:           in.ToolCall.ID,
				Tool:                 in.ToolCall.Tool,
				CommandName:          "track.group.apply_control",
				Status:               "needs_confirmation",
				RequiresConfirmation: true,
				Preview:              fmt.Sprintf("Set group %s member faders to %+.2f dB (%d tracks)", groupID, targetDB, len(trackIDs)),
				UndoLabel:            "Apply track group volume",
				Result: map[string]any{
					"status":                "needs_confirmation",
					"group_id":              groupID,
					"track_ids":             trackIDs,
					"db":                    targetDB,
					"requires_confirmation": true,
				},
			}, nil
		}
		members := make([]map[string]any, 0, len(trackIDs))
		tracks := make([]map[string]any, 0, len(trackIDs))
		for _, trackID := range trackIDs {
			f.setProjectStateTrackVolume(trackID, targetDB)
			members = append(members, map[string]any{
				"track_id":  trackID,
				"status":    "ok",
				"target_db": targetDB,
				"after_db":  targetDB,
				"verified":  true,
			})
			tracks = append(tracks, map[string]any{
				"track_id":       trackID,
				"track_name":     trackID,
				"is_audio_track": true,
				"volume_db":      targetDB,
			})
		}
		return executorpkg.Result{
			ToolCallID:  in.ToolCall.ID,
			Tool:        in.ToolCall.Tool,
			CommandName: "track.group.apply_control",
			Status:      "ok",
			Result: map[string]any{
				"status":         "ok",
				"group_id":       groupID,
				"control":        "volume",
				"mode":           "absolute",
				"db":             targetDB,
				"applied_count":  len(trackIDs),
				"verified_count": len(trackIDs),
				"members":        members,
				"track_groups": []map[string]any{{
					"group_id":         groupID,
					"name":             firstNonEmpty(firstMapText(in.ToolCall.Args, "name"), "B1 Reference Level Reset"),
					"member_track_ids": trackIDs,
				}},
				"requires_refresh": true,
			},
			ObservedState: map[string]any{
				"tracks": tracks,
				"track_groups": []map[string]any{{
					"group_id":         groupID,
					"name":             firstNonEmpty(firstMapText(in.ToolCall.Args, "name"), "B1 Reference Level Reset"),
					"member_track_ids": trackIDs,
				}},
			},
		}, nil
	case "media.index_authorized_folder":
		return executorpkg.Result{
			ToolCallID:  in.ToolCall.ID,
			Tool:        in.ToolCall.Tool,
			CommandName: "media_index_authorized_folder",
			Status:      "ok",
			Result: map[string]any{
				"status": "ok",
				"count":  2,
				"artifacts": []map[string]any{
					{"id": "art_media_intro", "kind": "video", "title": "鐎癸綀顔夌憴鍡涱暥.mp4", "status": "ready"},
					{"id": "art_media_demo", "kind": "video", "title": "濠曟梻銇氱憴鍡涱暥.mp4", "status": "ready"},
				},
			},
		}, nil
	default:
		return executorpkg.Result{ToolCallID: in.ToolCall.ID, Tool: in.ToolCall.Tool, Status: "error", Error: "unexpected tool"}, nil
	}
}

func TestMessageLoopFastCompletesSimpleTrackAdd(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"Creating track.","tool_calls":[{"id":"create_track","tool":"track.add","args":{},"reason":"Create a new track"}]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "Create a new track",
		AllowedTools: []string{"track.add"},
	})

	if res.Status != "completed" || res.Reply != "Created new track: Track 1." {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(exec.calls) != 1 {
		t.Fatalf("executor calls = %+v", exec.calls)
	}
	if len(client.calls) != 1 {
		t.Fatalf("LLM calls = %d, want 1", len(client.calls))
	}
}

func TestMessageLoopDoesNotFastCompleteTrackAddWhenRequestHasMoreWork(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"Creating track first.","tool_calls":[{"id":"create_track","tool":"track.add","args":{},"reason":"Create a new track"}]}`,
		`{"final":true,"reply":"Track created; plugin still needs handling."}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "Create a new track, then load EQ",
		AllowedTools: []string{"track.add"},
	})

	if res.Status != "completed" || res.Reply != "Track created; plugin still needs handling." {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(client.calls) != 2 {
		t.Fatalf("LLM calls = %d, want 2", len(client.calls))
	}
}

func TestMessageLoopFastCompletesSimpleMidiClipCreate(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"Creating MIDI clip.","tool_calls":[{"id":"create_clip","tool":"midi.create_clip","args":{"track_id":"1007"},"reason":"Create MIDI clip"}]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "\u65b0\u5efa\u4e00\u4e2a MIDI clip",
		AllowedTools: []string{"midi.create_clip"},
	})

	if res.Status != "completed" || res.Reply != "\u5df2\u6210\u529f\u521b\u5efa MIDI \u7247\u6bb5\u3002" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(exec.calls) != 1 {
		t.Fatalf("executor calls = %+v", exec.calls)
	}
	if len(client.calls) != 1 {
		t.Fatalf("LLM calls = %d, want 1", len(client.calls))
	}
}

func TestMessageLoopDoesNotFinishMidiCompoundGoalBeforeNotesAreVerified(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"Creating MIDI clip first.","tool_calls":[{"id":"create_clip","tool":"midi.create_clip","args":{"track_id":"1007"},"reason":"Create MIDI clip"}]}`,
		`{"final":true,"reply":"MIDI clip created."}`,
		`{"final":false,"reply":"Writing notes.","tool_calls":[{"id":"write_notes","tool":"midi.apply_note_patch","args":{"clip_ref":"last_created_clip","time_unit":"beats","operations":[{"op":"insert_note","pitch":60,"start":0,"length":1},{"op":"insert_note","pitch":64,"start":1,"length":1}]},"reason":"Write 2 notes"}]}`,
		`{"final":true,"reply":"Created MIDI clip and wrote notes."}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 6, MaxToolCalls: 4, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "Create a MIDI clip and write 2 notes",
		AllowedTools: []string{"midi.create_clip", "midi.apply_note_patch"},
	})

	if res.Status != "completed" || res.Reply != "Created MIDI clip and wrote notes." {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(exec.calls) != 2 {
		t.Fatalf("executor calls = %+v", exec.calls)
	}
	if got := strings.TrimSpace(fmt.Sprint(exec.calls[1].Args["clip_id"])); got != "clip_1" {
		t.Fatalf("resolved clip_id = %q, want clip_1; call=%+v", got, exec.calls[1])
	}
	foundGate := false
	for _, event := range res.Trace {
		if event.Kind == "final_gate" && strings.Contains(event.Message, "MIDI note write") {
			foundGate = true
			break
		}
	}
	if !foundGate {
		t.Fatalf("expected final gate before note write; trace=%+v", res.Trace)
	}
}

func TestMessageLoopFinishesMidiGoalAfterReadbackConfirmsNotes(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"Creating MIDI clip first.","tool_calls":[{"id":"create_clip","tool":"midi.create_clip","args":{"track_id":"1007"},"reason":"Create MIDI clip"}]}`,
		`{"final":true,"reply":"MIDI clip created."}`,
		`{"final":false,"reply":"Writing notes.","tool_calls":[{"id":"write_notes","tool":"midi.apply_note_patch","args":{"clip_ref":"last_created_clip","time_unit":"beats","operations":[{"op":"insert_note","pitch":60,"start":0,"length":1},{"op":"insert_note","pitch":64,"start":1,"length":1}]},"reason":"Write 2 notes"}]}`,
		`{"final":true,"reply":"Notes written."}`,
		`{"final":false,"reply":"Reading notes for verification.","tool_calls":[{"id":"read_notes","tool":"midi.read_notes","args":{"clip_ref":"last_created_clip"},"reason":"Verify notes were written"}]}`,
		`{"final":true,"reply":"Created MIDI clip and wrote 2 notes."}`,
	}}
	exec := &fakeMessageExecutor{omitNoteWriteObservedNotes: true}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 8, MaxToolCalls: 5, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "Create a MIDI clip and write 2 notes",
		AllowedTools: []string{"midi.create_clip", "midi.apply_note_patch", "midi.read_notes"},
	})

	if res.Status != "completed" || res.Reply != "Created MIDI clip and wrote 2 notes." {
		t.Fatalf("result = status=%q reply=%q stop=%q error=%q", res.Status, res.Reply, res.StopReason, res.Error)
	}
	if len(exec.calls) != 3 {
		t.Fatalf("executor calls = %+v", exec.calls)
	}
	if got := strings.TrimSpace(fmt.Sprint(exec.calls[2].Args["clip_id"])); got != "clip_1" {
		t.Fatalf("readback clip_id = %q, want clip_1; call=%+v", got, exec.calls[2])
	}
	foundUnverifiedWriteGate := false
	for _, event := range res.Trace {
		if event.Kind == "final_gate" && strings.Contains(event.Message, "not been verified") {
			foundUnverifiedWriteGate = true
			break
		}
	}
	if !foundUnverifiedWriteGate {
		t.Fatalf("expected final gate before readback verification; trace=%+v", res.Trace)
	}
}

func TestMessageLoopExecutesDependentToolCallsWithoutIntermediateReplan(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"working","tool_calls":[{"id":"create_track","tool":"track.add","args":{}},{"id":"load_eq","tool":"plugin.load_to_rack","args":{"track_ref":"last_created_track","plugin_query":"TDR Nova","zone_id":"Z3"}}]}`,
		`{"final":true,"reply":"Track created and EQ loaded."}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 4, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "Create a new track and then load TDR Nova EQ",
		AllowedTools: []string{"track.add", "plugin.load_to_rack"},
	})

	if res.Status != "completed" || res.Reply != "Track created and EQ loaded." {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(exec.calls) != 2 {
		t.Fatalf("executor calls = %+v", exec.calls)
	}
	if got := strings.TrimSpace(exec.calls[1].Args["track_id"].(string)); got != "1007" {
		t.Fatalf("plugin target track_id = %q, want 1007; call=%+v", got, exec.calls[1])
	}
	if len(client.calls) != 2 {
		t.Fatalf("LLM calls = %d, want 2 after both tools finish", len(client.calls))
	}
	secondPrompt := client.calls[1]
	toolResultCount := 0
	for _, msg := range secondPrompt {
		if strings.HasPrefix(strings.TrimSpace(msg.Content), "<tool_result>") {
			toolResultCount++
		}
	}
	if toolResultCount != 2 {
		t.Fatalf("second LLM prompt should include two tool results, got %d messages=%+v", toolResultCount, secondPrompt)
	}
}

func TestMessageLoopResolvesFolderTrackBindingForDependentMove(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"working","tool_calls":[{"id":"create_folder","tool":"track.folder.create","args":{"folder_name":"鐠愭繃鏌?/ Low End"}},{"id":"move_bass","tool":"track.move_to_folder","args":{"track_id":"1152","folder_track_id":"$last_created_track"}}]}`,
		`{"final":true,"reply":"Folder created and track moved."}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 4, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "Create a folder and move the track into it",
		AllowedTools: []string{"track.folder.create", "track.move_to_folder"},
	})

	if res.Status != "completed" || res.Reply != "Folder created and track moved." {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(exec.calls) != 2 {
		t.Fatalf("executor calls = %+v", exec.calls)
	}
	if got := strings.TrimSpace(fmt.Sprint(exec.calls[1].Args["folder_track_id"])); got != "1322" {
		t.Fatalf("move folder_track_id = %q, want 1322; call=%+v", got, exec.calls[1])
	}
	if got := strings.TrimSpace(fmt.Sprint(exec.calls[1].Command["folder_track_id"])); got != "1322" {
		t.Fatalf("move command folder_track_id = %q, want 1322; call=%+v", got, exec.calls[1])
	}
}

func TestMessageLoopRequiresMediaToolForLocalMediaListing(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"閹垫儳鍩屾禍?2 娑擃亣顫嬫０鎴炴瀮娴犺绱扮€癸綀顔夌憴鍡涱暥.mp4閵嗕焦绱ㄧ粈楦款潒妫?mp4閵?}`,
		`{"final":false,"reply":"濮濓絽婀▔銊ュ斀婵帊缍嬬槐鐘虫綏","tool_calls":[{"id":"index_media","tool":"media.index_authorized_folder","args":{"asset_location":"C:\\Users\\timoz\\Desktop\\LLM md\\閸欏倽绂岄崘鍛啇","media_kinds":["video"],"limit":80},"reason":"濞夈劌鍞界拠銉ょ秴缂冾喕绗呴惃鍕潒妫版垹绀岄弶鎰礋婵帊缍嬪Ч鐘插幢閻?}]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     `Check which video files are under C:\Users\timoz\Desktop\LLM md\Contest Content`,
		AllowedTools: []string{"media.index_authorized_folder"},
	})

	if res.Status != "completed" || !strings.Contains(res.Reply, "2") || !strings.Contains(res.Reply, "cards") {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(exec.calls) != 1 || exec.calls[0].Tool != "media.index_authorized_folder" {
		t.Fatalf("executor calls = %+v", exec.calls)
	}
	if len(client.calls) != 2 {
		t.Fatalf("LLM calls = %d, want 2", len(client.calls))
	}
	foundGate := false
	for _, event := range res.Trace {
		if event.Kind == "final_gate" && strings.Contains(event.Message, "media artifact tool") {
			foundGate = true
			break
		}
	}
	if !foundGate {
		t.Fatalf("expected media final gate; trace=%+v", res.Trace)
	}
}

func TestMessageLoopToolResultHistorySummarizesLargeExecutionResult(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"reading","tool_calls":[{"id":"read_params","tool":"plugin.get_parameters","args":{"track_id":"1007","plugin_id":"plugin_1"}}]}`,
		`{"final":true,"reply":"Parameters summarized."}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 3, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "inspect plugin params",
		AllowedTools: []string{"plugin.get_parameters"},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q trace=%+v", res.Status, res.Reply, res.Error, res.Trace)
	}
	if len(client.calls) != 2 {
		t.Fatalf("LLM calls = %d, want 2", len(client.calls))
	}
	var toolResult string
	for _, msg := range client.calls[1] {
		content := strings.TrimSpace(msg.Content)
		if strings.HasPrefix(content, "<tool_result>") {
			toolResult = content
		}
	}
	if toolResult == "" {
		t.Fatalf("second prompt missing tool result: %+v", client.calls[1])
	}
	if strings.Contains(toolResult, "verbose parameter field") || strings.Contains(toolResult, `"parameter_values":[`) {
		t.Fatalf("tool result history leaked raw parameter payload: %s", toolResult)
	}
	for _, want := range []string{`"result_summary"`, `"plugin_id":"plugin_1"`, `"parameter_count":8`, `"preview_omitted"`} {
		if !strings.Contains(toolResult, want) {
			t.Fatalf("tool result history missing %s: %s", want, toolResult)
		}
	}
}

func TestMessageLoopNaturalMixRequestObservesBeforePluginLoad(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"Preparing EQ first.","tool_calls":[{"id":"load_eq","tool":"plugin.load_to_rack","args":{"track_id":"1007","plugin_query":"TDR Nova","zone_id":"Z3"},"reason":"For mixing"}]}`,
		`{"final":false,"reply":"I will observe the current audio first.","tool_calls":[{"id":"observe_mix","tool":"mix.request_observation","args":{"track_id":"1007"},"reason":"Observe before deciding"}]}`,
		`{"final":true,"reply":"Observation is complete; I will suggest next steps before loading plugins.","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 5, MaxToolCalls: 3, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "Can you help me compress this audio?",
		AllowedTools: []string{"plugin.load_to_rack", "mix.request_observation"},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q trace=%+v", res.Status, res.Reply, res.Error, res.Trace)
	}
	if len(exec.calls) == 0 || !isTestMixObservationTool(exec.calls[0].Tool) {
		t.Fatalf("executor calls = %+v, want first call to be mix observation", exec.calls)
	}
	foundGate := false
	for _, event := range res.Trace {
		if event.Kind == "final_gate" && strings.Contains(event.Message, "mix.observe") {
			foundGate = true
			break
		}
	}
	if !foundGate {
		t.Fatalf("expected observe-first gate; trace=%+v", res.Trace)
	}
}

func TestMessageLoopNaturalMixRequestPreflightsWhenModelFinalsWithoutTools(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"I checked the current track, but no acoustic data returned, so lower it 1 dB.","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{mixObservationResult: map[string]any{
		"status":         "ok",
		"mix_session_id": "mix_test",
		"observation_id": "obs_test",
		"track_id":       "1007",
		"acoustic_digest": map[string]any{
			"waveform": map[string]any{
				"status":      "ready",
				"peak_dbfs":   -0.4,
				"rms_dbfs":    -17.2,
				"headroom_db": 0.4,
				"crest_db":    16.8,
			},
		},
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 3, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "Help me mix the current track",
		AllowedTools: []string{"mix.observe", "mix.request_observation"},
		State: map[string]any{"tracks": []map[string]any{{
			"track_id": "1007",
			"clips": []map[string]any{{
				"id": "1011",
			}},
		}}},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(exec.calls) != 1 || exec.calls[0].Tool != "mix.observe" {
		t.Fatalf("executor calls = %+v, want deterministic mix.observe preflight", exec.calls)
	}
	if got := fmt.Sprint(exec.calls[0].Args["track_id"]); got != "1007" {
		t.Fatalf("track_id = %q, want 1007; args=%+v", got, exec.calls[0].Args)
	}
	if got := fmt.Sprint(exec.calls[0].Args["clip_id"]); got != "1011" {
		t.Fatalf("clip_id = %q, want 1011; args=%+v", got, exec.calls[0].Args)
	}
	if !strings.Contains(res.Reply, "continue executing") {
		t.Fatalf("reply did not ask for explicit execution confirmation: %q", res.Reply)
	}
}

func TestMessageLoopMixObservationUsableWithReadyAcousticEvidenceAndStaleReaderBlocker(t *testing.T) {
	result := map[string]any{
		"status": "ready",
		"mixboard": map[string]any{
			"status":        "ready",
			"open_blockers": []any{"audio_feature_reader_not_connected"},
			"package_status": map[string]any{
				"mix": "baseline_ready",
			},
		},
		"observation": map[string]any{
			"status": "ready",
			"global_summary": map[string]any{
				"band_energy_summary": map[string]any{
					"status": "ready",
					"bands": map[string]any{
						"bass": map[string]any{"energy_db": -12.0},
					},
				},
				"realtime_stereo_relation_summary": map[string]any{
					"status":               "ready",
					"capture_mode":         "realtime_playback",
					"correlation_estimate": 0.62,
				},
			},
		},
	}

	if !messageLoopMixObservationResultUsable(result) {
		t.Fatalf("ready acoustic evidence should make stale reader blocker non-fatal: %#v", result)
	}
}

func TestMessageLoopMixObservationStaleReaderBlockerFatalWithoutAcousticEvidence(t *testing.T) {
	result := map[string]any{
		"status": "ready",
		"mixboard": map[string]any{
			"status":        "ready",
			"open_blockers": []any{"audio_feature_reader_not_connected"},
			"package_status": map[string]any{
				"mix": "baseline_ready",
			},
		},
		"observation": map[string]any{
			"status": "ready",
		},
	}

	if messageLoopMixObservationResultUsable(result) {
		t.Fatalf("reader blocker without acoustic evidence should remain fatal: %#v", result)
	}
}

func TestMessageLoopRealtimeObservationRefreshesDespiteRecentObservation(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"Read realtime spectrum, level, and stereo data after playback; observation only, no changes.","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{mixObservationResult: map[string]any{
		"status":         "ok",
		"mix_session_id": "mix_realtime",
		"observation_id": "obs_realtime",
		"observation": map[string]any{
			"target_ref": map[string]any{"kind": "track", "id": "1007", "label": "Track 1"},
			"source_capabilities": map[string]any{
				"realtime_band_energy":     "ready",
				"realtime_stereo_relation": "ready",
			},
			"mix_package": map[string]any{
				"source_capabilities": map[string]any{
					"realtime_band_energy":     "ready",
					"realtime_stereo_relation": "ready",
				},
				"realtime_metrics": map[string]any{
					"band_energy": map[string]any{
						"status": "ready",
						"source": "live_level_meter_spectrum",
						"bands":  map[string]any{"bass": map[string]any{"unit_energy": 0.48}},
					},
					"stereo_relation": map[string]any{
						"status":               "ready",
						"source":               "live_level_meter_stereo",
						"correlation_estimate": 0.87,
					},
				},
			},
		},
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 3, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "After playback, check L2 realtime spectrum, level, and stereo image without changes",
		AllowedTools: []string{"project.state", "mix.observe", "mix.read", "mix.derive"},
		Context:      map[string]any{"selected_track_id": "1007"},
		State:        map[string]any{"selected_track_id": "1007", "tracks": []map[string]any{{"track_id": "1007", "clips": []map[string]any{{"id": "1011"}}}}},
		RecentObservation: &RecentObservation{
			Tool:        "mix.observe",
			CommandName: "mix_request_observation",
			Status:      "ok",
			Summary: map[string]any{
				"status":         "ok",
				"observation_id": "obs_previous",
				"mix_session_id": "mix_previous",
				"target_ref":     map[string]any{"kind": "track", "id": "1007", "label": "Track 1"},
			},
		},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q trace=%+v", res.Status, res.Reply, res.Error, res.Trace)
	}
	if len(exec.calls) == 0 || exec.calls[0].Tool != "mix.observe" {
		t.Fatalf("executor calls = %+v, want realtime preflight mix.observe before model reply", exec.calls)
	}
	call := exec.calls[0]
	if got := fmt.Sprint(call.Args["requested_layer"]); got != "l2_realtime" {
		t.Fatalf("requested_layer = %q args=%+v", got, call.Args)
	}
	if got := fmt.Sprint(call.Args["capture_mode"]); got != "realtime_playback" {
		t.Fatalf("capture_mode = %q args=%+v", got, call.Args)
	}
	if got := fmt.Sprint(call.Args["prefer_realtime"]); got != "true" {
		t.Fatalf("prefer_realtime = %q args=%+v", got, call.Args)
	}
	if got := fmt.Sprint(call.Args["track_id"]); got != "1007" {
		t.Fatalf("track_id = %q args=%+v", got, call.Args)
	}
	if got := fmt.Sprint(call.Args["clip_id"]); got != "1011" {
		t.Fatalf("clip_id = %q args=%+v", got, call.Args)
	}
}

func TestMessageLoopObservationFallbackCompletesWhenLLMTimeoutAfterReadOnlyObserve(t *testing.T) {
	client := &fakeMessageCompleter{errors: []error{context.DeadlineExceeded}}
	exec := &fakeMessageExecutor{mixObservationResult: map[string]any{
		"status":         "ok",
		"scope":          "full_project",
		"mix_session_id": "mix_test",
		"observation_id": "obs_test",
		"acoustic_digest": map[string]any{
			"waveform": map[string]any{
				"status":      "ready",
				"peak_dbfs":   -6.0,
				"rms_dbfs":    -9.0,
				"headroom_db": 6.0,
			},
			"source_capabilities": map[string]any{
				"waveform_envelope": "ready",
				"band_energy":       "ready",
				"stereo_relation":   "ready",
				"lufs_analysis":     "deferred",
				"masking_analysis":  "deferred",
				"reference_match":   "deferred",
			},
		},
		"observation": map[string]any{
			"status": "ready",
			"project_package": map[string]any{
				"tracks": []map[string]any{
					{"track_id": "1007", "track_name": "Track 1", "peak_dbfs": -6.0, "rms_dbfs": -9.0, "headroom_db": 6.0},
					{"track_id": "1010", "track_name": "Track 2", "peak_dbfs": 0.0, "rms_dbfs": -10.6, "headroom_db": 0.0},
				},
				"headroom_risk": []map[string]any{
					{"track_id": "1010", "track_name": "Track 2", "peak_dbfs": 0.0, "rms_dbfs": -10.6, "headroom_db": 0.0, "risk": "high"},
				},
			},
		},
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 3, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "help me inspect the overall mix",
		AllowedTools: []string{"mix.observe", "mix.read", "mix.derive"},
	})

	if res.Status != "completed" || res.StopReason != StopReasonDone {
		t.Fatalf("result = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	if res.Error != "" || res.FailureReason != "" {
		t.Fatalf("fallback should not surface LLM timeout as failure: error=%q failure=%q", res.Error, res.FailureReason)
	}
	if len(exec.calls) != 1 || exec.calls[0].Tool != "mix.observe" {
		t.Fatalf("executor calls = %+v, want only read-only mix.observe", exec.calls)
	}
	for _, want := range []string{"observation read model", "Track 2", "deferred"} {
		if !strings.Contains(res.Reply, want) {
			t.Fatalf("fallback reply missing %q:\n%s", want, res.Reply)
		}
	}
	foundFallbackTrace := false
	for _, event := range res.Trace {
		if event.Kind == "final_gate" && strings.Contains(event.Message, "materialized observation fallback") {
			foundFallbackTrace = true
			break
		}
	}
	if !foundFallbackTrace {
		t.Fatalf("fallback trace marker missing: %+v", res.Trace)
	}
}

func TestMessageLoopConfirmedImportFallbackCompletesWhenFinalLLM502(t *testing.T) {
	client := &fakeMessageCompleter{errors: []error{fmt.Errorf("LLM HTTP error 502: error code: 502")}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 3, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}

	pending := planner.ToolCall{
		ID:   "import_audio",
		Tool: "clip.import_media_to_track",
		Args: map[string]any{
			"track_id":   "1007",
			"file_path":  `E:\BaiduNetdiskDownload\sattelites\lead_vocal.wav`,
			"media_type": "audio",
			"start_time": 0,
			"mode":       "non_destructive",
		},
	}
	res := loop.ResumeAfterConfirmation(context.Background(), Continuation{
		GoalID:          "goal_import",
		RunID:           "run_import",
		UserText:        "confirm import audio",
		Summary:         "confirmed single audio import",
		AllowedTools:    []string{"clip.import_media_to_track"},
		Budget:          Budget{MaxTurns: 3, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
		PendingToolCall: &pending,
	})

	if res.Status != "completed" || res.StopReason != StopReasonDone {
		t.Fatalf("result = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	if res.Error != "" || res.FailureReason != "" {
		t.Fatalf("fallback should not surface final LLM 502 as failure: error=%q failure=%q", res.Error, res.FailureReason)
	}
	if len(exec.calls) != 1 || exec.calls[0].Tool != "clip.import_media_to_track" {
		t.Fatalf("executor calls = %+v", exec.calls)
	}
	for _, want := range []string{"lead_vocal.wav", "Lead Vocal", "\u5df2\u6210\u529f\u5c06\u97f3\u9891\u7247\u6bb5"} {
		if !strings.Contains(res.Reply, want) {
			t.Fatalf("fallback reply missing %q:\n%s", want, res.Reply)
		}
	}
}

func TestResolveMessageLoopBindingsKeepsTypedMixApplyTickCommandEmpty(t *testing.T) {
	state := &runState{
		executionMemory: ExecutionMemory{
			LastMixTickID:           "mix_tick_test",
			ActiveWorkTargetTrackID: "1192",
		},
	}
	call := planner.ToolCall{
		ID:   "apply_pending_mix_tick",
		Tool: "mix.apply_tick",
		Args: map[string]any{"track_id": "1192"},
	}

	resolved := resolveMessageLoopBindings(state, call)

	if got := firstMapText(resolved.Args, "tick_id"); got != "mix_tick_test" {
		t.Fatalf("resolved tick_id = %q, want mix_tick_test; call=%+v", got, resolved)
	}
	if got := firstMapText(resolved.Args, "track_id"); got != "1192" {
		t.Fatalf("resolved track_id = %q, want 1192; call=%+v", got, resolved)
	}
	if got := firstMapText(resolved.Command, "tool"); got != "mix.apply_tick" {
		t.Fatalf("typed tool binding command identity = %q, want mix.apply_tick; command=%+v", got, resolved.Command)
	}
	if got := firstMapText(resolved.Command, "tick_id"); got != "mix_tick_test" {
		t.Fatalf("resolved command tick_id = %q, want mix_tick_test; command=%+v", got, resolved.Command)
	}
	if got := firstMapText(resolved.Command, "track_id"); got != "1192" {
		t.Fatalf("resolved command track_id = %q, want 1192; command=%+v", got, resolved.Command)
	}
	if firstMapText(resolved.Command, "cmd", "action", "command") == "" && firstMapText(resolved.Command, "tool") == "" {
		t.Fatalf("typed tool binding created malformed command payload: %+v", resolved.Command)
	}
}

func TestMessageLoopStemsPreflightClarificationBecomesImportConfirmation(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"preflight first","tool_calls":[{"id":"preflight_stems","tool":"project.import_preflight","args":{"folder_path":"E:\\BaiduNetdiskDownload\\sattelites","intended_mode":"stems_folder","target_policy":"create_tracks","start_time_seconds":0},"reason":"preflight stems folder before importing"}]}`,
		`{"final":false,"needs_clarification":true,"clarification_question":"Do you want me to import these stems?","reply":"Do you want me to import these stems?","tool_calls":[]}`,
		`{"final":true,"reply":"stems imported","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 5, MaxToolCalls: 5, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "import this stems folder into the project",
		AllowedTools: []string{"project.import_preflight", "project.import_folder_as_stems", "project.audio_analysis_status"},
	})

	if res.Status != "waiting_confirmation" || res.StopReason != StopReasonNeedsConfirmation {
		t.Fatalf("result = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	if res.NeedsClarification {
		t.Fatalf("stems import fallback should not leave the turn in clarification: %+v", res)
	}
	if res.Continuation == nil || res.Continuation.PendingToolCall == nil {
		t.Fatalf("missing pending import confirmation: %+v", res.Continuation)
	}
	pending := res.Continuation.PendingToolCall
	if pending.Tool != "project.import_folder_as_stems" {
		t.Fatalf("pending tool = %q, want project.import_folder_as_stems", pending.Tool)
	}
	if got := firstMapText(pending.Args, "folder_path"); got != `E:\BaiduNetdiskDownload\sattelites` {
		t.Fatalf("pending folder_path = %q", got)
	}
	if len(exec.calls) != 2 || exec.calls[0].Tool != "project.import_preflight" || exec.calls[1].Tool != "project.import_folder_as_stems" {
		t.Fatalf("executor calls = %+v", exec.calls)
	}
	if len(exec.confirmed) != 2 || exec.confirmed[0] || exec.confirmed[1] {
		t.Fatalf("confirmed flags before user confirmation = %+v", exec.confirmed)
	}

	cont := *res.Continuation
	cont.UserText = "confirm"
	resumed := loop.ResumeAfterConfirmation(context.Background(), cont)
	if resumed.Status != "completed" || resumed.StopReason != StopReasonDone {
		t.Fatalf("resumed = status=%q stop=%q reply=%q error=%q", resumed.Status, resumed.StopReason, resumed.Reply, resumed.Error)
	}
	if len(exec.calls) != 4 || exec.calls[2].Tool != "project.import_folder_as_stems" || exec.calls[3].Tool != "project.audio_analysis_status" {
		t.Fatalf("executor calls after confirmation = %+v", exec.calls)
	}
	if len(exec.confirmed) != 4 || !exec.confirmed[2] || exec.confirmed[3] {
		t.Fatalf("confirmed flags after resume = %+v", exec.confirmed)
	}
}

func TestMessageLoopStemsImportConfirmationCompletesWithA1A2Report(t *testing.T) {
	t.Setenv("VIT_AGENT_DAD_GATE_POLL_MS", "0")
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"model should not be needed"}`,
	}}
	exec := &fakeMessageExecutor{audioAnalysisStatusResults: []map[string]any{{
		"status":                "ok",
		"command":               "project.audio_analysis_status",
		"analysis_job_id":       "audio_analysis_test_1",
		"analysis_queue_status": "submitted",
		"tim_projection":        testReadyImportTIMProjection(),
	}}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 5, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "\u628a E:\\\\BaiduNetdiskDownload\\\\sattelites \u8fd9\u4e2a\u591a\u8f68\u6587\u4ef6\u5939\u5bfc\u5165\u5de5\u7a0b",
		AllowedTools: []string{"project.import_preflight", "project.import_folder_as_stems", "project.audio_analysis_status"},
	})
	if res.Status != "waiting_confirmation" || res.Continuation == nil {
		t.Fatalf("start = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}

	cont := *res.Continuation
	cont.UserText = "confirm"
	resumed := loop.ResumeAfterConfirmation(context.Background(), cont)
	if resumed.Status != "completed" || resumed.StopReason != StopReasonDone {
		t.Fatalf("resumed = status=%q stop=%q reply=%q error=%q", resumed.Status, resumed.StopReason, resumed.Reply, resumed.Error)
	}
	if len(client.calls) != 0 {
		t.Fatalf("deterministic stems import report should not call LLM, got %d calls", len(client.calls))
	}
	for _, want := range []string{
		"project blackboard",
		"Conclusion",
		"A-F status",
		"A1",
		"A2",
		"3 / 3",
		"\u7d20\u6750\u6e05\u70b9",
		"\u7d20\u6750\u89c4\u683c",
		"\u5de5\u7a0b\u89c4\u683c",
		"\u91c7\u6837\u7387\u4e0d\u5339\u914d 0",
		"48000 Hz",
		"24-bit",
		"TIM",
		"\u6280\u672f\u5b8c\u6574\u6027",
		"\u68c0\u67e5\u8303\u56f4",
		"\u89c4\u683c\u5206\u5e03",
		"48000 Hz x3",
		"24-bit x3",
		"2ch x3",
		"A3 TOM \u667a\u80fd\u6574\u7406\u5efa\u8bae",
		"\u5efa\u8bae\u5206\u7ec4",
		"\u9f13\u7ec4 x1",
		"\u8d1d\u65af/\u4f4e\u9891 x1",
		"\u4e3b\u4eba\u58f0 x1",
		"\u5f53\u524d\u672a\u4fee\u6539\u5de5\u7a0b",
		"A4 EPM Clip \u88c1\u526a/Fade \u5efa\u8bae",
		"\u6574\u9996 Stem \u5bf9\u9f50",
		"\u9ed8\u8ba4\u4e0d\u81ea\u52a8\u88c1\u526a",
		"A5 EPM \u6bb5\u843d\u5730\u56fe\u63a8\u8350",
		"\u6bb5\u843d\u8349\u6848",
		"\u63a8\u8350\u6bb5\u843d",
		"Marker \u5199\u5165\uff1a\u53ef\u5199\u5165",
		"\u786e\u8ba4\u540e\u4f1a\u5199\u5165\u4e3a\u6bb5\u843d marker",
		"B Static Mix",
		"Static Mix",
		"B1 Gain Staging",
		"static_mix.gain_staging.v0",
		"B2 Static Balance",
		"static_mix.static_balance.v0",
		"B3 Pan Layout",
		"static_mix.pan_layout.v0",
		"B4 Low-End Relation",
		"static_mix.low_end_relation.v0",
		"B5 Focus Position",
		"static_mix.focus_position.v0",
		"not_started",
		"A-F phases are capability layers",
	} {
		if !strings.Contains(resumed.Reply, want) {
			t.Fatalf("reply missing %q:\n%s", want, resumed.Reply)
		}
	}
	for _, unwanted := range []string{
		"Source preview",
		"Project spec",
		"Analysis deferred",
		"sample-rate mismatch",
		"bit-depth/format mismatch",
		"metadata precheck",
		"reference_original",
		"audio_analysis_test_1",
		"audio_analysis_",
		"background analysis",
		"background task",
		"submitted",
		"project.apply_track_organization",
	} {
		if strings.Contains(resumed.Reply, unwanted) {
			t.Fatalf("reply should not contain old verbose label %q:\n%s", unwanted, resumed.Reply)
		}
	}
}

func TestMessageLoopProjectBlackboardStatusReadsProjectStateWithoutLLM(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"model should not be needed"}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "閻滄澘婀銉р柤濞ｇ兘鐓堕悩鑸碘偓浣光偓鑽ょ波娑撯偓娑撳绱濇潻娑樺閸掓澘鎽㈡禍鍡吹",
		AllowedTools: []string{"project.state"},
	})

	if res.Status != "completed" || res.StopReason != StopReasonDone {
		t.Fatalf("result = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	if len(client.calls) != 0 {
		t.Fatalf("project blackboard status report should not call LLM, got %d calls", len(client.calls))
	}
	if len(exec.calls) != 1 || exec.calls[0].Tool != "project.state" {
		t.Fatalf("executor calls = %+v", exec.calls)
	}
	for _, want := range []string{
		"project blackboard",
		"瀹搞儳鈻煎鍌氬枌",
		"tracks/clips",
		"Marker",
		"A-F status",
		"A Project Prep",
		"B Static Mix",
		"B1 Gain Staging",
		"static_mix.gain_staging.v0",
		"B2 Static Balance",
		"static_mix.static_balance.v0",
		"B3 Pan Layout",
		"static_mix.pan_layout.v0",
		"B4 Low-End Relation",
		"static_mix.low_end_relation.v0",
		"B5 Focus Position",
		"static_mix.focus_position.v0",
		"not_started",
		"A-F phases are capability layers",
		"data source",
		"project.state",
	} {
		if !strings.Contains(res.Reply, want) {
			t.Fatalf("reply missing %q:\n%s", want, res.Reply)
		}
	}
	if strings.Contains(res.Reply, "must complete first") {
		t.Fatalf("status report must not enforce a linear gate:\n%s", res.Reply)
	}
}

func TestMessageLoopStaticMixCapabilityContractAnswersWithoutLLM(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"model should not be needed"}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "What capabilities are in phase B? Can B1/B3/B4 be done directly?",
		AllowedTools: []string{"project.state", "mix.observe"},
	})

	if res.Status != "completed" || res.StopReason != StopReasonDone {
		t.Fatalf("result = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	if len(client.calls) != 0 {
		t.Fatalf("static mix capability contract should not call LLM, got %d calls", len(client.calls))
	}
	if len(exec.calls) != 0 {
		t.Fatalf("static mix capability contract should not call tools, got %+v", exec.calls)
	}
	for _, want := range []string{
		"B Capability Contract v0",
		"static_mix capability",
		"B1 Gain Staging",
		"static_mix.gain_staging.v0",
		"B2 Static Balance",
		"static_mix.static_balance.v0",
		"B3 Pan Layout",
		"static_mix.pan_layout.v0",
		"B4 Low-End Relation",
		"static_mix.low_end_relation.v0",
		"B5 Focus Position",
		"static_mix.focus_position.v0",
		"not_started",
		"pending_confirmation",
		"project.state",
		"mix.observe",
		"mix.apply_tick",
		"track.volume",
		"track.pan",
		"clip.gain.set",
		"A-F and B1-B5 are capability layers",
	} {
		if !strings.Contains(res.Reply, want) {
			t.Fatalf("reply missing %q:\n%s", want, res.Reply)
		}
	}
	if strings.Contains(res.Reply, "must complete first") {
		t.Fatalf("static mix capability contract must not enforce a linear gate:\n%s", res.Reply)
	}
}

func TestMessageLoopStaticMixCapabilityContractDoesNotHijackLowEndObservation(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"Low-end relation observation complete; read-only analysis, no project mutation.","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 3, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "Analyze current project low-end relation without changes",
		AllowedTools: []string{"project.state", "mix.observe", "mix.read", "mix.derive"},
	})

	if res.Status != "completed" || res.StopReason != StopReasonDone {
		t.Fatalf("result = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	if len(exec.calls) == 0 || exec.calls[0].Tool != "mix.observe" {
		t.Fatalf("low-end observation was hijacked by capability contract or another route: calls=%+v reply=%q", exec.calls, res.Reply)
	}
	if strings.Contains(res.Reply, "B Capability Contract v0") {
		t.Fatalf("low-end observation should not return contract reply:\n%s", res.Reply)
	}
}

func TestMessageLoopPendingSectionMarkersConfirmationWritesMarkers(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"model should not be needed"}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 3, MaxToolCalls: 3, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "\u786e\u8ba4\u628a A5 \u6bb5\u843d\u5730\u56fe\u5199\u5165 marker",
		AllowedTools: []string{"project.markers.apply_section_markers"},
		ExecutionMemory: ExecutionMemory{PendingSectionMarkers: map[string]any{
			"section_count":    2,
			"replace_existing": true,
			"source":           "epm_a5",
			"sections": []map[string]any{
				{"name": "Intro", "start_seconds": 0.0, "end_seconds": 30.0, "confidence": "medium"},
				{"name": "Verse", "start_seconds": 30.0, "end_seconds": 90.0, "confidence": "low"},
			},
		}},
	})

	if res.Status != "completed" || res.StopReason != StopReasonDone {
		t.Fatalf("result = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	if len(client.calls) != 0 {
		t.Fatalf("pending marker confirmation should not call LLM, got %d calls", len(client.calls))
	}
	if len(exec.calls) != 1 || exec.calls[0].Tool != "project.markers.apply_section_markers" {
		t.Fatalf("executor calls = %+v", exec.calls)
	}
	if len(exec.confirmed) != 1 || !exec.confirmed[0] {
		t.Fatalf("marker apply should execute as confirmed, flags=%+v", exec.confirmed)
	}
	sections := messageLoopMapRows(exec.calls[0].Args["sections"])
	if len(sections) != 2 || firstMapText(sections[1], "name") != "Verse" {
		t.Fatalf("marker apply did not use complete pending sections: %#v", sections)
	}
	if exec.calls[0].Args["source"] != "epm_a5" || exec.calls[0].Args["replace_existing"] != true {
		t.Fatalf("marker args mismatch: %#v", exec.calls[0].Args)
	}
	if !strings.Contains(res.Reply, "2 \u4e2a\u6bb5\u843d marker") {
		t.Fatalf("reply should summarize marker write:\n%s", res.Reply)
	}
}

func TestMessageLoopStemsImportDADGateWaitsUntilA2ReadyWithoutStartingAnalysis(t *testing.T) {
	t.Setenv("VIT_AGENT_DAD_GATE_POLL_MS", "0")
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"model should not be needed"}`,
	}}
	exec := &fakeMessageExecutor{audioAnalysisStatusResults: []map[string]any{
		{
			"status":                 "ok",
			"command":                "project.audio_analysis_status",
			"analysis_job_id":        "audio_analysis_test_1",
			"analysis_queue_status":  "submitted",
			"total_clips":            3,
			"submitted_clips":        3,
			"total_feature_jobs":     9,
			"submitted_feature_jobs": 9,
		},
		{
			"status":                "ok",
			"command":               "project.audio_analysis_status",
			"analysis_job_id":       "audio_analysis_test_1",
			"analysis_queue_status": "ready",
			"tim_projection":        testReadyImportTIMProjection(),
		},
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 5, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "\u628a E:\\\\BaiduNetdiskDownload\\\\sattelites \u8fd9\u4e2a\u591a\u8f68\u6587\u4ef6\u5939\u5bfc\u5165\u5de5\u7a0b",
		AllowedTools: []string{"project.import_preflight", "project.import_folder_as_stems", "project.audio_analysis_status"},
	})
	if res.Status != "waiting_confirmation" || res.Continuation == nil {
		t.Fatalf("start = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	cont := *res.Continuation
	cont.UserText = "confirm"
	resumed := loop.ResumeAfterConfirmation(context.Background(), cont)
	if resumed.Status != "completed" || resumed.StopReason != StopReasonDone {
		t.Fatalf("resumed = status=%q stop=%q reply=%q error=%q", resumed.Status, resumed.StopReason, resumed.Reply, resumed.Error)
	}
	if !strings.Contains(resumed.Reply, "A2 TIM \u6280\u672f\u5b8c\u6574\u6027") {
		t.Fatalf("A2 should be emitted only after DAD acoustic facts are ready:\n%s", resumed.Reply)
	}
	if strings.Contains(resumed.Reply, "\u6682\u4e0d\u8f93\u51fa") || strings.Contains(resumed.Reply, "\u7b49\u5f85\u8d85\u65f6") {
		t.Fatalf("reply must not output a waiting/timeout A2 placeholder:\n%s", resumed.Reply)
	}
	statusReads := 0
	for _, call := range exec.calls {
		switch call.Tool {
		case "project.audio_analysis_start", "project.audio_analysis_cancel":
			t.Fatalf("DAD gate must not trigger analysis control tools; calls=%+v", exec.calls)
		case "project.audio_analysis_status":
			statusReads++
		}
	}
	if statusReads < 2 {
		t.Fatalf("DAD gate should keep reading status until A2 is ready; calls=%+v", exec.calls)
	}
}

func TestMessageLoopStemsImportDADGateBuildsA2FromFeatureSnapshotPath(t *testing.T) {
	t.Setenv("VIT_AGENT_DAD_GATE_POLL_MS", "0")
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "mixboard_feature_snapshot.json")
	snapshot := `{
		"schema_version":"mixboard_feature_snapshot.v1",
		"track_waveform_envelopes":[
			{"status":"ready","track_id":"track_drums","clip_id":"clip_drums","source_revision":"rev_drums","clip_revision":"clip_rev_drums","source_path":"E:/stems/drums.wav","duration_seconds":228.57,"rms":0.2,"peak_abs":0.7},
			{"status":"ready","track_id":"track_bass","clip_id":"clip_bass","source_revision":"rev_bass","clip_revision":"clip_rev_bass","source_path":"E:/stems/bass.wav","duration_seconds":228.57,"rms":0.18,"peak_abs":0.65},
			{"status":"ready","track_id":"track_vocal","clip_id":"clip_vocal","source_revision":"rev_vocal","clip_revision":"clip_rev_vocal","source_path":"E:/stems/vocal.wav","duration_seconds":228.57,"rms":0.16,"peak_abs":0.6}
		],
		"waveform_envelope":{"status":"missing"},
		"spectrogram_tiles":{"status":"missing"},
		"band_energy_summary":{"status":"missing"},
		"stereo_relation_summary":{"status":"missing"},
		"loudness_summary":{"status":"missing"}
	}`
	if err := os.WriteFile(snapshotPath, []byte(snapshot), 0o644); err != nil {
		t.Fatal(err)
	}
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"model should not be needed"}`,
	}}
	exec := &fakeMessageExecutor{audioAnalysisStatusResults: []map[string]any{{
		"status":                 "ok",
		"command":                "project.audio_analysis_status",
		"analysis_job_id":        "audio_analysis_test_1",
		"analysis_queue_status":  "submitted",
		"total_clips":            3,
		"submitted_clips":        3,
		"total_feature_jobs":     3,
		"submitted_feature_jobs": 3,
		"feature_snapshot_path":  snapshotPath,
	}}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 5, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "\u628a E:\\\\BaiduNetdiskDownload\\\\sattelites \u8fd9\u4e2a\u591a\u8f68\u6587\u4ef6\u5939\u5bfc\u5165\u5de5\u7a0b",
		AllowedTools: []string{"project.import_preflight", "project.import_folder_as_stems", "project.audio_analysis_status"},
	})
	if res.Status != "waiting_confirmation" || res.Continuation == nil {
		t.Fatalf("start = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	cont := *res.Continuation
	cont.UserText = "confirm"
	resumed := loop.ResumeAfterConfirmation(context.Background(), cont)
	if resumed.Status != "completed" || resumed.StopReason != StopReasonDone {
		t.Fatalf("resumed = status=%q stop=%q reply=%q error=%q", resumed.Status, resumed.StopReason, resumed.Reply, resumed.Error)
	}
	if len(client.calls) != 0 {
		t.Fatalf("deterministic snapshot-backed A2 report should not call LLM, got %d calls", len(client.calls))
	}
	if !strings.Contains(resumed.Reply, "A2 TIM \u6280\u672f\u5b8c\u6574\u6027") || !strings.Contains(resumed.Reply, "3 / 3") {
		t.Fatalf("reply should include A2 built from feature snapshot:\n%s", resumed.Reply)
	}
	for _, call := range exec.calls {
		if call.Tool == "project.audio_analysis_start" || call.Tool == "project.audio_analysis_cancel" {
			t.Fatalf("DAD gate must not trigger analysis control tools; calls=%+v", exec.calls)
		}
	}
}

func TestMessageLoopTIMProjectionFromImportDADSnapshotDoesNotTreatVisibleSubsetAsComplete(t *testing.T) {
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "mixboard_feature_snapshot.json")
	refs := testImportDADRefs(61)
	visibleRows := make([]map[string]any, 0, 8)
	for i := 0; i < 8; i++ {
		visibleRows = append(visibleRows, testReadyImportDADRow(refs[i]))
	}
	testWriteImportDADSnapshot(t, snapshotPath, visibleRows)
	result := map[string]any{
		"summary": map[string]any{
			"tracks_created": 61,
			"clips_created":  61,
		},
		"imported_track_refs":   refs,
		"feature_snapshot_path": snapshotPath,
		"tim_projection": map[string]any{
			"status": "ready",
			"technical_summary": map[string]any{
				"track_count":                12,
				"clip_count":                 12,
				"acoustic_ready_track_count": 12,
			},
			"coverage": map[string]any{
				"acoustic_package": map[string]any{"status": "ready", "known_count": 12, "total_count": 12},
			},
		},
	}

	messageLoopRefreshImportTIMFromDADSnapshot(result)
	proj := messageLoopMapValue(result["tim_projection"])
	ready, readyCount, totalCount := messageLoopTIMProjectionAcousticReady(proj, 61)
	if ready {
		t.Fatalf("visible subset must not be treated as complete: ready=%t count=%d/%d proj=%+v", ready, readyCount, totalCount, proj)
	}
	if readyCount != 8 || totalCount != 61 {
		t.Fatalf("filtered TIM coverage = %d/%d, want 8/61; proj=%+v", readyCount, totalCount, proj)
	}
}

func TestMessageLoopTIMProjectionFromImportDADSnapshotUsesAllImportRefs(t *testing.T) {
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "mixboard_feature_snapshot.json")
	refs := testImportDADRefs(61)
	rows := make([]map[string]any, 0, len(refs)+3)
	rows = append(rows, map[string]any{
		"status":              "ready",
		"track_id":            "track_000",
		"clip_id":             "clip_000",
		"source_path":         "E:/stems/stem_000.wav",
		"tile_count_seen":     12,
		"tile_count_expected": 46,
		"rms":                 0.1,
		"peak_abs":            0.3,
	})
	for _, ref := range refs {
		rows = append(rows, testReadyImportDADRow(ref))
	}
	rows = append(rows, map[string]any{
		"status":      "ready",
		"track_id":    "visible_only_track",
		"clip_id":     "visible_only_clip",
		"source_path": "E:/other/visible.wav",
		"rms":         0.1,
		"peak_abs":    0.2,
	})
	testWriteImportDADSnapshot(t, snapshotPath, rows)
	result := map[string]any{
		"summary": map[string]any{
			"tracks_created": 61,
			"clips_created":  61,
		},
		"imported_track_refs":   refs,
		"feature_snapshot_path": snapshotPath,
	}

	proj := messageLoopTIMProjectionFromImportDADSnapshot(result)
	ready, readyCount, totalCount := messageLoopTIMProjectionAcousticReady(proj, 61)
	if !ready || readyCount != 61 || totalCount != 61 {
		t.Fatalf("full import DAD rows should be ready, got ready=%t count=%d/%d proj=%+v", ready, readyCount, totalCount, proj)
	}
}

func TestMessageLoopTIMProjectionFromImportDADStatusOverridesVisibleSnapshot(t *testing.T) {
	root := t.TempDir()
	snapshotPath := filepath.Join(root, "mixboard_feature_snapshot.json")
	refs := testImportDADRefs(61)
	visibleRows := make([]map[string]any, 0, 8)
	for i := 0; i < 8; i++ {
		visibleRows = append(visibleRows, testReadyImportDADRow(refs[i]))
	}
	testWriteImportDADSnapshot(t, snapshotPath, visibleRows)
	statusRows := make([]map[string]any, 0, len(refs))
	for _, ref := range refs {
		statusRows = append(statusRows, testReadyImportDADRow(ref))
	}
	result := map[string]any{
		"summary": map[string]any{
			"tracks_created": 61,
			"clips_created":  61,
		},
		"imported_track_refs":   refs,
		"feature_snapshot_path": snapshotPath,
		"analysis_job": map[string]any{
			"analysis_job_id":          "audio_analysis_status_rows",
			"dad_fact_status":          "ready",
			"dad_fact_ready_count":     61,
			"dad_fact_total_count":     61,
			"track_waveform_envelopes": statusRows,
		},
		"tim_projection": map[string]any{
			"status": "partial",
			"technical_summary": map[string]any{
				"track_count":                8,
				"clip_count":                 8,
				"acoustic_ready_track_count": 8,
			},
			"coverage": map[string]any{
				"acoustic_package": map[string]any{"status": "partial", "known_count": 8, "total_count": 61},
			},
		},
	}

	messageLoopRefreshImportTIMFromDADSnapshot(result)
	proj := messageLoopMapValue(result["tim_projection"])
	ready, readyCount, totalCount := messageLoopTIMProjectionAcousticReady(proj, 61)
	if !ready || readyCount != 61 || totalCount != 61 {
		t.Fatalf("job-scoped DAD status rows should override visible snapshot, got ready=%t count=%d/%d proj=%+v", ready, readyCount, totalCount, proj)
	}
}

func TestMessageLoopTIMProjectionFromImportDADStatusPartialRowsAreNotReady(t *testing.T) {
	refs := testImportDADRefs(61)
	statusRows := make([]map[string]any, 0, 15)
	for i := 0; i < 15; i++ {
		statusRows = append(statusRows, testReadyImportDADRow(refs[i]))
	}
	result := map[string]any{
		"summary": map[string]any{
			"tracks_created": 61,
			"clips_created":  61,
		},
		"imported_track_refs": refs,
		"analysis_job": map[string]any{
			"analysis_job_id":          "audio_analysis_status_partial_rows",
			"dad_fact_status":          "partial",
			"dad_fact_ready_count":     15,
			"dad_fact_total_count":     61,
			"dad_fact_pending_count":   46,
			"track_waveform_envelopes": statusRows,
		},
	}

	messageLoopRefreshImportTIMFromDADSnapshot(result)
	proj := messageLoopMapValue(result["tim_projection"])
	ready, readyCount, totalCount := messageLoopTIMProjectionAcousticReady(proj, 61)
	if ready {
		t.Fatalf("partial job-scoped DAD status rows must not be ready: count=%d/%d proj=%+v", readyCount, totalCount, proj)
	}
	if readyCount != 15 || totalCount != 61 {
		t.Fatalf("partial job-scoped DAD coverage = %d/%d, want 15/61; proj=%+v", readyCount, totalCount, proj)
	}
}

func testImportDADRefs(count int) []map[string]any {
	refs := make([]map[string]any, 0, count)
	for i := 0; i < count; i++ {
		refs = append(refs, map[string]any{
			"track_id":         fmt.Sprintf("track_%03d", i),
			"track_name":       fmt.Sprintf("stem %03d", i),
			"clip_id":          fmt.Sprintf("clip_%03d", i),
			"clip_name":        fmt.Sprintf("stem_%03d.wav", i),
			"source_file_path": fmt.Sprintf("E:/stems/stem_%03d.wav", i),
			"duration_seconds": 228.57,
			"sample_rate_hz":   48000,
			"bit_depth":        24,
			"pcm_format":       "int24",
			"channel_count":    2,
		})
	}
	return refs
}

func testReadyImportDADRow(ref map[string]any) map[string]any {
	return map[string]any{
		"status":              "ready",
		"track_id":            ref["track_id"],
		"clip_id":             ref["clip_id"],
		"source_path":         ref["source_file_path"],
		"duration_seconds":    228.57,
		"rms":                 0.2,
		"peak_abs":            0.7,
		"tile_count_seen":     46,
		"tile_count_expected": 46,
	}
}

func testWriteImportDADSnapshot(t *testing.T, path string, rows []map[string]any) {
	t.Helper()
	snapshot := map[string]any{
		"schema_version":           "mixboard_feature_snapshot.v1",
		"latest_request":           map[string]any{"request_id": "asset_manifest_visible_window", "status": "ready"},
		"track_waveform_envelopes": rows,
		"waveform_envelope":        map[string]any{"status": "missing"},
		"spectrogram_tiles":        map[string]any{"status": "missing"},
		"band_energy_summary":      map[string]any{"status": "missing"},
		"stereo_relation_summary":  map[string]any{"status": "missing"},
		"loudness_summary":         map[string]any{"status": "missing"},
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestMessageLoopStemsImportDADGateTimesOutWhenAutomaticDADDoesNotProgress(t *testing.T) {
	t.Setenv("VIT_AGENT_DAD_GATE_POLL_MS", "1")
	t.Setenv("VIT_AGENT_DAD_GATE_IDLE_TIMEOUT_MS", "1")
	t.Setenv("VIT_AGENT_DAD_GATE_MAX_WAIT_MS", "100")
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"model should not be needed"}`,
	}}
	stuckStatus := map[string]any{
		"status":                 "ok",
		"command":                "project.audio_analysis_status",
		"analysis_job_id":        "audio_analysis_test_1",
		"analysis_queue_status":  "queued",
		"total_clips":            3,
		"submitted_clips":        0,
		"pending_clips":          3,
		"total_feature_jobs":     3,
		"submitted_feature_jobs": 0,
		"pending_feature_jobs":   3,
		"progress":               0.0,
	}
	statuses := make([]map[string]any, 32)
	for i := range statuses {
		statuses[i] = cloneMap(stuckStatus)
	}
	exec := &fakeMessageExecutor{audioAnalysisStatusResults: statuses}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 5, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "\u628a E:\\\\BaiduNetdiskDownload\\\\sattelites \u8fd9\u4e2a\u591a\u8f68\u6587\u4ef6\u5939\u5bfc\u5165\u5de5\u7a0b",
		AllowedTools: []string{"project.import_preflight", "project.import_folder_as_stems", "project.audio_analysis_status"},
	})
	if res.Status != "waiting_confirmation" || res.Continuation == nil {
		t.Fatalf("start = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	cont := *res.Continuation
	cont.UserText = "confirm"
	resumed := loop.ResumeAfterConfirmation(context.Background(), cont)
	if resumed.Status != "failed" || resumed.StopReason != StopReasonFailed {
		t.Fatalf("resumed = status=%q stop=%q reply=%q error=%q", resumed.Status, resumed.StopReason, resumed.Reply, resumed.Error)
	}
	for _, want := range []string{"A2 TIM", "DAD", "timeout", "project.audio_analysis_status"} {
		if !strings.Contains(resumed.Error, want) {
			t.Fatalf("timeout error missing %q:\n%s", want, resumed.Error)
		}
	}
	statusReads := 0
	for _, call := range exec.calls {
		switch call.Tool {
		case "project.audio_analysis_start", "project.audio_analysis_cancel":
			t.Fatalf("DAD gate must not trigger analysis control tools; calls=%+v", exec.calls)
		case "project.audio_analysis_status":
			statusReads++
		}
	}
	if statusReads == 0 {
		t.Fatalf("DAD gate should read status before timing out; calls=%+v", exec.calls)
	}
}

func TestMessageLoopBlocksDADAnalysisControlToolsBeforeExecutor(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"start analysis","tool_calls":[{"id":"start_dad","tool":"project.audio_analysis_start","args":{"analysis_job_id":"audio_analysis_test_1"},"reason":"manual DAD start"}]}`,
		`{"final":true,"reply":"DAD analysis is automatic; I will only read status.","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 3, MaxToolCalls: 3, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "check DAD analysis",
		AllowedTools: []string{"project.audio_analysis_start", "project.audio_analysis_status"},
	})
	if res.Status != "completed" || res.StopReason != StopReasonDone {
		t.Fatalf("result = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	if len(exec.calls) != 0 {
		t.Fatalf("DAD analysis control tool reached executor: %+v", exec.calls)
	}
	foundBlock := false
	for _, event := range res.Trace {
		if event.Kind == "tool_call_blocked" && strings.Contains(event.Message, "must not start or cancel DAD analysis") {
			foundBlock = true
			break
		}
	}
	if !foundBlock {
		t.Fatalf("missing DAD analysis control block trace: %+v", res.Trace)
	}
}

func TestMessageLoopStemsImportEmptyProjectMismatchKeepsProjectRateByDefault(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"model should not be needed"}`,
	}}
	exec := &fakeMessageExecutor{
		projectSampleRateHz: 48000,
		preflightSampleRateDecision: map[string]any{
			"status":                         "mismatch",
			"project_sample_rate_hz":         48000,
			"source_sample_rate_hz":          44100,
			"source_sample_rate_is_unique":   true,
			"project_audio_clip_count":       0,
			"project_is_empty":               true,
			"can_switch_project_sample_rate": true,
			"requires_user_choice":           true,
			"recommended_action":             "ask_user_switch_or_keep_project_rate",
			"project_audio_settings_patch": map[string]any{
				"sample_rate_hz":                44100,
				"render_default_sample_rate_hz": 44100,
			},
		},
	}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 5, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "\u628a E:\\\\BaiduNetdiskDownload\\\\sattelites \u8fd9\u4e2a\u591a\u8f68\u6587\u4ef6\u5939\u5bfc\u5165\u5de5\u7a0b",
		AllowedTools: []string{"project.import_preflight", "project.import_folder_as_stems", "project.set_audio_settings", "project.audio_analysis_status"},
	})
	if res.Status != "waiting_confirmation" || res.Continuation == nil || res.Continuation.PendingToolCall == nil {
		t.Fatalf("start = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	pending := res.Continuation.PendingToolCall
	patch := messageLoopMapValue(pending.Args["project_audio_settings_patch"])
	if len(patch) > 0 {
		t.Fatalf("default empty-project mismatch should not attach automatic sample-rate patch: %+v", patch)
	}
	for _, want := range []string{"48000", "44100", "keep project rate"} {
		if !strings.Contains(res.Preview, want) {
			t.Fatalf("preview missing %q:\n%s", want, res.Preview)
		}
	}

	cont := *res.Continuation
	cont.UserText = "confirm"
	resumed := loop.ResumeAfterConfirmation(context.Background(), cont)
	if resumed.Status != "completed" || resumed.StopReason != StopReasonDone {
		t.Fatalf("resumed = status=%q stop=%q reply=%q error=%q", resumed.Status, resumed.StopReason, resumed.Reply, resumed.Error)
	}
	if len(exec.calls) != 4 {
		t.Fatalf("executor calls = %+v", exec.calls)
	}
	wantTools := []string{"project.import_preflight", "project.import_folder_as_stems", "project.import_folder_as_stems", "project.audio_analysis_status"}
	for i, want := range wantTools {
		if exec.calls[i].Tool != want {
			t.Fatalf("call[%d] = %q, want %q; calls=%+v", i, exec.calls[i].Tool, want, exec.calls)
		}
	}
	if len(exec.confirmed) != 4 || exec.confirmed[0] || exec.confirmed[1] || !exec.confirmed[2] || exec.confirmed[3] {
		t.Fatalf("confirmed flags = %+v", exec.confirmed)
	}
	for _, call := range exec.calls {
		if call.Tool == "project.set_audio_settings" {
			t.Fatalf("default mismatch should not auto-set project audio settings; calls=%+v", exec.calls)
		}
	}
	for _, want := range []string{"48000 Hz", "\u91c7\u6837\u7387\u4e0d\u5339\u914d 3"} {
		if !strings.Contains(resumed.Reply, want) {
			t.Fatalf("reply missing %q:\n%s", want, resumed.Reply)
		}
	}
	if strings.Contains(resumed.Reply, "\u5de5\u7a0b\u8bbe\u7f6e\uff1a\u5bfc\u5165\u524d\u5df2\u540c\u6b65") {
		t.Fatalf("reply should not report automatic project sample-rate switch:\n%s", resumed.Reply)
	}
}

func TestMessageLoopStemsImportExplicitSampleRateSwitchAppliesPatchBeforeImport(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"model should not be needed"}`,
	}}
	exec := &fakeMessageExecutor{
		projectSampleRateHz: 48000,
		preflightSampleRateDecision: map[string]any{
			"status":                         "mismatch",
			"project_sample_rate_hz":         48000,
			"source_sample_rate_hz":          44100,
			"source_sample_rate_is_unique":   true,
			"project_audio_clip_count":       0,
			"project_is_empty":               true,
			"can_switch_project_sample_rate": true,
			"requires_user_choice":           true,
			"recommended_action":             "ask_user_switch_or_keep_project_rate",
			"project_audio_settings_patch": map[string]any{
				"sample_rate_hz":                44100,
				"render_default_sample_rate_hz": 44100,
			},
		},
	}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 5, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "\u628a E:\\\\BaiduNetdiskDownload\\\\sattelites \u8fd9\u4e2a\u591a\u8f68\u6587\u4ef6\u5939\u5bfc\u5165\u5de5\u7a0b\uff0c\u5e76\u5207\u6362\u5de5\u7a0b\u91c7\u6837\u7387\u5339\u914d\u7d20\u6750\u91c7\u6837\u7387",
		AllowedTools: []string{"project.import_preflight", "project.import_folder_as_stems", "project.set_audio_settings", "project.audio_analysis_status"},
	})
	if res.Status != "waiting_confirmation" || res.Continuation == nil || res.Continuation.PendingToolCall == nil {
		t.Fatalf("start = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	pending := res.Continuation.PendingToolCall
	patch := messageLoopMapValue(pending.Args["project_audio_settings_patch"])
	if got := firstPositiveMapInt(patch, "sample_rate_hz"); got != 44100 {
		t.Fatalf("pending sample-rate patch = %+v, want sample_rate_hz=44100", patch)
	}
	if firstPositiveMapInt(patch, "record_bit_depth") > 0 || firstPositiveMapInt(patch, "render_default_bit_depth") > 0 {
		t.Fatalf("sample-rate patch must not change bit-depth fields: %+v", patch)
	}
	applyPatch, ok := firstMapBool(pending.Args, "apply_project_audio_settings_patch")
	if !ok || !applyPatch {
		t.Fatalf("explicit sample-rate switch should mark patch for application: %+v", pending.Args)
	}
	for _, want := range []string{"48000", "44100"} {
		if !strings.Contains(res.Preview, want) {
			t.Fatalf("preview missing %q:\n%s", want, res.Preview)
		}
	}

	cont := *res.Continuation
	cont.UserText = "confirm"
	resumed := loop.ResumeAfterConfirmation(context.Background(), cont)
	if resumed.Status != "completed" || resumed.StopReason != StopReasonDone {
		t.Fatalf("resumed = status=%q stop=%q reply=%q error=%q", resumed.Status, resumed.StopReason, resumed.Reply, resumed.Error)
	}
	if len(exec.calls) != 5 {
		t.Fatalf("executor calls = %+v", exec.calls)
	}
	wantTools := []string{"project.import_preflight", "project.import_folder_as_stems", "project.set_audio_settings", "project.import_folder_as_stems", "project.audio_analysis_status"}
	for i, want := range wantTools {
		if exec.calls[i].Tool != want {
			t.Fatalf("call[%d] = %q, want %q; calls=%+v", i, exec.calls[i].Tool, want, exec.calls)
		}
	}
	if len(exec.confirmed) != 5 || exec.confirmed[0] || exec.confirmed[1] || !exec.confirmed[2] || !exec.confirmed[3] || exec.confirmed[4] {
		t.Fatalf("confirmed flags = %+v", exec.confirmed)
	}
	settings := messageLoopMapValue(exec.calls[2].Args["audio_settings"])
	if got := firstPositiveMapInt(settings, "sample_rate_hz"); got != 44100 {
		t.Fatalf("project.set_audio_settings args = %+v, want sample_rate_hz=44100", settings)
	}
	allowSwitch, ok := firstMapBool(exec.calls[2].Args, "allow_import_sample_rate_switch")
	if !ok || !allowSwitch {
		t.Fatalf("explicit sample-rate switch should pass backend allow flag: %+v", exec.calls[2].Args)
	}
	if firstPositiveMapInt(settings, "record_bit_depth") > 0 || firstPositiveMapInt(settings, "render_default_bit_depth") > 0 {
		t.Fatalf("project.set_audio_settings sample-rate patch must not change bit-depth fields: %+v", settings)
	}
	for _, want := range []string{"44100 Hz", "\u91c7\u6837\u7387\u4e0d\u5339\u914d 0", "\u5de5\u7a0b\u8bbe\u7f6e\uff1a\u5bfc\u5165\u524d\u5df2\u540c\u6b65\u5230 44100 Hz"} {
		if !strings.Contains(resumed.Reply, want) {
			t.Fatalf("reply missing %q:\n%s", want, resumed.Reply)
		}
	}
}

func TestMessageLoopStemsImportNonEmptyMismatchKeepsProjectSampleRate(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"model should not be needed"}`,
	}}
	exec := &fakeMessageExecutor{
		projectSampleRateHz: 48000,
		preflightSampleRateDecision: map[string]any{
			"status":                         "mismatch",
			"project_sample_rate_hz":         48000,
			"source_sample_rate_hz":          44100,
			"source_sample_rate_is_unique":   true,
			"project_audio_clip_count":       2,
			"project_is_empty":               false,
			"can_switch_project_sample_rate": false,
			"requires_user_choice":           true,
			"recommended_action":             "keep_project_sample_rate_confirm_import",
		},
	}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 5, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "\u628a E:\\\\BaiduNetdiskDownload\\\\sattelites \u8fd9\u4e2a\u591a\u8f68\u6587\u4ef6\u5939\u5bfc\u5165\u5de5\u7a0b",
		AllowedTools: []string{"project.import_preflight", "project.import_folder_as_stems", "project.set_audio_settings", "project.audio_analysis_status"},
	})
	if res.Status != "waiting_confirmation" || res.Continuation == nil || res.Continuation.PendingToolCall == nil {
		t.Fatalf("start = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	if patch := messageLoopMapValue(res.Continuation.PendingToolCall.Args["project_audio_settings_patch"]); len(patch) > 0 {
		t.Fatalf("non-empty project should not attach automatic sample-rate patch: %+v", patch)
	}

	cont := *res.Continuation
	cont.UserText = "confirm"
	resumed := loop.ResumeAfterConfirmation(context.Background(), cont)
	if resumed.Status != "completed" || resumed.StopReason != StopReasonDone {
		t.Fatalf("resumed = status=%q stop=%q reply=%q error=%q", resumed.Status, resumed.StopReason, resumed.Reply, resumed.Error)
	}
	if len(exec.calls) != 4 {
		t.Fatalf("executor calls = %+v", exec.calls)
	}
	if exec.calls[2].Tool != "project.import_folder_as_stems" {
		t.Fatalf("confirmed import call = %q, want project.import_folder_as_stems", exec.calls[2].Tool)
	}
	if exec.calls[3].Tool != "project.audio_analysis_status" {
		t.Fatalf("DAD readiness call = %q, want project.audio_analysis_status", exec.calls[3].Tool)
	}
	for _, call := range exec.calls {
		if call.Tool == "project.set_audio_settings" {
			t.Fatalf("non-empty mismatch should not auto-set project audio settings; calls=%+v", exec.calls)
		}
	}
	for _, want := range []string{"48000 Hz", "\u91c7\u6837\u7387\u4e0d\u5339\u914d 3"} {
		if !strings.Contains(resumed.Reply, want) {
			t.Fatalf("reply missing %q:\n%s", want, resumed.Reply)
		}
	}
}

func TestMessageLoopChineseMultitrackFolderImportRunsDeterministicPreflight(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"model should not be needed"}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 3, MaxToolCalls: 4, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "\u628a E:\\BaiduNetdiskDownload\\yingge - sattelites tracks out \u8fd9\u4e2a\u591a\u8f68\u6587\u4ef6\u5939\u5bfc\u5165\u5de5\u7a0b",
		AllowedTools: []string{"project.import_preflight", "project.import_folder_as_stems", "project.audio_analysis_status"},
	})

	if res.Status != "waiting_confirmation" || res.StopReason != StopReasonNeedsConfirmation {
		t.Fatalf("result = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	if len(client.calls) != 0 {
		t.Fatalf("deterministic stems import should not call LLM, got %d calls", len(client.calls))
	}
	if len(exec.calls) != 2 || exec.calls[0].Tool != "project.import_preflight" || exec.calls[1].Tool != "project.import_folder_as_stems" {
		t.Fatalf("executor calls = %+v", exec.calls)
	}
	wantFolder := `E:\BaiduNetdiskDownload\yingge - sattelites tracks out`
	if got := firstMapText(exec.calls[0].Args, "folder_path"); got != wantFolder {
		t.Fatalf("preflight folder_path = %q, want %q", got, wantFolder)
	}
	if got := firstMapText(exec.calls[1].Args, "folder_path"); got != wantFolder {
		t.Fatalf("import folder_path = %q, want %q", got, wantFolder)
	}
	if len(exec.confirmed) != 2 || exec.confirmed[0] || exec.confirmed[1] {
		t.Fatalf("confirmed flags before user confirmation = %+v", exec.confirmed)
	}
}

func TestMessageLoopQuotedProjectFolderImportRunsDeterministicPreflight(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"model should not be needed"}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 3, MaxToolCalls: 4, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "\u5e2e\u6211\u5bfc\u5165\u8fd9\u4e2a\u5de5\u7a0b\"E:\\BaiduNetdiskDownload\\yingge - sattelites tracks out\"",
		AllowedTools: []string{"project.import_preflight", "project.import_folder_as_stems", "project.audio_analysis_status"},
	})

	if res.Status != "waiting_confirmation" || res.StopReason != StopReasonNeedsConfirmation {
		t.Fatalf("result = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	if len(client.calls) != 0 {
		t.Fatalf("deterministic stems import should not call LLM, got %d calls", len(client.calls))
	}
	if len(exec.calls) != 2 || exec.calls[0].Tool != "project.import_preflight" || exec.calls[1].Tool != "project.import_folder_as_stems" {
		t.Fatalf("executor calls = %+v", exec.calls)
	}
}

func TestMessageLoopSingleAudioProjectImportDoesNotRunStemsPreflight(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"importing file","tool_calls":[{"id":"import_audio","tool":"clip.import_media_to_track","args":{"track_id":"1007","file_path":"E:\\BaiduNetdiskDownload\\sample.wav","media_type":"audio","start_time":0,"mode":"non_destructive"},"reason":"single audio file import"}]}`,
		`{"final":true,"reply":"audio imported","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 3, MaxToolCalls: 4, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "\u5e2e\u6211\u5bfc\u5165\u8fd9\u4e2a\u5de5\u7a0b\"E:\\BaiduNetdiskDownload\\sample.wav\"",
		AllowedTools: []string{"project.import_preflight", "project.import_folder_as_stems", "clip.import_media_to_track"},
	})

	if res.Status != "completed" || res.StopReason != StopReasonDone {
		t.Fatalf("result = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	if len(client.calls) == 0 {
		t.Fatalf("single audio import should go through the LLM/tool context, not deterministic stems preflight")
	}
	if len(exec.calls) != 1 || exec.calls[0].Tool != "clip.import_media_to_track" {
		t.Fatalf("executor calls = %+v", exec.calls)
	}
}

func TestMessageLoopMediaFolderInspectionDoesNotRunStemsPreflight(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"index media","tool_calls":[{"id":"index_media","tool":"media.index_authorized_folder","args":{"asset_location":"E:\\BaiduNetdiskDownload\\yingge - sattelites tracks out","media_kinds":["audio"],"limit":80},"reason":"show media assets in the folder"}]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 3, MaxToolCalls: 3, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "\u770b\u770b E:\\BaiduNetdiskDownload\\yingge - sattelites tracks out \u8fd9\u4e2a\u6587\u4ef6\u5939\u91cc\u6709\u4ec0\u4e48\u97f3\u9891\u7d20\u6750",
		AllowedTools: []string{"project.import_preflight", "project.import_folder_as_stems", "media.index_authorized_folder"},
	})

	if res.Status != "completed" || res.StopReason != StopReasonDone {
		t.Fatalf("result = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	if len(exec.calls) != 1 || exec.calls[0].Tool != "media.index_authorized_folder" {
		t.Fatalf("executor calls = %+v", exec.calls)
	}
	if len(client.calls) != 1 {
		t.Fatalf("LLM calls = %d, want 1", len(client.calls))
	}
}

func TestMessageLoopMediaArtifactToolResultUsesCompactSummary(t *testing.T) {
	rows := make([]map[string]any, 0, 10)
	for i := 0; i < 10; i++ {
		rows = append(rows, map[string]any{
			"id":         fmt.Sprintf("art_%02d", i),
			"kind":       "audio",
			"title":      fmt.Sprintf("stem_%02d.wav", i),
			"path":       fmt.Sprintf(`E:\BaiduNetdiskDownload\sattelites\stem_%02d.wav`, i),
			"status":     "ready",
			"summary":    "Audio file, duration 120.00s",
			"metadata":   map[string]any{"sample_rate": 48000, "channels": 2},
			"size_bytes": 123456,
		})
	}
	record := map[string]any{
		"status":       "ok",
		"tool":         "media.index_authorized_folder",
		"command_name": "media_index_authorized_folder",
		"result": map[string]any{
			"status":    "ok",
			"count":     10,
			"locations": []string{`E:\BaiduNetdiskDownload\sattelites`},
			"artifacts": rows,
		},
	}

	compact := compactMessageLoopExecution(record)
	data, err := json.Marshal(compact)
	if err != nil {
		t.Fatalf("marshal compact execution: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "media_artifact_summary") || !strings.Contains(text, "sample_omitted_count") {
		t.Fatalf("compact summary missing expected fields: %s", text)
	}
	if strings.Contains(text, `stem_09.wav`) || strings.Contains(text, `\sattelites\stem_`) || strings.Contains(text, "metadata") {
		t.Fatalf("compact summary leaked full artifact payload: %s", text)
	}
	if _, ok := compact["result_summary"]; ok {
		t.Fatalf("media artifact execution should not include generic result_summary: %+v", compact)
	}
}

func TestMessageLoopObservationFallbackAsksClarificationForUnresolvedVocal(t *testing.T) {
	client := &fakeMessageCompleter{errors: []error{context.DeadlineExceeded}}
	exec := &fakeMessageExecutor{mixObservationResult: map[string]any{
		"status":         "ok",
		"scope":          "full_project_with_focus_track",
		"mix_session_id": "mix_focus",
		"observation_id": "obs_focus",
		"digest": map[string]any{
			"scope": "full_project_with_focus_track",
			"target": map[string]any{
				"kind": "project",
				"id":   "current",
			},
		},
		"observation": map[string]any{
			"status":     "ready",
			"target_ref": map[string]any{"kind": "project", "id": "current", "label": "Current project"},
			"project_package": map[string]any{
				"track_count":                 2,
				"active_acoustic_track_count": 2,
				"tracks": []map[string]any{{
					"track_id":         "1007",
					"name":             "Track 1",
					"track_name":       "Track 1",
					"user_label":       "Track 1",
					"user_track_index": 1,
					"role_guess":       "unknown",
					"peak_dbfs":        -6.0,
					"headroom_db":      6.0,
				}, {
					"track_id":         "1010",
					"name":             "Track 2",
					"track_name":       "Track 2",
					"user_label":       "Track 2",
					"user_track_index": 2,
					"role_guess":       "unknown",
					"peak_dbfs":        0.0,
					"headroom_db":      0.0,
				}},
			},
		},
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 3, MaxToolCalls: 3, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "make the lead vocal more forward",
		AllowedTools: []string{"mix.observe", "mix.read", "mix.derive"},
	})

	if res.Status != "waiting_clarification" || res.StopReason != StopReasonNeedsClarification || !res.NeedsClarification {
		t.Fatalf("result = status=%q stop=%q needs=%v reply=%q error=%q", res.Status, res.StopReason, res.NeedsClarification, res.Reply, res.Error)
	}
	if len(exec.calls) < 1 || exec.calls[0].Tool != "mix.observe" {
		t.Fatalf("executor calls = %+v, want mix.observe first", exec.calls)
	}
	if res.ExecutionMemory.PendingMixTickCandidate != nil || res.ExecutionMemory.PendingMixTreatment != nil {
		t.Fatalf("unresolved vocal fallback created pending action: %+v", res.ExecutionMemory)
	}
	if !strings.Contains(res.Reply, "Track") || (!strings.Contains(res.Reply, "vocal") && !strings.Contains(res.Reply, "\u4e3b\u5531")) {
		t.Fatalf("clarification reply should ask for the vocal track, got %q", res.Reply)
	}
}

func TestMessageLoopNaturalMixObservationDoesNotAskExecutionWhenDeepPackageBuilding(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"I observed the mix. L3 spectrum and stereo packages are still building.\n\nShould I execute this?","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{mixObservationResult: map[string]any{
		"status":         "ok",
		"mix_session_id": "mix_test",
		"observation_id": "obs_test",
		"track_id":       "1007",
		"acoustic_package_status": map[string]any{
			"schema_version":  "acoustic_package_status.v0",
			"status":          "partial",
			"project_id":      "current",
			"track_id":        "1007",
			"clip_id":         "1011",
			"source_revision": "rev_test",
			"package_layers": map[string]any{
				"l1_static": map[string]any{
					"status": "ready",
				},
				"l3_deep": map[string]any{
					"status": "building",
					"features": map[string]any{
						"spectrogram_tiles":       map[string]any{"status": "building"},
						"band_energy_summary":     map[string]any{"status": "building"},
						"stereo_relation_summary": map[string]any{"status": "building"},
						"lufs_analysis":           map[string]any{"status": "deferred"},
						"masking_analysis":        map[string]any{"status": "deferred"},
						"reference_match":         map[string]any{"status": "deferred"},
					},
				},
			},
		},
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 3, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "help me mix the current track",
		AllowedTools: []string{"mix.observe"},
		State: map[string]any{"tracks": []map[string]any{{
			"track_id": "1007",
			"clips": []map[string]any{{
				"id": "1011",
			}},
		}}},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if strings.Contains(strings.ToLower(res.Reply), "execute") || strings.Contains(res.Reply, "缂佈呯敾閹笛嗩攽") {
		t.Fatalf("building deep package reply should not ask to execute: %q", res.Reply)
	}
	if res.ExecutionMemory.PendingMixTickCandidate != nil || res.ExecutionMemory.PendingMixTreatment != nil {
		t.Fatalf("building deep package observation created pending action: %+v", res.ExecutionMemory)
	}
}

func TestMessageLoopNaturalMixRequestDoesNotAllowObserveAndPluginLoadInSameModelTurn(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"閸忓牐顫囩€电喎鍟€閸旂姾娴囬妴?,"tool_calls":[{"id":"observe_mix","tool":"mix.request_observation","args":{"track_id":"1007"}},{"id":"load_eq","tool":"plugin.load_to_rack","args":{"track_id":"1007","plugin_query":"TDR Nova","zone_id":"Z3"}}]}`,
		`{"final":true,"reply":"瀹歌尙绮＄憴鍌氱檪鐎瑰本鍨氶敍灞肩瑓娑撯偓濮濄儲鍨滄导姘帥鐠囧瓨妲戝楦款唴閵?,"tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 3, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "鐢喗鍨滅紓鈺傝穿瑜版挸澧犳潪銊╀壕",
		AllowedTools: []string{"plugin.load_to_rack", "mix.request_observation"},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(exec.calls) != 1 || !isTestMixObservationTool(exec.calls[0].Tool) {
		t.Fatalf("executor calls = %+v, want same-turn plugin load blocked", exec.calls)
	}
}

func TestMessageLoopPanRequestBlocksPrimitivePanUntilConfirmation(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"I will move the pan now.","tool_calls":[{"id":"pan_track","tool":"track.pan","args":{"track_id":"1007","pan":-0.1},"reason":"move Track 2 left"}]}`,
		`{"final":true,"reply":"I observed Track 2 and can move it left a little after confirmation.\nmix_treatment_pending: {\"schema_version\":\"mix_treatment_pending.v0\",\"status\":\"pending_confirmation\",\"intent\":\"move Track 2 left a little\",\"target_ref\":\"track:1007\",\"action_kind\":\"pan_balance\",\"processor_type\":\"utility\",\"delta_pan\":-0.1,\"target\":{\"delta_pan\":-0.1},\"reasoning_summary\":\"single explicit pan step after observation\",\"confidence\":\"high\",\"evidence_refs\":[\"observation.digest\"],\"needs_resolution\":[],\"expires_after_context_change\":true}","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 3, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "Track 2 pan left a little",
		AllowedTools: []string{"mix.observe", "mix.request_observation", "track.pan"},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(exec.calls) != 1 || exec.calls[0].Tool != "mix.observe" {
		t.Fatalf("executor calls = %+v, want only deterministic mix.observe", exec.calls)
	}
	if got := fmt.Sprint(exec.calls[0].Args["track_id"]); got != "Track 2" {
		t.Fatalf("track_id = %q, want Track 2; args=%+v", got, exec.calls[0].Args)
	}
	if got := fmt.Sprint(exec.calls[0].Args["user_track_index"]); got != "2" {
		t.Fatalf("user_track_index = %q, want 2; args=%+v", got, exec.calls[0].Args)
	}
	treatment := res.ExecutionMemory.PendingMixTreatment
	if treatment == nil || treatment.ActionKind != "pan_balance" || treatment.DeltaPan != -0.1 {
		t.Fatalf("pending treatment = %+v", treatment)
	}
	if strings.Contains(res.Reply, "mix_treatment_pending") {
		t.Fatalf("reply leaked treatment marker: %q", res.Reply)
	}
}

func TestMessageLoopReadOnlyObservationDoesNotCreatePendingCandidate(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"Track 2 is loud; lower Track 2 by 1 dB. Should I lower Track 2 by 1 dB?","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 3, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "read-only acoustic observation: analyze current mix stereo status and loudness; do not modify.",
		AllowedTools: []string{"mix.observe", "mix.read", "mix.derive", "mix.request_observation", "mix.propose_tick", "mix.apply_tick"},
		State: map[string]any{"tracks": []map[string]any{{
			"track_id":         "1007",
			"track_name":       "Track 1",
			"user_track_index": 1,
		}, {
			"track_id":         "1010",
			"track_name":       "Track 2",
			"user_track_index": 2,
		}}},
		ExecutionMemory: ExecutionMemory{
			PendingMixTickCandidate: &PendingMixTickCandidate{Status: "pending_confirmation", TrackID: "old"},
			PendingMixTreatment:     &MixTreatmentPending{Status: "pending_confirmation", TargetRef: "track:old"},
		},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(exec.calls) != 1 || exec.calls[0].Tool != "mix.observe" {
		t.Fatalf("executor calls = %+v, want only mix.observe", exec.calls)
	}
	if exec.calls[0].Args["mutation_barrier"] != true || exec.calls[0].Args["no_pending"] != true || exec.calls[0].Args["workflow_intent"] != "observation_only" {
		t.Fatalf("read-only observation args missing barrier metadata: %+v", exec.calls[0].Args)
	}
	if res.ExecutionMemory.PendingMixTickCandidate != nil || res.ExecutionMemory.PendingMixTreatment != nil {
		t.Fatalf("read-only turn kept pending memory: %+v", res.ExecutionMemory)
	}
	reply := strings.ToLower(res.Reply)
	if strings.Contains(reply, "should i") || strings.Contains(reply, "mix_treatment_pending") {
		t.Fatalf("read-only reply leaked confirmation language or marker: %q", res.Reply)
	}
}

func TestMessageLoopFrequencyStereoReadOnlyObservationAddsProjection(t *testing.T) {
	args := messageLoopMixObservationArgs("Observe current project frequency and stereo state without making changes", nil)
	if args["projection"] != "frequency_stereo" || args["include_raw"] != false || args["target_scope"] == "" {
		t.Fatalf("projection args missing: %+v", args)
	}
	if args["mutation_barrier"] != true || args["no_pending"] != true || args["workflow_intent"] != "observation_only" {
		t.Fatalf("read-only guard args missing: %+v", args)
	}
	keys := messageLoopStringList(args["feature_keys"])
	keySet := map[string]bool{}
	for _, key := range keys {
		keySet[key] = true
	}
	for _, want := range []string{"band_energy_summary", "stereo_relation_summary", "spectrogram_tiles", "acoustic_package_status", "source_identity"} {
		if !keySet[want] {
			t.Fatalf("feature_keys missing %s: %+v", want, args)
		}
	}
}

func TestMessageLoopClipFadeGainReadDoesNotRouteToMixGain(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"Reading current clip fade/gain state.","tool_calls":[{"id":"read_fade","tool":"clip.fade.read","args":{"clip_id":"1011"},"reason":"read clip fade"},{"id":"read_gain","tool":"clip.gain.read","args":{"clip_id":"1011"},"reason":"read clip gain"}]}`,
		`{"final":true,"reply":"Current clip fade in 0.25s, fade out 0.50s, clip gain -2.75 dB.","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{
		mixObservationResult: map[string]any{
			"status":         "ok",
			"track_id":       "1007",
			"observation_id": "stale_obs",
			"mix_session_id": "stale_mix",
			"observation": map[string]any{
				"target_ref": map[string]any{"kind": "track", "id": "1007", "label": "Track 1"},
			},
		},
	}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 5, MaxToolCalls: 4, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "Read current selected clip fade and gain state",
		AllowedTools: []string{"clip.fade.read", "clip.gain.read", "mix.observe", "mix.propose_tick", "mix.apply_tick"},
		Context: map[string]any{
			"selected_clip_id":  "1011",
			"selected_track_id": "1007",
		},
		State: map[string]any{"tracks": []map[string]any{{
			"track_id":   "1007",
			"track_name": "Track 1",
			"clips": []map[string]any{{
				"clip_id":          "1011",
				"fade_in_seconds":  0.25,
				"fade_out_seconds": 0.50,
				"clip_gain_db":     -2.75,
			}},
		}}},
		ExecutionMemory: ExecutionMemory{
			ActiveWorkTargetTrackID: "1007",
			ActiveWorkTargetClipID:  "1011",
		},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	var tools []string
	for _, call := range exec.calls {
		tools = append(tools, call.Tool)
		switch call.Tool {
		case "mix.observe", "mix.propose_tick", "mix.apply_tick":
			t.Fatalf("clip fade/gain read routed to mix tool: calls=%+v", exec.calls)
		}
	}
	if !messageLoopTestStringSliceContains(tools, "clip.fade.read") || !messageLoopTestStringSliceContains(tools, "clip.gain.read") {
		t.Fatalf("executor calls = %+v, want clip.fade.read and clip.gain.read", exec.calls)
	}
	if res.ExecutionMemory.PendingMixTickCandidate != nil || res.ExecutionMemory.PendingMixTreatment != nil {
		t.Fatalf("clip fade/gain read created pending mix memory: %+v", res.ExecutionMemory)
	}
}

func TestMessageLoopClipFadeGainReadPreflightBypassesMixObservation(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"bad route","tool_calls":[{"id":"observe_mix","tool":"mix.observe","args":{"scope":"selected_clip"},"reason":"wrong route"}]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 5, MaxToolCalls: 4, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "clip fade gain",
		AllowedTools: []string{"clip.fade.read", "clip.gain.read", "mix.observe"},
		Context: map[string]any{
			"current_selection": map[string]any{
				"selected_clip_id":  "1011",
				"selected_track_id": "1007",
			},
		},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(client.calls) != 0 {
		t.Fatalf("LLM should not be called for deterministic clip fade/gain read, calls=%d", len(client.calls))
	}
	var tools []string
	for _, call := range exec.calls {
		tools = append(tools, call.Tool)
	}
	if len(tools) != 2 || tools[0] != "clip.fade.read" || tools[1] != "clip.gain.read" {
		t.Fatalf("executor calls = %+v, want deterministic clip fade/gain reads", exec.calls)
	}
	if strings.Contains(strings.Join(tools, ","), "mix.observe") {
		t.Fatalf("clip fade/gain preflight routed to mix: %+v", tools)
	}
	if !strings.Contains(res.Reply, "-2.75") || !strings.Contains(res.Reply, "0.250") {
		t.Fatalf("reply should summarize clip read result, got %q", res.Reply)
	}
}

func TestMessageLoopStripSilenceSuggestPreflightBypassesMixObservation(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"bad route","tool_calls":[{"id":"observe_mix","tool":"mix.observe","args":{"scope":"selected_clip"},"reason":"wrong route"}]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 5, MaxToolCalls: 4, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "Analyze selected range strip silence cleanup parameters and estimate noise threshold",
		AllowedTools: []string{"clip.strip_silence.suggest", "clip.strip_silence.analyze", "clip.strip_silence.apply", "clip.strip_silence.apply_batch", "mix.observe"},
		Context: map[string]any{
			"current_selection": map[string]any{
				"selected_clip_id":       "1011",
				"selected_clip_track_id": "1007",
				"selected_clip_ranges": []any{
					map[string]any{
						"range_id":         "range_1",
						"clip_id":          "1011",
						"track_id":         "1007",
						"start_seconds":    1.0,
						"end_seconds":      2.0,
						"duration_seconds": 1.0,
					},
				},
			},
		},
	})

	if res.Status != "waiting_confirmation" || res.StopReason != StopReasonNeedsConfirmation {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(client.calls) != 0 {
		t.Fatalf("LLM should not be called for deterministic strip silence suggest, calls=%d", len(client.calls))
	}
	if len(exec.calls) != 1 || exec.calls[0].Tool != "clip.strip_silence.suggest" {
		t.Fatalf("executor calls = %+v, want clip.strip_silence.suggest only", exec.calls)
	}
	if firstMapText(exec.calls[0].Args, "scope") != "selected_ranges" {
		t.Fatalf("suggest args = %+v", exec.calls[0].Args)
	}
	if ranges := messageLoopMapRows(exec.calls[0].Args["selected_clip_ranges"]); len(ranges) != 1 || firstMapText(ranges[0], "range_id") != "range_1" {
		t.Fatalf("selected ranges not forwarded: %+v", exec.calls[0].Args)
	}
	if strings.Contains(res.Reply, "mix") || !strings.Contains(res.Reply, "strip silence") || !strings.Contains(res.Reply, "-48.0 dBFS") {
		if res.Continuation == nil || res.Continuation.PendingToolCall == nil || res.Continuation.PendingToolCall.Tool != "clip.strip_silence.apply" {
			t.Fatalf("pending continuation = %+v", res.Continuation)
		}
		pending := res.Continuation.PendingToolCall
		if firstMapText(pending.Args, "clip_id") != "1011" || len(messageLoopMapRows(pending.Args["strip_regions"])) != 1 {
			t.Fatalf("pending apply args = %+v", pending.Args)
		}
	}
}

func TestMessageLoopStripSilenceSuggestPreflightPrefersClipWhenRangeNotRequested(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"bad route","tool_calls":[{"id":"observe_mix","tool":"mix.observe","args":{"scope":"selected_clip"},"reason":"wrong route"}]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 5, MaxToolCalls: 4, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "suggest strip silence cleanup parameters for the current selected audio clip and estimate threshold",
		AllowedTools: []string{"clip.strip_silence.suggest", "clip.strip_silence.analyze", "clip.strip_silence.apply", "clip.strip_silence.apply_batch", "mix.observe"},
		Context: map[string]any{
			"current_selection": map[string]any{
				"selected_clip_id":       "1011",
				"selected_clip_track_id": "1007",
				"selected_clip_ranges": []any{
					map[string]any{
						"range_id":         "stale_range",
						"clip_id":          "old_clip",
						"track_id":         "old_track",
						"start_seconds":    10.0,
						"end_seconds":      11.0,
						"duration_seconds": 1.0,
					},
				},
			},
		},
	})

	if res.Status != "waiting_confirmation" || res.StopReason != StopReasonNeedsConfirmation {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(client.calls) != 0 {
		t.Fatalf("LLM should not be called for deterministic strip silence suggest, calls=%d", len(client.calls))
	}
	if len(exec.calls) != 1 || exec.calls[0].Tool != "clip.strip_silence.suggest" {
		t.Fatalf("executor calls = %+v, want clip.strip_silence.suggest only", exec.calls)
	}
	if firstMapText(exec.calls[0].Args, "scope") != "selected_clip" {
		t.Fatalf("suggest args = %+v, want selected_clip despite stale ranges", exec.calls[0].Args)
	}
	if _, ok := exec.calls[0].Args["selected_clip_ranges"]; ok {
		t.Fatalf("stale selected ranges should not be forwarded for clip request: %+v", exec.calls[0].Args)
	}
	if firstMapText(exec.calls[0].Args, "clip_id") != "1011" || firstMapText(exec.calls[0].Args, "track_id") != "1007" {
		t.Fatalf("selected clip not forwarded: %+v", exec.calls[0].Args)
	}
	if res.Continuation == nil || res.Continuation.PendingToolCall == nil || res.Continuation.PendingToolCall.Tool != "clip.strip_silence.apply" {
		t.Fatalf("pending continuation = %+v", res.Continuation)
	}
	pending := res.Continuation.PendingToolCall
	if firstMapText(pending.Args, "clip_id") != "1011" || firstMapText(pending.Args, "track_id") != "1007" || len(messageLoopMapRows(pending.Args["strip_regions"])) != 1 {
		t.Fatalf("pending apply args = %+v", pending.Args)
	}
}

func TestMessageLoopStripSilenceSuggestPreflightRoutesSelectedTrackScope(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"bad route","tool_calls":[{"id":"observe_mix","tool":"mix.observe","args":{"scope":"selected_track"},"reason":"wrong route"}]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 5, MaxToolCalls: 4, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "suggest strip silence cleanup parameters for the current selected track",
		AllowedTools: []string{"clip.strip_silence.suggest", "clip.strip_silence.analyze", "clip.strip_silence.apply", "clip.strip_silence.apply_batch", "mix.observe"},
		Context: map[string]any{
			"current_selection": map[string]any{
				"selected_track_id": "1010",
				"selected_clip_ranges": []any{
					map[string]any{
						"range_id":         "stale_range",
						"clip_id":          "old_clip",
						"track_id":         "old_track",
						"start_seconds":    10.0,
						"end_seconds":      11.0,
						"duration_seconds": 1.0,
					},
				},
			},
		},
	})

	if res.Status != "waiting_confirmation" || res.StopReason != StopReasonNeedsConfirmation {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(client.calls) != 0 {
		t.Fatalf("LLM should not be called for deterministic strip silence selected-track suggest, calls=%d", len(client.calls))
	}
	if len(exec.calls) != 1 || exec.calls[0].Tool != "clip.strip_silence.suggest" {
		t.Fatalf("executor calls = %+v, want clip.strip_silence.suggest only", exec.calls)
	}
	if firstMapText(exec.calls[0].Args, "scope") != "selected_track" || firstMapText(exec.calls[0].Args, "track_id") != "1010" {
		t.Fatalf("suggest args = %+v, want selected_track on 1010", exec.calls[0].Args)
	}
	if _, ok := exec.calls[0].Args["clip_id"]; ok {
		t.Fatalf("selected track request should not be narrowed to selected clip: %+v", exec.calls[0].Args)
	}
	if _, ok := exec.calls[0].Args["selected_clip_ranges"]; ok {
		t.Fatalf("selected track request should ignore stale selected ranges: %+v", exec.calls[0].Args)
	}
	if res.Continuation == nil || res.Continuation.PendingToolCall == nil || res.Continuation.PendingToolCall.Tool != "clip.strip_silence.apply_batch" {
		t.Fatalf("pending continuation = %+v", res.Continuation)
	}
	if rows := messageLoopMapRows(res.Continuation.PendingToolCall.Args["pending_actions"]); len(rows) != 2 {
		t.Fatalf("pending batch actions = %+v", res.Continuation.PendingToolCall.Args)
	}
}

func TestMessageLoopStripSilenceSuggestChineseAllProjectIgnoresStaleRanges(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"bad route","tool_calls":[{"id":"observe_mix","tool":"mix.observe","args":{"scope":"selected_ranges"},"reason":"wrong route"}]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 5, MaxToolCalls: 4, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "Strip Silence \u8bf7\u4e3a\u5168\u5de5\u7a0b\u6240\u6709\u8f68\u9053\u751f\u6210\u7247\u6bb5\u6e05\u7406\u5efa\u8bae\uff0c\u4e0d\u8981\u6267\u884c",
		AllowedTools: []string{"clip.strip_silence.suggest", "clip.strip_silence.analyze", "clip.strip_silence.apply", "clip.strip_silence.apply_batch", "mix.observe"},
		Context: map[string]any{
			"current_selection": map[string]any{
				"selected_clip_id":       "1011",
				"selected_clip_track_id": "1007",
				"selected_clip_ranges": []any{
					map[string]any{
						"range_id":         "stale_range",
						"clip_id":          "1011",
						"track_id":         "1007",
						"start_seconds":    1.0,
						"end_seconds":      2.0,
						"duration_seconds": 1.0,
					},
				},
			},
		},
	})

	if res.Status != "waiting_confirmation" || res.StopReason != StopReasonNeedsConfirmation {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(client.calls) != 0 {
		t.Fatalf("LLM should not be called for deterministic all-project strip silence suggest, calls=%d", len(client.calls))
	}
	if len(exec.calls) != 1 || exec.calls[0].Tool != "clip.strip_silence.suggest" {
		t.Fatalf("executor calls = %+v, want clip.strip_silence.suggest only", exec.calls)
	}
	if firstMapText(exec.calls[0].Args, "scope") != "all_project" {
		t.Fatalf("suggest args = %+v, want all_project", exec.calls[0].Args)
	}
	if _, ok := exec.calls[0].Args["selected_clip_ranges"]; ok {
		t.Fatalf("all-project request should ignore stale selected ranges: %+v", exec.calls[0].Args)
	}
	if res.Continuation == nil || res.Continuation.PendingToolCall == nil || res.Continuation.PendingToolCall.Tool != "clip.strip_silence.apply_batch" {
		t.Fatalf("pending continuation = %+v", res.Continuation)
	}
}

func TestMessageLoopB1GainStagingCheckReturnsTechnicalReport(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"B1 pack ready","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{
		projectState: map[string]any{
			"status":                   "ok",
			"raw_project_state_marker": "should_not_leak_project_state_raw",
			"tracks": []map[string]any{
				{
					"track_id":   "vocal_1",
					"track_name": "Lead Vocal",
					"volume_db":  -7.0,
					"clips": []map[string]any{{
						"clip_id":          "clip_v",
						"type":             "audio",
						"clip_gain_db":     7.0,
						"duration_seconds": 12.0,
					}},
				},
				{
					"track_id":   "bass_1",
					"track_name": "Bass",
					"volume_db":  0.0,
					"clips": []map[string]any{{
						"clip_id":          "clip_b",
						"type":             "audio",
						"clip_gain_db":     0.0,
						"duration_seconds": 12.0,
					}},
				},
			},
		},
		mixObservationResult: map[string]any{
			"status":         "ok",
			"observation_id": "obs_b1",
			"mix_session_id": "mix_b1",
			"observation": map[string]any{
				"observation_id": "obs_b1",
				"target_ref":     map[string]any{"kind": "project", "id": "current"},
				"llm_context":    map[string]any{"raw_observation_marker": "should_not_leak_observation_raw"},
				"project_package": map[string]any{
					"tracks": []map[string]any{
						{"track_id": "vocal_1", "role_guess": "vocal", "peak_dbfs": -0.05, "rms_dbfs": -14.0, "headroom_db": 0.05},
						{"track_id": "bass_1", "role_guess": "bass", "peak_dbfs": -4.0, "rms_dbfs": -22.0, "headroom_db": 4.0},
					},
				},
			},
		},
	}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Now:      func() time.Time { return time.Date(2026, 7, 9, 1, 2, 3, 0, time.UTC) },
	}

	result := loop.Start(context.Background(), Input{
		UserText: "\u68c0\u67e5 B1 \u589e\u76ca\u7ed3\u6784\u548c headroom \u662f\u5426\u5065\u5eb7",
		AllowedTools: []string{
			"project.state",
			"mix.observe", "mix.read", "mix.derive", "mix.request_observation",
			"clip.gain.read", "clip.gain.set",
			"mix.propose_tick", "mix.apply_tick",
		},
	})

	if result.Status != agentruntime.StatusCompleted {
		t.Fatalf("status = %s error=%s reply=%s", result.Status, result.Error, result.Reply)
	}
	if !strings.Contains(result.Reply, "B1 技术增益结构报告") || !strings.Contains(result.Reply, "可控轨道推子") || !strings.Contains(result.Reply, "clip gain") {
		t.Fatalf("B1 technical report missing expected sections: %q", result.Reply)
	}
	if result.ExecutionMemory.PendingMixTickCandidate != nil || result.ExecutionMemory.PendingMixTreatment != nil {
		t.Fatalf("read-only B1 check created pending action: %+v", result.ExecutionMemory)
	}
	if result.Continuation != nil && result.Continuation.PendingToolCall != nil {
		t.Fatalf("read-only B1 check created pending tool call: %+v", result.Continuation.PendingToolCall)
	}
	if len(exec.calls) < 2 || exec.calls[0].Tool != "mix.observe" || exec.calls[1].Tool != "project.state" {
		t.Fatalf("expected mix.observe then project.state before LLM, calls=%+v", exec.calls)
	}
	if firstMapText(exec.calls[0].Args, "scope") != "full_project" {
		t.Fatalf("B1 observe scope = %+v, want full_project", exec.calls[0].Args)
	}
	if len(client.calls) != 0 {
		t.Fatalf("B1 technical report should be deterministic, got %d LLM calls", len(client.calls))
	}
	for _, blocked := range []string{"<tool_result>{", "should_not_leak_project_state_raw", "should_not_leak_observation_raw"} {
		if strings.Contains(result.Reply, blocked) {
			t.Fatalf("B1 report leaked raw preflight result %q:\n%s", blocked, result.Reply)
		}
	}
}

func TestMessageLoopB1VolumeCheckRoutesToFullProjectContextPack(t *testing.T) {
	cases := []string{
		"\u5e2e\u6211\u8fdb\u884cB1\u9636\u6bb5\u7684\u97f3\u91cf\u68c0\u67e5",
		"\u6211\u60f3\u8ba9\u4f60\u5bf9\u6574\u4e2a\u5de5\u7a0b\u7684\u7535\u5e73\u8fdb\u884c\u68c0\u67e5\u505a\u5f97\u5230\u5417\uff1f",
	}
	for _, userText := range cases {
		t.Run(userText, func(t *testing.T) {
			client := &fakeMessageCompleter{responses: []string{
				`{"final":true,"reply":"B1 full project level pack ready","tool_calls":[]}`,
			}}
			exec := &fakeMessageExecutor{
				projectState: map[string]any{
					"status": "ok",
					"tracks": []map[string]any{{
						"track_id":   "track_selected",
						"track_name": "Selected Track",
						"volume_db":  0.0,
						"clips": []map[string]any{{
							"clip_id":          "clip_selected",
							"type":             "audio",
							"clip_gain_db":     0.0,
							"duration_seconds": 12.0,
						}},
					}, {
						"track_id":   "track_other",
						"track_name": "Other Track",
						"volume_db":  0.0,
						"clips": []map[string]any{{
							"clip_id":          "clip_other",
							"type":             "audio",
							"clip_gain_db":     0.0,
							"duration_seconds": 12.0,
						}},
					}},
				},
				mixObservationResult: map[string]any{
					"status":         "ok",
					"observation_id": "obs_b1_volume",
					"mix_session_id": "mix_b1_volume",
					"observation": map[string]any{
						"observation_id": "obs_b1_volume",
						"target_ref":     map[string]any{"kind": "project", "id": "current"},
						"project_package": map[string]any{
							"tracks": []map[string]any{
								{"track_id": "track_selected", "track_name": "Selected Track", "peak_dbfs": -12.0, "rms_dbfs": -24.0, "headroom_db": 12.0},
								{"track_id": "track_other", "track_name": "Other Track", "peak_dbfs": -9.0, "rms_dbfs": -20.0, "headroom_db": 9.0},
							},
						},
					},
				},
			}
			loop := &MessageLoop{
				Client:   client,
				Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
				Executor: exec,
				Now:      func() time.Time { return time.Date(2026, 7, 9, 1, 2, 3, 0, time.UTC) },
			}

			result := loop.Start(context.Background(), Input{
				UserText: userText,
				AllowedTools: []string{
					"project.state",
					"mix.observe", "mix.read", "mix.derive", "mix.request_observation",
					"clip.gain.read", "clip.gain.set",
					"mix.propose_tick", "mix.apply_tick",
				},
				Context: map[string]any{
					"ui_context": map[string]any{
						"selected_track_id": "track_selected",
						"selected_clip_id":  "clip_selected",
					},
				},
			})

			if result.Status != agentruntime.StatusCompleted {
				t.Fatalf("status = %s error=%s reply=%s", result.Status, result.Error, result.Reply)
			}
			if len(exec.calls) < 2 || exec.calls[0].Tool != "mix.observe" || exec.calls[1].Tool != "project.state" {
				t.Fatalf("expected B1 mix.observe then project.state before LLM, calls=%+v", exec.calls)
			}
			if exec.calls[0].ID != "observe_b1_gain_staging" {
				t.Fatalf("first call id = %q, want observe_b1_gain_staging", exec.calls[0].ID)
			}
			if firstMapText(exec.calls[0].Args, "scope") != "full_project" {
				t.Fatalf("B1 volume check scope = %+v, want full_project", exec.calls[0].Args)
			}
			if firstMapText(exec.calls[0].Args, "track_id", "selected_track_id", "target_track_id", "clip_id", "selected_clip_id") != "" {
				t.Fatalf("B1 full-project volume check leaked GUI selection into observe args: %+v", exec.calls[0].Args)
			}
			if len(client.calls) != 0 {
				t.Fatalf("B1 volume check should return deterministic technical report, got %d LLM calls", len(client.calls))
			}
			if !strings.Contains(result.Reply, "B1 技术增益结构报告") || !strings.Contains(result.Reply, "可控轨道推子") {
				t.Fatalf("B1 volume report missing expected content:\n%s", result.Reply)
			}
		})
	}
}

func TestMessageLoopB1FaderResetCreatesGroupAbsolutePendingAndCompletesAfterConfirmation(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"model should not be needed","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{
		projectState: map[string]any{
			"status": "ok",
			"tracks": []map[string]any{{
				"track_id":       "track_a",
				"track_name":     "A",
				"is_audio_track": true,
				"volume_db":      -60.0,
				"clips": []map[string]any{{
					"clip_id":          "clip_a",
					"type":             "audio",
					"duration_seconds": 12.0,
				}},
			}, {
				"track_id":       "track_b",
				"track_name":     "B",
				"is_audio_track": true,
				"volume_db":      -18.0,
				"clips": []map[string]any{{
					"clip_id":          "clip_b",
					"type":             "audio",
					"duration_seconds": 12.0,
				}},
			}, {
				"track_id":        "master",
				"track_name":      "Master",
				"is_master_track": true,
				"track_type":      "master",
				"volume_db":       -3.0,
			}},
		},
	}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Now:      func() time.Time { return time.Date(2026, 7, 9, 1, 2, 3, 0, time.UTC) },
	}

	res := loop.Start(context.Background(), Input{
		UserText: "B1 first step: set all track faders to 0 dB",
		AllowedTools: []string{
			"project.state",
			"mix.observe", "mix.read", "mix.derive", "mix.request_observation",
			"track.group.list", "track.group.apply_control",
		},
	})

	if res.Status != agentruntime.StatusWaitingConfirmation || res.StopReason != StopReasonNeedsConfirmation {
		t.Fatalf("result = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	if len(client.calls) != 0 {
		t.Fatalf("B1 fader reset should not call LLM before pending action, got %d calls", len(client.calls))
	}
	if len(exec.calls) != 3 || exec.calls[0].Tool != "mix.observe" || exec.calls[1].Tool != "project.state" || exec.calls[2].Tool != "track.group.apply_control" {
		t.Fatalf("executor calls = %+v", exec.calls)
	}
	pending := res.Continuation.PendingToolCall
	if pending == nil || pending.Tool != "track.group.apply_control" {
		t.Fatalf("pending continuation = %+v", res.Continuation)
	}
	if mode := firstMapText(pending.Args, "mode"); mode != "absolute" {
		t.Fatalf("pending mode = %q args=%+v", mode, pending.Args)
	}
	if db, ok := firstNumericMapValue(pending.Args, "db"); !ok || db != 0 {
		t.Fatalf("pending db = %v ok=%v args=%+v", db, ok, pending.Args)
	}
	if ids := messageLoopStringSlice(pending.Args["track_ids"]); len(ids) != 2 || ids[0] != "track_a" || ids[1] != "track_b" {
		t.Fatalf("pending track_ids = %#v", pending.Args["track_ids"])
	}
	if !messageLoopBool(pending.Args["create_group_if_missing"]) || !messageLoopBool(pending.Args["replace_members"]) {
		t.Fatalf("pending should create/reuse and replace B1 group members: %+v", pending.Args)
	}

	confirmed := loop.ResumeAfterConfirmation(context.Background(), *res.Continuation)
	if confirmed.Status != agentruntime.StatusCompleted || confirmed.StopReason != StopReasonDone {
		t.Fatalf("confirmed result = status=%q stop=%q reply=%q error=%q", confirmed.Status, confirmed.StopReason, confirmed.Reply, confirmed.Error)
	}
	if !strings.Contains(confirmed.Reply, "仍非 0 dB 0") || !strings.Contains(confirmed.Reply, "可控轨道 2") {
		t.Fatalf("confirmed reply does not summarize verified reset: %q", confirmed.Reply)
	}
	if len(exec.calls) != 6 || exec.calls[3].Tool != "track.group.apply_control" || !exec.confirmed[3] || exec.calls[4].Tool != "project.state" || exec.calls[5].Tool != "mix.observe" {
		t.Fatalf("confirmed executor calls=%+v confirmed=%+v", exec.calls, exec.confirmed)
	}
	if len(confirmed.Executed) == 0 {
		t.Fatalf("confirmed execution records missing")
	}
	var result map[string]any
	for _, record := range confirmed.Executed {
		if firstNonEmpty(messageLoopText(record["tool"]), messageLoopText(record["command_name"])) == "track.group.apply_control" {
			result = messageLoopMapValue(record["result"])
			break
		}
	}
	if len(result) == 0 {
		t.Fatalf("confirmed group execution record missing: %+v", confirmed.Executed)
	}
	for _, member := range messageLoopMapRows(result["members"]) {
		if db, ok := firstNumericMapValue(member, "after_db"); !ok || db != 0 {
			t.Fatalf("member was not verified at 0 dB: %+v", member)
		}
	}
	for _, trackID := range []string{"track_a", "track_b"} {
		if db, ok := messageLoopB1TrackVolumeFromProjectState(exec.projectState, trackID); !ok || db != 0 {
			t.Fatalf("fake project state did not persist fader reset for %s: db=%v ok=%v state=%+v", trackID, db, ok, exec.projectState)
		}
	}
}

func TestMessageLoopB1AbnormalFaderResetWordingCreatesGroupAbsolutePending(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"model should not be needed","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{
		projectState: map[string]any{
			"status": "ok",
			"tracks": []map[string]any{{
				"track_id":       "track_low",
				"track_name":     "Pulled Low",
				"is_audio_track": true,
				"volume_db":      -60.0,
			}, {
				"track_id":       "track_ok",
				"track_name":     "Unity",
				"is_audio_track": true,
				"volume_db":      0.0,
			}},
		},
	}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Now:      func() time.Time { return time.Date(2026, 7, 9, 1, 2, 3, 0, time.UTC) },
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "Please reset the abnormal faders to 0 dB for B1 gain staging",
		AllowedTools: []string{"project.state", "mix.observe", "track.group.apply_control"},
	})

	if res.Status != agentruntime.StatusWaitingConfirmation || res.StopReason != StopReasonNeedsConfirmation {
		t.Fatalf("result = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	if len(client.calls) != 0 {
		t.Fatalf("abnormal fader reset should be deterministic, got %d LLM calls", len(client.calls))
	}
	if len(exec.calls) != 3 || exec.calls[2].Tool != "track.group.apply_control" {
		t.Fatalf("executor calls = %+v", exec.calls)
	}
	ids := messageLoopStringSlice(exec.calls[2].Args["track_ids"])
	if len(ids) != 1 || ids[0] != "track_low" {
		t.Fatalf("reset target IDs = %+v args=%+v", ids, exec.calls[2].Args)
	}
	if db, ok := firstNumericMapValue(exec.calls[2].Args, "db"); !ok || db != 0 {
		t.Fatalf("reset db = %v ok=%v args=%+v", db, ok, exec.calls[2].Args)
	}
}

func TestMessageLoopB1GainStagingClipGainRiskCreatesClipGainConfirmation(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"model should not be needed","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{
		projectState: map[string]any{
			"status": "ok",
			"tracks": []map[string]any{{
				"track_id":   "track_vocal",
				"track_name": "Lead Vocal",
				"volume_db":  0.0,
				"clips": []map[string]any{{
					"clip_id":          "clip_vocal",
					"clip_name":        "Lead Vocal.wav",
					"type":             "audio",
					"clip_gain_db":     7.0,
					"duration_seconds": 12.0,
				}},
			}},
		},
	}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Now:      func() time.Time { return time.Date(2026, 7, 9, 1, 2, 3, 0, time.UTC) },
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "\u7ed9 B1 \u589e\u76ca\u7ed3\u6784\u751f\u6210\u4fee\u590d\u5efa\u8bae",
		AllowedTools: []string{"project.state", "mix.observe", "clip.gain.set", "mix.propose_tick", "mix.apply_tick"},
	})

	if res.Status != agentruntime.StatusWaitingConfirmation || res.StopReason != StopReasonNeedsConfirmation {
		t.Fatalf("result = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	if len(client.calls) != 0 {
		t.Fatalf("B1 deterministic clip gain suggestion should not call LLM, got %d calls", len(client.calls))
	}
	if len(exec.calls) != 3 || exec.calls[0].Tool != "mix.observe" || exec.calls[1].Tool != "project.state" || exec.calls[2].Tool != "clip.gain.set" {
		t.Fatalf("executor calls = %+v", exec.calls)
	}
	if got := firstMapText(exec.calls[2].Args, "clip_id"); got != "clip_vocal" {
		t.Fatalf("clip.gain.set clip_id = %q", got)
	}
	if gainDB, ok := firstNumericMapValue(exec.calls[2].Args, "gain_db"); !ok || gainDB != 0 {
		t.Fatalf("clip.gain.set gain_db = %v ok=%v args=%+v", gainDB, ok, exec.calls[2].Args)
	}
	if res.Continuation == nil || res.Continuation.PendingToolCall == nil || res.Continuation.PendingToolCall.Tool != "clip.gain.set" {
		t.Fatalf("pending continuation = %+v", res.Continuation)
	}
	if res.ExecutionMemory.PendingMixTickCandidate != nil || res.ExecutionMemory.PendingMixTreatment != nil {
		t.Fatalf("clip gain suggestion should use native pending tool, not mix memory: %+v", res.ExecutionMemory)
	}
}

func TestMessageLoopB12SourceCalibrationBlocksSelectedOnlyCoverage(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"model should not be needed","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{
		projectState: map[string]any{
			"status": "ok",
			"tracks": []map[string]any{{
				"track_id":       "track_quiet",
				"track_name":     "Quiet",
				"is_audio_track": true,
				"volume_db":      0.0,
				"clips": []map[string]any{{
					"clip_id":          "clip_quiet",
					"clip_name":        "Quiet.wav",
					"type":             "audio",
					"clip_gain_db":     0.0,
					"duration_seconds": 12.0,
				}},
			}, {
				"track_id":       "track_ref_a",
				"track_name":     "Reference A",
				"is_audio_track": true,
				"volume_db":      0.0,
				"clips": []map[string]any{{
					"clip_id":          "clip_ref_a",
					"type":             "audio",
					"clip_gain_db":     0.0,
					"duration_seconds": 12.0,
				}},
			}, {
				"track_id":       "track_ref_b",
				"track_name":     "Reference B",
				"is_audio_track": true,
				"volume_db":      0.0,
				"clips": []map[string]any{{
					"clip_id":          "clip_ref_b",
					"type":             "audio",
					"clip_gain_db":     0.0,
					"duration_seconds": 12.0,
				}},
			}},
		},
		mixObservationResult: map[string]any{
			"status":         "ok",
			"observation_id": "obs_selected_only",
			"mix_session_id": "mix_selected_only",
			"observation": map[string]any{
				"observation_id": "obs_selected_only",
				"target_ref":     map[string]any{"kind": "selected_clip", "id": "clip_quiet"},
				"project_package": map[string]any{
					"tracks": []map[string]any{
						{"track_id": "track_quiet", "track_name": "Quiet", "rms_dbfs": -30.0, "peak_dbfs": -12.0, "headroom_db": 12.0},
					},
				},
			},
		},
	}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Now:      func() time.Time { return time.Date(2026, 7, 10, 1, 2, 3, 0, time.UTC) },
	}

	res := loop.Start(context.Background(), Input{
		UserText: "\u8fdb\u884cB1.2",
		AllowedTools: []string{
			"project.state",
			"mix.observe", "mix.read", "mix.derive", "mix.request_observation",
			"clip.gain.read", "clip.gain.set", "clip.gain.set_batch",
			"mix.propose_tick", "mix.apply_tick", "track.group.apply_control",
		},
	})

	if res.Status != agentruntime.StatusCompleted || res.StopReason != StopReasonDone {
		t.Fatalf("result = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	if res.Continuation != nil && res.Continuation.PendingToolCall != nil {
		t.Fatalf("selected-only evidence must not create pending action: %+v", res.Continuation.PendingToolCall)
	}
	if len(client.calls) != 0 {
		t.Fatalf("B1.2 partial evidence block should be deterministic, got %d LLM calls", len(client.calls))
	}
	if len(exec.calls) != 2 || exec.calls[0].Tool != "mix.observe" || exec.calls[1].Tool != "project.state" {
		t.Fatalf("executor calls = %+v", exec.calls)
	}
	for _, call := range exec.calls {
		if call.Tool == "clip.gain.set" || call.Tool == "clip.gain.set_batch" || call.Tool == "track.group.apply_control" || call.Tool == "mix.propose_tick" || call.Tool == "mix.apply_tick" {
			t.Fatalf("B1.2 partial selected-only evidence must not mutate: %+v", exec.calls)
		}
	}
	if !strings.Contains(res.Reply, "没有生成待确认修改") || !strings.Contains(res.Reply, "b1_2_source_level_admission_partial") {
		t.Fatalf("reply should explain B1.2 admission block:\n%s", res.Reply)
	}
}

func TestMessageLoopB12SourceCalibrationCreatesClipGainPendingAndVerifiesAfterConfirmation(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"model should not be needed","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{
		projectState: map[string]any{
			"status": "ok",
			"tracks": []map[string]any{{
				"track_id":       "track_quiet",
				"track_name":     "Quiet",
				"is_audio_track": true,
				"volume_db":      0.0,
				"clips": []map[string]any{{
					"clip_id":          "clip_quiet",
					"clip_name":        "Quiet.wav",
					"type":             "audio",
					"clip_gain_db":     0.0,
					"duration_seconds": 12.0,
				}},
			}, {
				"track_id":       "track_ref_a",
				"track_name":     "Reference A",
				"is_audio_track": true,
				"volume_db":      0.0,
				"clips": []map[string]any{{
					"clip_id":          "clip_ref_a",
					"type":             "audio",
					"clip_gain_db":     0.0,
					"duration_seconds": 12.0,
				}},
			}, {
				"track_id":       "track_ref_b",
				"track_name":     "Reference B",
				"is_audio_track": true,
				"volume_db":      0.0,
				"clips": []map[string]any{{
					"clip_id":          "clip_ref_b",
					"type":             "audio",
					"clip_gain_db":     0.0,
					"duration_seconds": 12.0,
				}},
			}},
		},
		mixObservationResult: map[string]any{
			"status":         "ok",
			"observation_id": "obs_b12",
			"mix_session_id": "mix_b12",
			"observation": map[string]any{
				"observation_id": "obs_b12",
				"target_ref":     map[string]any{"kind": "project", "id": "current"},
				"project_package": map[string]any{
					"tracks": []map[string]any{
						{"track_id": "track_quiet", "track_name": "Quiet", "rms_dbfs": -30.0, "peak_dbfs": -12.0, "headroom_db": 12.0},
						{"track_id": "track_ref_a", "track_name": "Reference A", "rms_dbfs": -20.0, "peak_dbfs": -8.0, "headroom_db": 8.0},
						{"track_id": "track_ref_b", "track_name": "Reference B", "rms_dbfs": -20.0, "peak_dbfs": -8.0, "headroom_db": 8.0},
					},
				},
			},
		},
	}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Now:      func() time.Time { return time.Date(2026, 7, 10, 1, 2, 3, 0, time.UTC) },
	}

	res := loop.Start(context.Background(), Input{
		UserText: "Continue B1.2 source level calibration from full-project level evidence",
		AllowedTools: []string{
			"project.state",
			"mix.observe", "mix.read", "mix.derive", "mix.request_observation",
			"clip.gain.read", "clip.gain.set",
			"mix.propose_tick", "mix.apply_tick", "track.group.apply_control",
		},
	})

	if res.Status != agentruntime.StatusWaitingConfirmation || res.StopReason != StopReasonNeedsConfirmation {
		t.Fatalf("result = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	if len(client.calls) != 0 {
		t.Fatalf("B1.2 source calibration should be deterministic, got %d LLM calls", len(client.calls))
	}
	if len(exec.calls) != 3 || exec.calls[0].Tool != "mix.observe" || exec.calls[1].Tool != "project.state" || exec.calls[2].Tool != "clip.gain.set" {
		t.Fatalf("executor calls = %+v", exec.calls)
	}
	for _, call := range exec.calls {
		if call.Tool == "mix.propose_tick" || call.Tool == "mix.apply_tick" || call.Tool == "track.group.apply_control" {
			t.Fatalf("B1.2 must not use fader/group/mix tick adjustment route: %+v", exec.calls)
		}
	}
	pending := res.Continuation.PendingToolCall
	if pending == nil || pending.Tool != "clip.gain.set" {
		t.Fatalf("pending continuation = %+v", res.Continuation)
	}
	if got := firstMapText(pending.Args, "origin"); got != "b1_2_source_calibration" {
		t.Fatalf("pending origin = %q args=%+v", got, pending.Args)
	}
	if got := firstMapText(pending.Args, "b1_action_kind"); got != "source_clip_gain_calibration" {
		t.Fatalf("pending action kind = %q args=%+v", got, pending.Args)
	}
	if got := firstMapText(pending.Args, "clip_id"); got != "clip_quiet" {
		t.Fatalf("pending clip_id = %q args=%+v", got, pending.Args)
	}
	if gainDB, ok := firstNumericMapValue(pending.Args, "gain_db"); !ok || gainDB != 10 {
		t.Fatalf("pending gain = %v ok=%v args=%+v", gainDB, ok, pending.Args)
	}
	if !strings.Contains(res.Reply, "B1.2") || !strings.Contains(res.Reply, "project.state / mix.observe") {
		t.Fatalf("pending reply should include B1.2 report and verification plan:\n%s", res.Reply)
	}

	confirmed := loop.ResumeAfterConfirmation(context.Background(), *res.Continuation)
	if confirmed.Status != agentruntime.StatusCompleted || confirmed.StopReason != StopReasonDone {
		t.Fatalf("confirmed result = status=%q stop=%q reply=%q error=%q", confirmed.Status, confirmed.StopReason, confirmed.Reply, confirmed.Error)
	}
	if len(exec.calls) != 6 {
		t.Fatalf("executor calls after confirm = %+v", exec.calls)
	}
	if exec.calls[3].Tool != "clip.gain.set" || !exec.confirmed[3] || exec.calls[4].Tool != "project.state" || exec.calls[5].Tool != "mix.observe" {
		t.Fatalf("confirm route = calls=%+v confirmed=%+v", exec.calls, exec.confirmed)
	}
	if !strings.Contains(confirmed.Reply, "project.state clip gain") || !strings.Contains(confirmed.Reply, "mix.observe") || !strings.Contains(confirmed.Reply, "+10.00 dB") {
		t.Fatalf("confirmed reply missing verification summary:\n%s", confirmed.Reply)
	}
	if db, ok := messageLoopB12ClipGainFromProjectState(&runState{input: Input{State: exec.projectState}}, "clip_quiet"); !ok || db != 10 {
		t.Fatalf("fake project state did not persist clip gain: db=%v ok=%v state=%+v", db, ok, exec.projectState)
	}
}

func TestMessageLoopB12StrictReferenceCalibrationCreatesSingleBatchForManyTracks(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"model should not be needed","tool_calls":[]}`,
	}}
	projectTracks := make([]map[string]any, 0, 10)
	waveRows := make([]map[string]any, 0, 10)
	levels := []float64{-21.0, -20.8, -20.6, -20.4, -20.2, -19.8, -19.6, -19.4, -19.2, -19.0}
	for i, level := range levels {
		trackID := fmt.Sprintf("track_%02d", i)
		clipID := fmt.Sprintf("clip_%02d", i)
		projectTracks = append(projectTracks, map[string]any{
			"track_id":       trackID,
			"track_name":     fmt.Sprintf("Track %02d", i),
			"is_audio_track": true,
			"track_type":     "audio",
			"volume_db":      0.0,
			"clips": []map[string]any{{
				"clip_id":          clipID,
				"clip_name":        clipID + ".wav",
				"type":             "audio",
				"clip_gain_db":     0.0,
				"duration_seconds": 12.0,
			}},
		})
		waveRows = append(waveRows, map[string]any{
			"track_id":        trackID,
			"clip_id":         clipID,
			"active_rms_dbfs": level,
			"rms_dbfs":        level - 1.0,
			"peak_dbfs":       -8.0,
			"headroom_db":     8.0,
		})
	}
	exec := &fakeMessageExecutor{
		projectState: map[string]any{
			"status": "ok",
			"tracks": projectTracks,
		},
		audioAnalysisStatusResults: []map[string]any{{
			"status":  "ok",
			"command": "project.audio_analysis_status",
			"analysis_job": map[string]any{
				"track_waveform_envelopes": waveRows,
			},
		}},
	}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Now:      func() time.Time { return time.Date(2026, 7, 10, 1, 2, 3, 0, time.UTC) },
	}

	res := loop.Start(context.Background(), Input{
		UserText: "B1.2 strict reference level calibration for the full project",
		Conversation: []llm.Message{{
			Role:    "user",
			Content: `<capability_context_pack>{"capability_id":"static_mix.gain_staging.v0","pack_id":"stale_previous_turn"}</capability_context_pack>`,
		}},
		AllowedTools: []string{
			"project.state", "project.audio_analysis_status",
			"mix.observe", "mix.read", "mix.derive", "mix.request_observation",
			"clip.gain.read", "clip.gain.set", "clip.gain.set_batch",
			"mix.propose_tick", "mix.apply_tick", "track.group.apply_control",
		},
	})

	if res.Status != agentruntime.StatusWaitingConfirmation || res.StopReason != StopReasonNeedsConfirmation {
		t.Fatalf("result = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	if len(client.calls) != 0 {
		t.Fatalf("B1.2 strict reference calibration should be deterministic, got %d LLM calls", len(client.calls))
	}
	if len(exec.calls) != 4 ||
		exec.calls[0].Tool != "mix.observe" ||
		exec.calls[1].Tool != "project.state" ||
		exec.calls[2].Tool != "project.audio_analysis_status" ||
		exec.calls[3].Tool != "clip.gain.set_batch" {
		t.Fatalf("executor calls = %+v", exec.calls)
	}
	for _, call := range exec.calls {
		if call.Tool == "clip.gain.set" || call.Tool == "mix.propose_tick" || call.Tool == "mix.apply_tick" || call.Tool == "track.group.apply_control" {
			t.Fatalf("strict B1.2 must use one batch clip gain route, calls=%+v", exec.calls)
		}
	}
	pending := res.Continuation.PendingToolCall
	if pending == nil || pending.Tool != "clip.gain.set_batch" {
		t.Fatalf("pending continuation = %+v", res.Continuation)
	}
	actions := messageLoopMapRows(pending.Args["pending_actions"])
	if len(actions) != 10 {
		t.Fatalf("pending actions = %d %+v", len(actions), actions)
	}
	if metadata := messageLoopMapValue(pending.Args["b1_metadata"]); firstMapText(metadata, "calibration_mode") != "strict" {
		t.Fatalf("batch metadata = %+v args=%+v", metadata, pending.Args)
	}

	confirmed := loop.ResumeAfterConfirmation(context.Background(), *res.Continuation)
	if confirmed.Status != agentruntime.StatusCompleted || confirmed.StopReason != StopReasonDone {
		t.Fatalf("confirmed result = status=%q stop=%q reply=%q error=%q", confirmed.Status, confirmed.StopReason, confirmed.Reply, confirmed.Error)
	}
	if len(exec.calls) != 7 || exec.calls[4].Tool != "clip.gain.set_batch" || !exec.confirmed[4] || exec.calls[5].Tool != "project.state" || exec.calls[6].Tool != "mix.observe" {
		t.Fatalf("confirm route = calls=%+v confirmed=%+v", exec.calls, exec.confirmed)
	}
	if db, ok := messageLoopB12ClipGainFromProjectState(&runState{input: Input{State: exec.projectState}}, "clip_00"); !ok || db != 1 {
		t.Fatalf("first clip gain not persisted: db=%v ok=%v state=%+v", db, ok, exec.projectState)
	}
	if db, ok := messageLoopB12ClipGainFromProjectState(&runState{input: Input{State: exec.projectState}}, "clip_09"); !ok || db != -1 {
		t.Fatalf("last clip gain not persisted: db=%v ok=%v state=%+v", db, ok, exec.projectState)
	}
}

func TestMessageLoopB1FullWorkflowSkipsUnityResetWaitsForDADAndQueuesB12(t *testing.T) {
	t.Setenv("VIT_AGENT_DAD_GATE_POLL_MS", "0")
	client := &fakeMessageCompleter{responses: []string{`{"final":true,"reply":"model should not be needed","tool_calls":[]}`}}
	levels := []float64{-30.0, -20.0, -10.0}
	projectTracks := make([]map[string]any, 0, len(levels))
	waveRows := make([]map[string]any, 0, len(levels))
	for i, level := range levels {
		trackID := fmt.Sprintf("track_full_%d", i)
		clipID := fmt.Sprintf("clip_full_%d", i)
		projectTracks = append(projectTracks, map[string]any{
			"track_id": trackID, "track_name": trackID, "is_audio_track": true, "track_type": "audio", "volume_db": 0.0,
			"clips": []map[string]any{{"clip_id": clipID, "type": "audio", "clip_gain_db": 0.0, "duration_seconds": 12.0}},
		})
		waveRows = append(waveRows, map[string]any{
			"status": "ready", "track_id": trackID, "clip_id": clipID,
			"rms_dbfs": level, "peak_dbfs": -6.0, "headroom_db": 6.0,
		})
	}
	partialJob := map[string]any{
		"status": "submitted", "analysis_queue_status": "submitted", "dad_fact_status": "partial",
		"dad_fact_ready_count": 1, "dad_fact_total_count": 3, "dad_fact_pending_count": 2,
		"track_waveform_envelopes": waveRows[:1],
	}
	readyJob := map[string]any{
		"status": "submitted", "analysis_queue_status": "submitted", "dad_fact_status": "ready",
		"dad_fact_ready_count": 3, "dad_fact_total_count": 3, "dad_fact_pending_count": 0,
		"track_waveform_envelopes": waveRows,
	}
	exec := &fakeMessageExecutor{
		projectState: map[string]any{"status": "ok", "tracks": projectTracks},
		audioAnalysisStatusResults: []map[string]any{
			{"status": "ok", "analysis_job": partialJob},
			{"status": "ok", "analysis_job": readyJob},
		},
	}
	loop := &MessageLoop{
		Client: client, Config: config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec, Now: func() time.Time { return time.Date(2026, 7, 11, 1, 2, 3, 0, time.UTC) },
	}

	res := loop.Start(context.Background(), Input{
		UserText: "\u5e2e\u6211\u6267\u884cB1",
		AllowedTools: []string{
			"project.state", "project.audio_analysis_status", "project.audio_analysis_start",
			"mix.observe", "mix.read", "mix.derive", "mix.request_observation",
			"clip.gain.read", "clip.gain.set", "clip.gain.set_batch", "track.group.apply_control",
		},
	})

	if res.Status != agentruntime.StatusWaitingConfirmation || res.StopReason != StopReasonNeedsConfirmation {
		t.Fatalf("result = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	if len(client.calls) != 0 {
		t.Fatalf("full B1 workflow should be deterministic, got %d LLM calls", len(client.calls))
	}
	wantTools := []string{"mix.observe", "project.state", "project.audio_analysis_status", "project.audio_analysis_start", "project.audio_analysis_status", "clip.gain.set_batch"}
	if len(exec.calls) != len(wantTools) {
		t.Fatalf("executor calls = %+v", exec.calls)
	}
	for i, want := range wantTools {
		if exec.calls[i].Tool != want {
			t.Fatalf("executor call %d = %q, want %q; calls=%+v", i, exec.calls[i].Tool, want, exec.calls)
		}
	}
	pending := res.Continuation.PendingToolCall
	if pending == nil || pending.Tool != "clip.gain.set_batch" {
		t.Fatalf("pending continuation = %+v", res.Continuation)
	}
	if actions := messageLoopMapRows(pending.Args["pending_actions"]); len(actions) != 2 {
		t.Fatalf("full B1 pending actions = %d %+v", len(actions), actions)
	}
	for _, call := range exec.calls {
		if call.Tool == "track.group.apply_control" {
			t.Fatalf("already-unity B1 must skip B1.1 mutation: %+v", exec.calls)
		}
	}
}

func TestMessageLoopB12ChineseStrictReferenceIntentRoutesToBatch(t *testing.T) {
	userText := "\u4e25\u683c\u53c2\u8003\u7535\u5e73\u6821\u51c6\uff1a\u4f7f\u7528\u5168\u5de5\u7a0b\u540c\u4e00\u53c2\u8003\u6307\u6807\uff0c\u8bc6\u522b\u9700\u8981\u9759\u6001 clip gain \u6821\u51c6\u7684\u4e3b\u97f3\u9891\u7247\u6bb5\uff0c\u5e76\u51c6\u5907 clip.gain.set_batch \u5f85\u786e\u8ba4\u52a8\u4f5c"
	if !messageLoopGainStagingCapabilityRequest(userText) {
		t.Fatalf("Chinese strict reference request should be routed as B1 gain staging")
	}
	if messageLoopClipFadeGainRequest(userText) {
		t.Fatalf("B1.2 clip gain calibration must not be routed as single-clip fade/gain")
	}
	if !messageLoopGainStagingSourceCalibrationRequest(userText) {
		t.Fatalf("Chinese strict reference request should be source calibration")
	}
	if !messageLoopGainStagingSuggestionRequest(userText) {
		t.Fatalf("Chinese strict reference request should prepare a pending action")
	}
	if messageLoopReadOnlyObservationRequest(userText) {
		t.Fatalf("Chinese strict reference request with prepare/calibration intent must not be read-only")
	}

	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"model should not be needed","tool_calls":[]}`,
	}}
	projectTracks := make([]map[string]any, 0, 6)
	waveRows := make([]map[string]any, 0, 6)
	levels := []float64{-22.0, -21.0, -20.0, -19.0, -18.0, -17.0}
	for i, level := range levels {
		trackID := fmt.Sprintf("track_cn_%02d", i)
		clipID := fmt.Sprintf("clip_cn_%02d", i)
		projectTracks = append(projectTracks, map[string]any{
			"track_id":       trackID,
			"track_name":     fmt.Sprintf("CN Track %02d", i),
			"is_audio_track": true,
			"track_type":     "audio",
			"volume_db":      0.0,
			"clips": []map[string]any{{
				"clip_id":          clipID,
				"clip_name":        clipID + ".wav",
				"type":             "audio",
				"clip_gain_db":     0.0,
				"duration_seconds": 12.0,
			}},
		})
		waveRows = append(waveRows, map[string]any{
			"track_id":        trackID,
			"clip_id":         clipID,
			"active_rms_dbfs": level,
			"rms_dbfs":        level - 1.0,
			"peak_dbfs":       -8.0,
			"headroom_db":     8.0,
		})
	}
	exec := &fakeMessageExecutor{
		projectState: map[string]any{
			"status": "ok",
			"tracks": projectTracks,
		},
		audioAnalysisStatusResults: []map[string]any{{
			"status":  "ok",
			"command": "project.audio_analysis_status",
			"analysis_job": map[string]any{
				"track_waveform_envelopes": waveRows,
			},
		}},
	}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Now:      func() time.Time { return time.Date(2026, 7, 10, 1, 2, 3, 0, time.UTC) },
	}

	res := loop.Start(context.Background(), Input{
		UserText: userText,
		AllowedTools: []string{
			"project.state", "project.audio_analysis_status",
			"mix.observe", "mix.read", "mix.derive", "mix.request_observation",
			"clip.gain.read", "clip.gain.set", "clip.gain.set_batch",
			"mix.propose_tick", "mix.apply_tick", "track.group.apply_control",
		},
	})

	if res.Status != agentruntime.StatusWaitingConfirmation || res.StopReason != StopReasonNeedsConfirmation {
		t.Fatalf("result = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	if len(client.calls) != 0 {
		t.Fatalf("Chinese strict reference calibration should be deterministic, got %d LLM calls", len(client.calls))
	}
	if len(exec.calls) != 4 ||
		exec.calls[0].Tool != "mix.observe" ||
		exec.calls[1].Tool != "project.state" ||
		exec.calls[2].Tool != "project.audio_analysis_status" ||
		exec.calls[3].Tool != "clip.gain.set_batch" {
		t.Fatalf("executor calls = %+v", exec.calls)
	}
	pending := res.Continuation.PendingToolCall
	if pending == nil || pending.Tool != "clip.gain.set_batch" {
		t.Fatalf("pending continuation = %+v", res.Continuation)
	}
	actions := messageLoopMapRows(pending.Args["pending_actions"])
	if len(actions) != len(levels) {
		t.Fatalf("pending actions = %d %+v", len(actions), actions)
	}
	if metadata := messageLoopMapValue(pending.Args["b1_metadata"]); firstMapText(metadata, "calibration_mode") != "strict" {
		t.Fatalf("batch metadata = %+v args=%+v", metadata, pending.Args)
	}
}

func TestMessageLoopB13FaderResetConfirmationReobservesAndCreatesB12Pending(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"model should not be needed","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{
		projectState: map[string]any{
			"status": "ok",
			"tracks": []map[string]any{{
				"track_id":       "track_quiet_a",
				"track_name":     "Quiet A",
				"is_audio_track": true,
				"volume_db":      -12.0,
				"clips": []map[string]any{{
					"clip_id":          "clip_quiet_a",
					"clip_name":        "Quiet A.wav",
					"type":             "audio",
					"clip_gain_db":     0.0,
					"duration_seconds": 12.0,
				}},
			}, {
				"track_id":       "track_quiet_b",
				"track_name":     "Quiet B",
				"is_audio_track": true,
				"volume_db":      0.0,
				"clips": []map[string]any{{
					"clip_id":          "clip_quiet_b",
					"clip_name":        "Quiet B.wav",
					"type":             "audio",
					"clip_gain_db":     0.0,
					"duration_seconds": 12.0,
				}},
			}, {
				"track_id":       "track_ref_a",
				"track_name":     "Reference A",
				"is_audio_track": true,
				"volume_db":      0.0,
				"clips": []map[string]any{{
					"clip_id":          "clip_ref_a",
					"type":             "audio",
					"clip_gain_db":     0.0,
					"duration_seconds": 12.0,
				}},
			}, {
				"track_id":       "track_ref_b",
				"track_name":     "Reference B",
				"is_audio_track": true,
				"volume_db":      0.0,
				"clips": []map[string]any{{
					"clip_id":          "clip_ref_b",
					"type":             "audio",
					"clip_gain_db":     0.0,
					"duration_seconds": 12.0,
				}},
			}, {
				"track_id":       "track_ref_c",
				"track_name":     "Reference C",
				"is_audio_track": true,
				"volume_db":      0.0,
				"clips": []map[string]any{{
					"clip_id":          "clip_ref_c",
					"type":             "audio",
					"clip_gain_db":     0.0,
					"duration_seconds": 12.0,
				}},
			}},
		},
		mixObservationResult: map[string]any{
			"status":         "ok",
			"observation_id": "obs_b13",
			"mix_session_id": "mix_b13",
			"observation": map[string]any{
				"observation_id": "obs_b13",
				"target_ref":     map[string]any{"kind": "project", "id": "current"},
				"project_package": map[string]any{
					"tracks": []map[string]any{
						{"track_id": "track_quiet_a", "track_name": "Quiet A", "rms_dbfs": -30.0, "peak_dbfs": -12.0, "headroom_db": 12.0},
						{"track_id": "track_quiet_b", "track_name": "Quiet B", "rms_dbfs": -29.0, "peak_dbfs": -12.0, "headroom_db": 12.0},
						{"track_id": "track_ref_a", "track_name": "Reference A", "rms_dbfs": -20.0, "peak_dbfs": -8.0, "headroom_db": 8.0},
						{"track_id": "track_ref_b", "track_name": "Reference B", "rms_dbfs": -20.0, "peak_dbfs": -8.0, "headroom_db": 8.0},
						{"track_id": "track_ref_c", "track_name": "Reference C", "rms_dbfs": -20.0, "peak_dbfs": -8.0, "headroom_db": 8.0},
					},
				},
			},
		},
	}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Now:      func() time.Time { return time.Date(2026, 7, 10, 1, 2, 3, 0, time.UTC) },
	}

	res := loop.Start(context.Background(), Input{
		UserText: "execute B1 gain staging repair for the whole project",
		AllowedTools: []string{
			"project.state",
			"mix.observe", "mix.read", "mix.derive", "mix.request_observation",
			"clip.gain.read", "clip.gain.set", "clip.gain.set_batch",
			"track.group.apply_control", "mix.propose_tick", "mix.apply_tick",
		},
	})

	if res.Status != agentruntime.StatusWaitingConfirmation || res.StopReason != StopReasonNeedsConfirmation {
		t.Fatalf("first result = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	if len(exec.calls) != 3 || exec.calls[2].Tool != "track.group.apply_control" {
		t.Fatalf("initial calls = %+v", exec.calls)
	}
	if res.Continuation == nil || res.Continuation.PendingToolCall == nil || res.Continuation.PendingToolCall.Tool != "track.group.apply_control" {
		t.Fatalf("first pending continuation = %+v", res.Continuation)
	}

	afterReset := loop.ResumeAfterConfirmation(context.Background(), *res.Continuation)
	if afterReset.Status != agentruntime.StatusWaitingConfirmation || afterReset.StopReason != StopReasonNeedsConfirmation {
		t.Fatalf("after reset = status=%q stop=%q reply=%q error=%q", afterReset.Status, afterReset.StopReason, afterReset.Reply, afterReset.Error)
	}
	if len(exec.calls) != 7 {
		t.Fatalf("calls after B1.1 confirm = %+v", exec.calls)
	}
	if exec.calls[3].Tool != "track.group.apply_control" || !exec.confirmed[3] || exec.calls[4].Tool != "project.state" || exec.calls[5].Tool != "mix.observe" || exec.calls[6].Tool != "clip.gain.set_batch" || exec.confirmed[6] {
		t.Fatalf("B1.3 route = calls=%+v confirmed=%+v", exec.calls, exec.confirmed)
	}
	if db, ok := messageLoopB1TrackVolumeFromProjectState(exec.projectState, "track_quiet_a"); !ok || db != 0 {
		t.Fatalf("track fader reset not persisted before B1.2: db=%v ok=%v state=%+v", db, ok, exec.projectState)
	}
	pending := afterReset.Continuation.PendingToolCall
	if pending == nil || pending.Tool != "clip.gain.set_batch" {
		t.Fatalf("second pending continuation = %+v", afterReset.Continuation)
	}
	if got := firstMapText(pending.Args, "origin"); got != "b1_2_source_calibration" {
		t.Fatalf("second pending origin = %q args=%+v", got, pending.Args)
	}
	actions := messageLoopMapRows(pending.Args["pending_actions"])
	if len(actions) != 2 {
		t.Fatalf("pending actions = %+v args=%+v", actions, pending.Args)
	}
	if got := firstMapText(messageLoopMapValue(actions[0]["args"]), "clip_id"); got != "clip_quiet_a" {
		t.Fatalf("first batch clip_id = %q actions=%+v", got, actions)
	}
	if !strings.Contains(afterReset.Reply, "B1.3") || !strings.Contains(afterReset.Reply, "B1.2") {
		t.Fatalf("after reset reply should describe B1.3 continuation into B1.2:\n%s", afterReset.Reply)
	}

	complete := loop.ResumeAfterConfirmation(context.Background(), *afterReset.Continuation)
	if complete.Status != agentruntime.StatusCompleted || complete.StopReason != StopReasonDone {
		t.Fatalf("complete = status=%q stop=%q reply=%q error=%q", complete.Status, complete.StopReason, complete.Reply, complete.Error)
	}
	if len(exec.calls) != 10 {
		t.Fatalf("calls after B1.2 confirm = %+v", exec.calls)
	}
	if exec.calls[7].Tool != "clip.gain.set_batch" || !exec.confirmed[7] || exec.calls[8].Tool != "project.state" || exec.calls[9].Tool != "mix.observe" {
		t.Fatalf("B1.2 confirm route = calls=%+v confirmed=%+v", exec.calls, exec.confirmed)
	}
	if db, ok := messageLoopB12ClipGainFromProjectState(&runState{input: Input{State: exec.projectState}}, "clip_quiet_a"); !ok || db != 10 {
		t.Fatalf("clip A gain not persisted after B1.2: db=%v ok=%v state=%+v", db, ok, exec.projectState)
	}
	if db, ok := messageLoopB12ClipGainFromProjectState(&runState{input: Input{State: exec.projectState}}, "clip_quiet_b"); !ok || db != 9 {
		t.Fatalf("clip B gain not persisted after B1.2: db=%v ok=%v state=%+v", db, ok, exec.projectState)
	}
	if len(client.calls) != 0 {
		t.Fatalf("B1.3 deterministic chain should not call LLM, got %d calls", len(client.calls))
	}
}

func TestMessageLoopB1GainStagingTrackFaderRiskCreatesGroupAbsoluteConfirmation(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"model should not be needed","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{
		projectState: map[string]any{
			"status": "ok",
			"tracks": []map[string]any{{
				"track_id":   "track_pad",
				"track_name": "Pad",
				"volume_db":  -60.0,
				"clips": []map[string]any{{
					"clip_id":          "clip_pad",
					"type":             "audio",
					"clip_gain_db":     0.0,
					"duration_seconds": 12.0,
				}},
			}, {
				"track_id":   "track_fx",
				"track_name": "FX",
				"volume_db":  -12.0,
				"clips": []map[string]any{{
					"clip_id":          "clip_fx",
					"type":             "audio",
					"clip_gain_db":     0.0,
					"duration_seconds": 12.0,
				}},
			}, {
				"track_id":   "track_ok",
				"track_name": "Already Unity",
				"volume_db":  0.0,
				"clips": []map[string]any{{
					"clip_id":          "clip_ok",
					"type":             "audio",
					"clip_gain_db":     0.0,
					"duration_seconds": 12.0,
				}},
			}},
		},
	}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Now:      func() time.Time { return time.Date(2026, 7, 9, 1, 2, 3, 0, time.UTC) },
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "\u6267\u884c B1 gain staging \u4fee\u590d\u5efa\u8bae",
		AllowedTools: []string{"project.state", "mix.observe", "track.group.apply_control", "mix.propose_tick", "mix.apply_tick"},
	})

	if res.Status != agentruntime.StatusWaitingConfirmation || res.StopReason != StopReasonNeedsConfirmation {
		t.Fatalf("result = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	if len(client.calls) != 0 {
		t.Fatalf("B1 deterministic fader reset suggestion should not call LLM, got %d calls", len(client.calls))
	}
	if len(exec.calls) != 3 || exec.calls[0].Tool != "mix.observe" || exec.calls[1].Tool != "project.state" || exec.calls[2].Tool != "track.group.apply_control" {
		t.Fatalf("executor calls = %+v", exec.calls)
	}
	if exec.calls[2].Tool == "mix.propose_tick" {
		t.Fatalf("B1 fader risk must not use +/-2 dB mix tick: %+v", exec.calls[2])
	}
	ids := messageLoopStringSlice(exec.calls[2].Args["track_ids"])
	if len(ids) != 2 || ids[0] != "track_pad" || ids[1] != "track_fx" {
		t.Fatalf("fader reset ids = %+v args=%+v", ids, exec.calls[2].Args)
	}
	if mode := firstMapText(exec.calls[2].Args, "mode"); mode != "absolute" {
		t.Fatalf("mode = %q args=%+v", mode, exec.calls[2].Args)
	}
	if db, ok := firstNumericMapValue(exec.calls[2].Args, "db"); !ok || db != 0 {
		t.Fatalf("db = %v ok=%v args=%+v", db, ok, exec.calls[2].Args)
	}
	if res.Continuation == nil || res.Continuation.PendingToolCall == nil || res.Continuation.PendingToolCall.Tool != "track.group.apply_control" {
		t.Fatalf("pending continuation = %+v", res.Continuation)
	}
	if res.ExecutionMemory.PendingMixTickCandidate != nil || res.ExecutionMemory.PendingMixTreatment != nil {
		t.Fatalf("fader reset suggestion should use group pending tool, not mix memory: %+v", res.ExecutionMemory)
	}
}

func TestMessageLoopB1GainStagingHeadroomRiskDoesNotCreateMixTick(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"model should not be needed","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{
		projectState: map[string]any{
			"status": "ok",
			"tracks": []map[string]any{{
				"track_id":   "track_drums",
				"track_name": "Drums",
				"volume_db":  0.0,
				"clips": []map[string]any{{
					"clip_id":          "clip_drums",
					"type":             "audio",
					"clip_gain_db":     0.0,
					"duration_seconds": 12.0,
				}},
			}},
		},
		mixObservationResult: map[string]any{
			"status":         "ok",
			"observation_id": "obs_b1_headroom",
			"mix_session_id": "mix_b1_headroom",
			"observation": map[string]any{
				"observation_id": "obs_b1_headroom",
				"target_ref":     map[string]any{"kind": "project", "id": "current"},
				"project_package": map[string]any{
					"tracks": []map[string]any{{
						"track_id":    "track_drums",
						"track_name":  "Drums",
						"peak_dbfs":   -0.05,
						"rms_dbfs":    -10.0,
						"headroom_db": 0.05,
					}},
				},
			},
		},
	}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Now:      func() time.Time { return time.Date(2026, 7, 9, 1, 2, 3, 0, time.UTC) },
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "\u6267\u884c B1 gain staging \u4fee\u590d\u5efa\u8bae",
		AllowedTools: []string{"project.state", "mix.observe", "mix.propose_tick", "mix.apply_tick"},
	})

	if res.Status != agentruntime.StatusCompleted || res.StopReason != StopReasonDone {
		t.Fatalf("result = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	if len(client.calls) != 0 {
		t.Fatalf("B1 deterministic headroom report should not call LLM, got %d calls", len(client.calls))
	}
	if len(exec.calls) != 2 || exec.calls[0].Tool != "mix.observe" || exec.calls[1].Tool != "project.state" {
		t.Fatalf("executor calls = %+v, want observe+project.state only", exec.calls)
	}
	for _, call := range exec.calls {
		if call.Tool == "mix.propose_tick" || call.Tool == "mix.apply_tick" {
			t.Fatalf("B1 suggest should not execute mix tick before chat confirmation, calls=%+v", exec.calls)
		}
	}
	candidate := res.ExecutionMemory.PendingMixTickCandidate
	if candidate != nil {
		t.Fatalf("B1 headroom-only risk should not create track fader mix tick candidate: %+v", candidate)
	}
	if res.Continuation != nil && res.Continuation.PendingToolCall != nil {
		t.Fatalf("headroom-only suggestion should not create pending tool call: %+v", res.Continuation.PendingToolCall)
	}
	if !strings.Contains(res.Reply, "余量风险") || !strings.Contains(res.Reply, "没有生成待确认修改") {
		t.Fatalf("headroom report should explain report-only status: %q", res.Reply)
	}
}

func TestMessageLoopA4ClipCleanupRoutesToAllProjectStripSilenceSuggest(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"bad old A4 route","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 5, MaxToolCalls: 4, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "鐢喗鍨滈幍褑顢慉4閻楀洦顔岀憗浣稿",
		AllowedTools: []string{"clip.strip_silence.suggest", "clip.strip_silence.analyze", "clip.strip_silence.apply", "clip.strip_silence.apply_batch", "mix.observe"},
	})

	if res.Status != "waiting_confirmation" || res.StopReason != StopReasonNeedsConfirmation {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(client.calls) != 0 {
		t.Fatalf("LLM should not be called for deterministic A4 clip cleanup, calls=%d", len(client.calls))
	}
	if len(exec.calls) != 1 || exec.calls[0].Tool != "clip.strip_silence.suggest" {
		t.Fatalf("executor calls = %+v, want clip.strip_silence.suggest only", exec.calls)
	}
	if firstMapText(exec.calls[0].Args, "scope") != "all_project" {
		t.Fatalf("suggest args = %+v, want all_project", exec.calls[0].Args)
	}
	if strings.Contains(res.Reply, "bad old A4 route") || !strings.Contains(res.Reply, "strip silence") {
		if res.Continuation == nil || res.Continuation.PendingToolCall == nil || res.Continuation.PendingToolCall.Tool != "clip.strip_silence.apply_batch" {
			t.Fatalf("pending continuation = %+v", res.Continuation)
		}
		pending := res.Continuation.PendingToolCall
		if rows := messageLoopMapRows(pending.Args["pending_actions"]); len(rows) != 2 {
			t.Fatalf("pending batch args = %+v", pending.Args)
		}
	}
}

func TestMessageLoopConfirmedStripSilenceApplyUsesPendingArgsAndCompletes(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"model should not be needed"}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 5, MaxToolCalls: 4, MaxConsecutiveErrors: 2},
	}
	pending := planner.ToolCall{
		ID:   "suggest_strip_silence_apply_01",
		Tool: "clip.strip_silence.apply",
		Args: map[string]any{
			"cmd":         "clip.strip_silence.apply",
			"clip_id":     "1011",
			"track_id":    "1007",
			"analysis_id": "analysis_test",
			"strip_regions": []any{map[string]any{
				"clip_id":       "1011",
				"track_id":      "1007",
				"start_seconds": 0.0,
				"end_seconds":   0.35,
			}},
		},
		Reason: "apply confirmed Strip Silence preview",
	}
	pending.Command = cloneMap(pending.Args)

	res := loop.ResumeAfterConfirmation(context.Background(), Continuation{
		GoalID:          "goal_strip_silence",
		RunID:           "run_strip_silence",
		UserText:        "approve",
		Summary:         "confirmed Strip Silence apply",
		AllowedTools:    []string{"clip.strip_silence.apply"},
		PendingToolCall: &pending,
	})

	if res.Status != "completed" || res.StopReason != StopReasonDone {
		t.Fatalf("result = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	if len(exec.calls) != 1 || exec.calls[0].Tool != "clip.strip_silence.apply" {
		t.Fatalf("executor calls = %+v, want confirmed clip.strip_silence.apply only", exec.calls)
	}
	if len(exec.confirmed) != 1 || !exec.confirmed[0] {
		t.Fatalf("confirmed flags = %+v, want one confirmed execution", exec.confirmed)
	}
	if len(messageLoopMapRows(exec.calls[0].Args["strip_regions"])) != 1 {
		t.Fatalf("confirmed apply lost strip_regions: %+v", exec.calls[0].Args)
	}
	if len(client.calls) != 0 {
		t.Fatalf("LLM should not be called after confirmed Strip Silence apply, calls=%d", len(client.calls))
	}
}

func TestMessageLoopStripSilencePendingActionRowsDeduplicatesSingularAndListAction(t *testing.T) {
	action := map[string]any{
		"tool_name": "clip.strip_silence.apply",
		"args": map[string]any{
			"clip_id":     "clip_a",
			"track_id":    "track_a",
			"analysis_id": "analysis_a",
			"strip_regions": []any{map[string]any{
				"clip_id":       "clip_a",
				"track_id":      "track_a",
				"start_seconds": 0.0,
				"end_seconds":   0.25,
			}},
		},
	}

	calls := messageLoopStripSilencePendingActionCalls(planner.ToolCall{ID: "suggest_strip"}, map[string]any{
		"pending_action":  action,
		"pending_actions": []any{action},
	})

	if len(calls) != 1 {
		t.Fatalf("duplicate pending action should produce one apply call, got %+v", calls)
	}
	if firstMapText(calls[0].Args, "clip_id") != "clip_a" || len(messageLoopMapRows(calls[0].Args["strip_regions"])) != 1 {
		t.Fatalf("deduped call lost args: %+v", calls[0])
	}
}

func TestMessageLoopStripSilenceSingleActionUsesBatchWhenSourceHasNoCleanupClips(t *testing.T) {
	action := map[string]any{
		"tool_name": "clip.strip_silence.apply",
		"args": map[string]any{
			"clip_id":     "clip_a",
			"track_id":    "track_a",
			"analysis_id": "analysis_a",
			"strip_regions": []any{map[string]any{
				"clip_id":       "clip_a",
				"track_id":      "track_a",
				"start_seconds": 0.0,
				"end_seconds":   0.25,
			}},
		},
	}

	call, ok := messageLoopStripSilencePendingConfirmationCall(planner.ToolCall{ID: "suggest_strip"}, executorpkg.Result{
		Result: map[string]any{
			"target_count":                 3,
			"analyzed_clip_count":          3,
			"no_cleanup_needed_clip_count": 2,
			"pending_actions":              []any{action},
		},
	})

	if !ok || call.Tool != "clip.strip_silence.apply_batch" {
		t.Fatalf("single all-project action should preserve aggregate stats via apply_batch: ok=%v call=%+v", ok, call)
	}
	if got := firstPositiveMapInt(call.Args, "scanned_clip_count"); got != 3 {
		t.Fatalf("scanned_clip_count = %d, args=%+v", got, call.Args)
	}
	if got := firstPositiveMapInt(call.Args, "no_cleanup_needed_clip_count"); got != 2 {
		t.Fatalf("no_cleanup_needed_clip_count = %d, args=%+v", got, call.Args)
	}
}

func TestMessageLoopConfirmedStripSilenceBundleAppliesAllPendingActions(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"model should not be needed"}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 5, MaxToolCalls: 6, MaxConsecutiveErrors: 2},
	}
	actionRows := []any{
		map[string]any{
			"tool_name": "clip.strip_silence.apply",
			"args": map[string]any{
				"clip_id":  "clip_a",
				"track_id": "track_a",
				"strip_regions": []any{map[string]any{
					"clip_id":       "clip_a",
					"track_id":      "track_a",
					"start_seconds": 0.0,
					"end_seconds":   0.25,
				}},
			},
		},
		map[string]any{
			"tool_name": "clip.strip_silence.apply",
			"args": map[string]any{
				"clip_id":  "clip_b",
				"track_id": "track_b",
				"strip_regions": []any{map[string]any{
					"clip_id":       "clip_b",
					"track_id":      "track_b",
					"start_seconds": 1.0,
					"end_seconds":   1.25,
				}},
			},
		},
	}
	pending := planner.ToolCall{
		ID:   "apply_strip_silence_bundle",
		Tool: "clip.strip_silence.apply",
		Args: map[string]any{
			"_strip_silence_pending_bundle": true,
			"pending_action_count":          2,
			"pending_actions":               actionRows,
		},
		Reason: "apply confirmed Strip Silence bundle",
	}
	pending.Command = cloneMap(pending.Args)

	res := loop.ResumeAfterConfirmation(context.Background(), Continuation{
		GoalID:          "goal_strip_silence_bundle",
		RunID:           "run_strip_silence_bundle",
		UserText:        "approve",
		Summary:         "confirmed Strip Silence bundle",
		AllowedTools:    []string{"clip.strip_silence.apply"},
		PendingToolCall: &pending,
	})

	if res.Status != "completed" || res.StopReason != StopReasonDone {
		t.Fatalf("result = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	if len(exec.calls) != 1 || exec.calls[0].Tool != "clip.strip_silence.apply_batch" {
		t.Fatalf("executor calls = %+v, want one confirmed apply_batch", exec.calls)
	}
	if len(exec.confirmed) != 1 || !exec.confirmed[0] {
		t.Fatalf("confirmed flags = %+v, want one confirmed execution", exec.confirmed)
	}
	if rows := messageLoopMapRows(exec.calls[0].Args["pending_actions"]); len(rows) != 2 {
		t.Fatalf("batch actions = %+v", exec.calls[0].Args)
	}
	if len(client.calls) != 0 {
		t.Fatalf("LLM should not be called after confirmed Strip Silence bundle, calls=%d", len(client.calls))
	}
}

func TestMessageLoopStripSilenceApplyBatchReplySeparatesNoCleanupSkipAndFailure(t *testing.T) {
	reply := messageLoopStripSilenceApplyCompleteReply(map[string]any{
		"result": map[string]any{
			"scanned_clip_count":           3,
			"applied_clip_count":           1,
			"applied_region_count":         7,
			"no_cleanup_needed_clip_count": 1,
			"protected_skip_clip_count":    1,
			"failed_clip_count":            0,
		},
	}, planner.ToolCall{
		Tool: "clip.strip_silence.apply_batch",
		Args: map[string]any{"cmd": "clip.strip_silence.apply_batch"},
	})

	for _, want := range []string{"scanned 3 clips", "cleaned 1", "7 silent regions", "1 no cleanup", "1 protected skip", "0 true failures"} {
		if !strings.Contains(reply, want) {
			t.Fatalf("reply %q missing %q", reply, want)
		}
	}
	if strings.Contains(reply, "execution failed") {
		t.Fatalf("reply should not call protected skips execution failures: %q", reply)
	}
}

func TestMessageLoopClipGainSetPreflightCreatesClipConfirmation(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"bad route","tool_calls":[{"id":"propose","tool":"mix.propose_tick","args":{"operation":"track_gain_adjust","track_id":"1007","delta_db":-1},"reason":"wrong route"}]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 5, MaxToolCalls: 4, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "set current clip gain to -3 dB",
		AllowedTools: []string{"clip.gain.set", "mix.propose_tick", "mix.apply_tick"},
		Context: map[string]any{
			"ui_context": map[string]any{
				"selected_clip_id":       "1011",
				"selected_clip_track_id": "1007",
			},
		},
	})

	if res.Status != "waiting_confirmation" || res.StopReason != StopReasonNeedsConfirmation {
		t.Fatalf("result = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	if len(client.calls) != 0 {
		t.Fatalf("LLM should not be called for deterministic clip gain set, calls=%d", len(client.calls))
	}
	if len(exec.calls) != 1 || exec.calls[0].Tool != "clip.gain.set" {
		t.Fatalf("executor calls = %+v, want clip.gain.set only", exec.calls)
	}
	if firstMapText(exec.calls[0].Args, "clip_id") != "1011" {
		t.Fatalf("clip.gain.set args = %+v", exec.calls[0].Args)
	}
	if gainDB, ok := firstNumericMapValue(exec.calls[0].Args, "gain_db"); !ok || gainDB != -3 {
		t.Fatalf("clip.gain.set gain_db = %v ok=%v args=%+v", gainDB, ok, exec.calls[0].Args)
	}
	if res.Continuation == nil || res.Continuation.PendingToolCall == nil || res.Continuation.PendingToolCall.Tool != "clip.gain.set" {
		t.Fatalf("pending continuation = %+v", res.Continuation)
	}
	if res.ExecutionMemory.PendingMixTickCandidate != nil || res.ExecutionMemory.PendingMixTreatment != nil {
		t.Fatalf("clip gain set created pending mix memory: %+v", res.ExecutionMemory)
	}
}

func TestMessageLoopClipFadeSetPreflightCreatesClipConfirmation(t *testing.T) {
	userText := "\u628a\u5f53\u524d\u9009\u4e2d clip \u7684 fade in \u8bbe\u7f6e\u4e3a 0.15 \u79d2\uff0cfade out \u8bbe\u7f6e\u4e3a 0.25 \u79d2"
	if messageLoopClipFadeGainReadRequest(userText) {
		t.Fatal("fade write request should not be classified as a read")
	}
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"bad route","tool_calls":[{"id":"read_fade","tool":"clip.fade.read","args":{"clip_id":"1011"},"reason":"wrong read route"}]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 5, MaxToolCalls: 4, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     userText,
		AllowedTools: []string{"clip.fade.set", "clip.fade.read", "clip.gain.read", "mix.propose_tick", "mix.apply_tick"},
		Context: map[string]any{
			"ui_context": map[string]any{
				"selected_clip_id":       "1011",
				"selected_clip_track_id": "1007",
			},
		},
	})

	if res.Status != "waiting_confirmation" || res.StopReason != StopReasonNeedsConfirmation {
		t.Fatalf("result = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	if len(client.calls) != 0 {
		t.Fatalf("LLM should not be called for deterministic clip fade set, calls=%d", len(client.calls))
	}
	if len(exec.calls) != 1 || exec.calls[0].Tool != "clip.fade.set" {
		t.Fatalf("executor calls = %+v, want clip.fade.set only", exec.calls)
	}
	if firstMapText(exec.calls[0].Args, "clip_id") != "1011" {
		t.Fatalf("clip.fade.set args = %+v", exec.calls[0].Args)
	}
	if fadeIn, ok := firstNumericMapValue(exec.calls[0].Args, "fade_in_seconds"); !ok || fadeIn != 0.15 {
		t.Fatalf("clip.fade.set fade_in_seconds = %v ok=%v args=%+v", fadeIn, ok, exec.calls[0].Args)
	}
	if fadeOut, ok := firstNumericMapValue(exec.calls[0].Args, "fade_out_seconds"); !ok || fadeOut != 0.25 {
		t.Fatalf("clip.fade.set fade_out_seconds = %v ok=%v args=%+v", fadeOut, ok, exec.calls[0].Args)
	}
	if res.Continuation == nil || res.Continuation.PendingToolCall == nil || res.Continuation.PendingToolCall.Tool != "clip.fade.set" {
		t.Fatalf("pending continuation = %+v", res.Continuation)
	}
	if res.ExecutionMemory.PendingMixTickCandidate != nil || res.ExecutionMemory.PendingMixTreatment != nil {
		t.Fatalf("clip fade set created pending mix memory: %+v", res.ExecutionMemory)
	}
}

func TestMessageLoopConfirmedClipFadeSetDoesNotRequeueConfirmation(t *testing.T) {
	userText := "\u628a\u5f53\u524d\u9009\u4e2d clip \u7684 fade in \u8bbe\u7f6e\u4e3a 0.15 \u79d2\uff0cfade out \u8bbe\u7f6e\u4e3a 0.25 \u79d2"
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"model should not be needed"}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 5, MaxToolCalls: 4, MaxConsecutiveErrors: 2},
	}
	pending := planner.ToolCall{
		ID:   "set_clip_fade",
		Tool: "clip.fade.set",
		Args: map[string]any{
			"cmd":              "clip.fade.set",
			"clip_id":          "1011",
			"fade_in_seconds":  0.15,
			"fade_out_seconds": 0.25,
		},
		Command: map[string]any{
			"cmd":              "clip.fade.set",
			"clip_id":          "1011",
			"fade_in_seconds":  0.15,
			"fade_out_seconds": 0.25,
		},
		Reason: "Set selected clip fade in 0.150s, fade out 0.250s.",
	}

	res := loop.ResumeAfterConfirmation(context.Background(), Continuation{
		GoalID:       "goal_clip_fade",
		RunID:        "run_clip_fade",
		UserText:     userText,
		Summary:      userText,
		AllowedTools: []string{"clip.fade.set", "clip.fade.read", "clip.gain.read", "mix.propose_tick", "mix.apply_tick"},
		Context: map[string]any{
			"ui_context": map[string]any{
				"selected_clip_id":       "1011",
				"selected_clip_track_id": "1007",
			},
		},
		PendingToolCall: &pending,
	})

	if res.Status != "completed" || res.StopReason != StopReasonDone {
		t.Fatalf("result = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	if len(exec.calls) != 1 || exec.calls[0].Tool != "clip.fade.set" {
		t.Fatalf("executor calls = %+v, want confirmed clip.fade.set only", exec.calls)
	}
	if len(exec.confirmed) != 1 || !exec.confirmed[0] {
		t.Fatalf("confirmed flags = %+v, want one confirmed execution", exec.confirmed)
	}
	if strings.Contains(strings.ToLower(res.StopReason), "confirmation") {
		t.Fatalf("confirmed clip fade set requeued confirmation: %+v", res)
	}
	if res.Continuation != nil && res.Continuation.PendingToolCall != nil {
		t.Fatalf("completed result should not keep pending tool call: %+v", res.Continuation.PendingToolCall)
	}
	if len(client.calls) != 0 {
		t.Fatalf("LLM should not be called after confirmed deterministic clip fade set, calls=%d", len(client.calls))
	}
}

func messageLoopTestStringSliceContains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func TestMessageLoopReadOnlyObservationSuppressesTreatmentMarker(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"Observation is ready.\nmix_treatment_pending: {\"schema_version\":\"mix_treatment_pending.v0\",\"status\":\"pending_confirmation\",\"intent\":\"move guitar left\",\"target_ref\":\"track:1007\",\"action_kind\":\"pan_balance\",\"processor_type\":\"utility\",\"delta_pan\":-0.1,\"confidence\":\"high\",\"evidence_refs\":[\"observation.digest\"],\"needs_resolution\":[],\"expires_after_context_change\":true}","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 3, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "observe only: check stereo image status and phase correlation; no changes.",
		AllowedTools: []string{"mix.observe", "mix.read", "mix.derive", "mix.request_observation"},
		State: map[string]any{"tracks": []map[string]any{{
			"track_id": "1007",
		}}},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if res.ExecutionMemory.PendingMixTreatment != nil || res.ExecutionMemory.PendingMixTickCandidate != nil {
		t.Fatalf("read-only turn created pending: %+v", res.ExecutionMemory)
	}
	if strings.Contains(res.Reply, "mix_treatment_pending") {
		t.Fatalf("reply leaked treatment marker: %q", res.Reply)
	}
}

func TestMessageLoopReadOnlyObservationBlocksMixTickTool(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"I will observe, then propose a tick.","tool_calls":[{"id":"observe","tool":"mix.observe","args":{"scope":"full_project"},"reason":"read-only observation"},{"id":"propose","tool":"mix.propose_tick","args":{"operation":"track_gain_adjust","track_id":"1010","delta_db":-1},"reason":"small move"}]}`,
		`{"final":true,"reply":"Observation summary only.","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 5, MaxToolCalls: 4, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "analysis only: observe the mix loudness and stereo status, do not execute.",
		AllowedTools: []string{"mix.observe", "mix.read", "mix.derive", "mix.request_observation", "mix.propose_tick", "mix.apply_tick"},
		State: map[string]any{"tracks": []map[string]any{{
			"track_id": "1007",
		}}},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(exec.calls) != 1 || exec.calls[0].Tool != "mix.observe" {
		t.Fatalf("executor calls = %+v, want read-only guard to block mix.propose_tick", exec.calls)
	}
	if res.ExecutionMemory.PendingMixTreatment != nil || res.ExecutionMemory.PendingMixTickCandidate != nil {
		t.Fatalf("read-only turn created pending: %+v", res.ExecutionMemory)
	}
}

func TestMessageLoopMixExecutionQuestionIsNotClarification(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"needs_clarification":true,"reply":"Should I lower Track 2 by 1 dB?","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "overall mix feels messy, help me fix it",
		AllowedTools: []string{"mix.observe", "mix.request_observation"},
		State: map[string]any{"tracks": []map[string]any{{
			"track_id":         "1007",
			"track_name":       "Track 1",
			"user_track_index": 1,
		}, {
			"track_id":         "1010",
			"track_name":       "Track 2",
			"user_track_index": 2,
		}}},
	})

	if res.Status != "completed" || res.StopReason != StopReasonDone {
		t.Fatalf("result = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	if len(exec.calls) != 1 || exec.calls[0].Tool != "mix.observe" {
		t.Fatalf("executor calls = %+v, want deterministic mix.observe", exec.calls)
	}
	candidate := res.ExecutionMemory.PendingMixTickCandidate
	if candidate == nil || candidate.Operation != "track_gain_adjust" || candidate.TrackID != "1010" || candidate.DeltaDB != -1 {
		t.Fatalf("pending candidate = %+v", candidate)
	}
	if !strings.Contains(strings.ToLower(res.Reply), "should i lower track 2") {
		t.Fatalf("reply = %q", res.Reply)
	}
}

func TestMessageLoopPanAmountClarificationUsesSmallStepDefault(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"needs_clarification":true,"reply":"How far left should Track 2 move? -10%, -20%, or a target pan value?","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "Track 2 pan left a little",
		AllowedTools: []string{"mix.observe", "mix.request_observation"},
		State: map[string]any{"tracks": []map[string]any{{
			"track_id":         "1007",
			"track_name":       "Track 1",
			"user_track_index": 1,
		}, {
			"track_id":         "1010",
			"track_name":       "Track 2",
			"user_track_index": 2,
		}}},
	})

	if res.Status != "completed" || res.StopReason != StopReasonDone {
		t.Fatalf("result = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	if len(exec.calls) != 1 || exec.calls[0].Tool != "mix.observe" {
		t.Fatalf("executor calls = %+v, want deterministic mix.observe", exec.calls)
	}
	treatment := res.ExecutionMemory.PendingMixTreatment
	if treatment == nil || treatment.ActionKind != "pan_balance" || treatment.TargetRef != "track:1010" || treatment.DeltaPan != -0.1 {
		t.Fatalf("pending treatment = %+v", treatment)
	}
}

func TestMessageLoopMixObserveAddsScopeFromIntent(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"閹存垵鍘涚憴鍌氱檪閺佺繝缍嬪ǎ鐑界叾閵?,"tool_calls":[{"id":"observe_mix","tool":"mix.request_observation","args":{},"reason":"閸忓牆缂撶粩瀣紣缁嬪顫囩€电喍绗傛稉瀣瀮"}]}`,
		`{"final":true,"reply":"鐟欏倸鐧傜€瑰本鍨氶敍灞惧灉娴兼艾鐔€娴滃孩鏆ｆ担鎾充紣缁嬪鏆熼幑顔剧舶閸戝搫缂撶拋顔衡偓?,"tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "Help me inspect the overall mix",
		AllowedTools: []string{"mix.observe", "mix.read", "mix.derive", "mix.request_observation"},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(exec.calls) != 1 || exec.calls[0].Tool != "mix.observe" {
		t.Fatalf("executor calls = %+v, want mix.observe", exec.calls)
	}
	if got := fmt.Sprint(exec.calls[0].Args["scope"]); got != "full_project" {
		t.Fatalf("scope = %q, want full_project; args=%+v", got, exec.calls[0].Args)
	}
	if got := fmt.Sprint(exec.calls[0].Args["disclosure"]); got != "digest_catalog" {
		t.Fatalf("disclosure = %q, want digest_catalog; args=%+v", got, exec.calls[0].Args)
	}
	if exec.calls[0].Args["observation_only"] != true {
		t.Fatalf("observation_only not set: %+v", exec.calls[0].Args)
	}
}

func TestMessageLoopMixObserveOverridesTrackScopeForMultitrackIntent(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"I will compare track relationships first.","tool_calls":[{"id":"observe_mix","tool":"mix.observe","args":{"scope":"track","track_id":"1012"},"reason":"Observe current track first"}]}`,
		`{"final":true,"reply":"Observation complete.","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "Compare all tracks frequency occupancy and stereo relationship without changes",
		AllowedTools: []string{"mix.observe", "mix.read", "mix.derive", "mix.request_observation"},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(exec.calls) != 1 || exec.calls[0].Tool != "mix.observe" {
		t.Fatalf("executor calls = %+v, want mix.observe", exec.calls)
	}
	if got := fmt.Sprint(exec.calls[0].Args["scope"]); got != "full_project" {
		t.Fatalf("scope = %q, want full_project; args=%+v", got, exec.calls[0].Args)
	}
	if got := fmt.Sprint(exec.calls[0].Args["goal_text"]); got != "Compare all tracks frequency occupancy and stereo relationship without changes" {
		t.Fatalf("goal_text = %q; args=%+v", got, exec.calls[0].Args)
	}
	if got := fmt.Sprint(exec.calls[0].Args["target_scope"]); got != "full_project" {
		t.Fatalf("target_scope = %q, want full_project; args=%+v", got, exec.calls[0].Args)
	}
	if got := fmt.Sprint(exec.calls[0].Args["mom_intent"]); got != "project_multitrack_relation_observation" {
		t.Fatalf("mom_intent = %q, want project_multitrack_relation_observation; args=%+v", got, exec.calls[0].Args)
	}
	if got := fmt.Sprint(exec.calls[0].Args["requested_layer"]); got != "project_multitrack_relation" {
		t.Fatalf("requested_layer = %q, want project_multitrack_relation; args=%+v", got, exec.calls[0].Args)
	}
}

func TestMessageLoopVocalForwardRequestAddsFocusScopeAndHint(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"I will observe the vocal in project context first.","tool_calls":[{"id":"observe_mix","tool":"mix.observe","args":{},"reason":"observe focus relationship before suggesting a move"}]}`,
		`{"final":true,"reply":"The vocal relationship observation is ready. I can suggest a small move after confirmation.","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "make the lead vocal more forward",
		AllowedTools: []string{"mix.observe", "mix.read", "mix.derive", "mix.request_observation"},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(exec.calls) != 2 || exec.calls[0].Tool != "mix.observe" || exec.calls[1].Tool != "mix.derive" {
		t.Fatalf("executor calls = %+v, want mix.observe then mix.derive", exec.calls)
	}
	if got := fmt.Sprint(exec.calls[0].Args["scope"]); got != "full_project_with_focus_track" {
		t.Fatalf("scope = %q, want full_project_with_focus_track; args=%+v", got, exec.calls[0].Args)
	}
	focusHint := messageLoopMapValue(exec.calls[0].Args["focus_hint"])
	if focusHint["role"] != "vocal" || focusHint["source"] != "user_intent" {
		t.Fatalf("focus hint = %+v; args=%+v", focusHint, exec.calls[0].Args)
	}
	if exec.calls[0].Args["observation_only"] != true {
		t.Fatalf("observation_only not set: %+v", exec.calls[0].Args)
	}
	if got := fmt.Sprint(exec.calls[1].Args["type"]); got != "focus_vs_project" {
		t.Fatalf("derive type = %q; args=%+v", got, exec.calls[1].Args)
	}
	deriveFocus := messageLoopMapValue(exec.calls[1].Args["focus"])
	if deriveFocus["role"] != "vocal" {
		t.Fatalf("derive focus = %+v; args=%+v", deriveFocus, exec.calls[1].Args)
	}
	if got := fmt.Sprint(exec.calls[1].Args["observation_id"]); got != "obs_test" {
		t.Fatalf("derive observation_id = %q; args=%+v", got, exec.calls[1].Args)
	}
	if res.ExecutionMemory.PendingMixTickCandidate != nil {
		t.Fatalf("relationship observation should not synthesize a pending gain tick without a concrete dB suggestion: %+v", res.ExecutionMemory.PendingMixTickCandidate)
	}
}

func TestMessageLoopVocalForwardBlockedActionPreflightStillDerivesWithCommandAllowedTools(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"The vocal relationship observation is ready. I can suggest a small move after confirmation.","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{mixObservationResult: map[string]any{
		"status":         "ready",
		"observation_id": "obs_action_gate",
		"mix_session_id": "mix_action_gate",
		"digest": map[string]any{
			"scope": "full_project_with_focus_track",
		},
		"acoustic_package_status": map[string]any{
			"status": "ready",
			"package_layers": map[string]any{
				"l1_static": map[string]any{
					"status": "ready",
					"features": map[string]any{
						"waveform_envelope": map[string]any{"status": "ready"},
					},
				},
			},
		},
		"mom_projection": map[string]any{
			"mom_version":    "v1.4",
			"intent":         "action_preflight_observation",
			"observation_id": "obs_action_gate",
			"mix_session_id": "mix_action_gate",
			"evidence_refs":  []any{"observation:obs_action_gate"},
			"trust_quality": map[string]any{
				"overall_status":               "partial",
				"can_support_suggestion":       true,
				"can_support_action_preflight": false,
				"evidence_refs":                []any{"observation:obs_action_gate"},
				"blocked_reasons":              []any{"action_preflight_requires_ready_band_stereo_evidence"},
				"limitations":                  []any{"l2_tap_point_not_fully_closed_loop"},
			},
		},
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 3, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "make the lead vocal more forward",
		AllowedTools: []string{"mix_observe", "mix_read", "mix_derive", "mix_request_observation"},
	})

	if res.Status != "completed" || res.StopReason != StopReasonDone {
		t.Fatalf("result = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	if len(exec.calls) != 2 || exec.calls[0].Tool != "mix.observe" || exec.calls[1].Tool != "mix.derive" {
		t.Fatalf("executor calls = %+v, want mix.observe then mix.derive", exec.calls)
	}
	if got := fmt.Sprint(exec.calls[1].Args["type"]); got != "focus_vs_project" {
		t.Fatalf("derive type = %q; args=%+v", got, exec.calls[1].Args)
	}
	if got := fmt.Sprint(exec.calls[1].Args["observation_id"]); got != "obs_action_gate" {
		t.Fatalf("derive observation_id = %q; args=%+v", got, exec.calls[1].Args)
	}
	if strings.Contains(res.Reply, "action_preflight_requires_ready_band_stereo_evidence") {
		t.Fatalf("reply should not be the blocked action-preflight fallback before relationship derive: %q", res.Reply)
	}
}

func TestMessageLoopVocalForwardPendingFeatureObservationStillDerives(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"I will observe the vocal in project context first.","tool_calls":[{"id":"observe_mix","tool":"mix.observe","args":{},"reason":"observe focus relationship before suggesting a move"}]}`,
		`{"final":true,"reply":"I need the lead vocal identity before proposing a move.","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{mixObservationResult: map[string]any{
		"status":         "partial",
		"observation_id": "obs_pending_features",
		"mix_session_id": "mix_pending_features",
		"mixboard": map[string]any{
			"status":        "partial",
			"open_blockers": []any{"audio_feature_request_pending"},
			"package_status": map[string]any{
				"mix":     "limited",
				"project": "ready",
				"deep":    "async_available",
			},
		},
		"observation": map[string]any{
			"status": "partial",
			"target_ref": map[string]any{
				"kind": "project",
				"id":   "current",
			},
		},
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "make the lead vocal more forward",
		AllowedTools: []string{"mix.observe", "mix.read", "mix.derive", "mix.request_observation"},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(exec.calls) != 2 || exec.calls[0].Tool != "mix.observe" || exec.calls[1].Tool != "mix.derive" {
		t.Fatalf("executor calls = %+v, want mix.observe then mix.derive", exec.calls)
	}
	if got := fmt.Sprint(exec.calls[1].Args["observation_id"]); got != "obs_pending_features" {
		t.Fatalf("derive observation_id = %q; args=%+v", got, exec.calls[1].Args)
	}
}

func TestMessageLoopVocalForwardClarificationAddsFocusTrackIndexHint(t *testing.T) {
	args := messageLoopMixObservationArgs("Make the lead vocal more forward\n\nUser clarification: Track 1 is the lead vocal", map[string]any{})
	focusHint := messageLoopMapValue(args["focus_hint"])
	if got := focusHint["role"]; got != "vocal" {
		t.Fatalf("focus hint = %+v", focusHint)
	}
	if got := fmt.Sprint(focusHint["user_track_index"]); got != "1" {
		t.Fatalf("focus hint index = %q; hint=%+v", got, focusHint)
	}
	if got := focusHint["source"]; got != "user_clarification" {
		t.Fatalf("focus hint source = %+v", focusHint)
	}
	if got := fmt.Sprint(args["scope"]); got != "full_project_with_focus_track" {
		t.Fatalf("scope = %q; args=%+v", got, args)
	}
}

func TestMessageLoopVocalForwardClarificationDoesNotStorePendingTick(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"needs_clarification":true,"reply":"Do you want me to make the lead vocal feel more forward by lowering the competing Track 2 by 1 dB?","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{mixObservationResult: map[string]any{
		"status":         "ok",
		"mix_session_id": "mix_focus",
		"observation_id": "obs_focus",
		"digest": map[string]any{
			"scope": "full_project_with_focus_track",
			"target": map[string]any{
				"kind": "project",
				"id":   "current",
			},
		},
		"observation": map[string]any{
			"target_ref": map[string]any{"kind": "project", "id": "current", "label": "Current project"},
			"project_package": map[string]any{
				"track_count":                 2,
				"active_acoustic_track_count": 2,
				"tracks": []map[string]any{{
					"track_id":         "1007",
					"name":             "Lead Vocal",
					"track_name":       "Lead Vocal",
					"user_label":       "Lead Vocal",
					"user_track_index": 1,
					"role_guess":       "vocal",
					"volume_db":        0,
					"rms_dbfs":         -18.0,
					"peak_dbfs":        -2.0,
					"headroom_db":      2.0,
				}, {
					"track_id":         "1012",
					"name":             "Track 2",
					"track_name":       "Track 2",
					"user_label":       "Track 2",
					"user_track_index": 2,
					"volume_db":        0,
					"rms_dbfs":         -14.0,
					"peak_dbfs":        -0.5,
					"headroom_db":      0.5,
				}},
			},
		},
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "make the lead vocal more forward",
		AllowedTools: []string{"mix.observe", "mix.read", "mix.derive", "mix.request_observation"},
	})

	if res.Status != "waiting_clarification" || !res.NeedsClarification {
		t.Fatalf("result = status=%q needs=%v reply=%q error=%q", res.Status, res.NeedsClarification, res.Reply, res.Error)
	}
	if len(exec.calls) != 2 || exec.calls[0].Tool != "mix.observe" || exec.calls[1].Tool != "mix.derive" {
		t.Fatalf("executor calls = %+v, want mix.observe then mix.derive", exec.calls)
	}
	if candidate := res.ExecutionMemory.PendingMixTickCandidate; candidate != nil {
		t.Fatalf("clarification turn must not store a pending candidate: %+v", candidate)
	}
	if !strings.Contains(strings.ToLower(res.Reply), "track") {
		t.Fatalf("clarification reply should ask for the vocal track, got %q", res.Reply)
	}
}

func TestMessageLoopVocalForwardFinalSuggestionWithoutResolvedVocalAsksClarification(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"Track 1 is masking the lead vocal. I suggest lowering Track 1 by 1.5 dB. Do you want me to continue?","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{mixObservationResult: map[string]any{
		"status":         "ok",
		"mix_session_id": "mix_focus",
		"observation_id": "obs_focus",
		"digest": map[string]any{
			"scope": "full_project_with_focus_track",
			"target": map[string]any{
				"kind": "project",
				"id":   "current",
			},
		},
		"observation": map[string]any{
			"target_ref": map[string]any{"kind": "project", "id": "current", "label": "Current project"},
			"project_package": map[string]any{
				"track_count":                 2,
				"active_acoustic_track_count": 2,
				"tracks": []map[string]any{{
					"track_id":         "1007",
					"name":             "Track 1",
					"track_name":       "Track 1",
					"user_label":       "Track 1",
					"user_track_index": 1,
					"volume_db":        0,
					"rms_dbfs":         -10.0,
					"peak_dbfs":        -0.3,
					"headroom_db":      0.3,
				}, {
					"track_id":         "1012",
					"name":             "Track 2",
					"track_name":       "Track 2",
					"user_label":       "Track 2",
					"user_track_index": 2,
					"volume_db":        0,
					"rms_dbfs":         -14.0,
					"peak_dbfs":        -2.5,
					"headroom_db":      2.5,
				}},
			},
		},
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "make the lead vocal more forward",
		AllowedTools: []string{"mix.observe", "mix.read", "mix.derive", "mix.request_observation"},
	})

	if res.Status != "waiting_clarification" || !res.NeedsClarification {
		t.Fatalf("result = status=%q needs=%v reply=%q error=%q", res.Status, res.NeedsClarification, res.Reply, res.Error)
	}
	if len(exec.calls) != 2 || exec.calls[0].Tool != "mix.observe" || exec.calls[1].Tool != "mix.derive" {
		t.Fatalf("executor calls = %+v, want mix.observe then mix.derive", exec.calls)
	}
	if candidate := res.ExecutionMemory.PendingMixTickCandidate; candidate != nil {
		t.Fatalf("unresolved vocal target must not store a pending candidate: %+v", candidate)
	}
	if !strings.Contains(strings.ToLower(res.Reply), "track") {
		t.Fatalf("reply should ask which track is lead vocal, got %q", res.Reply)
	}
	if res.Continuation == nil {
		t.Fatalf("clarification should preserve continuation so the user's answer can resume the goal")
	}
}

func TestMessageLoopExtractsGainDeltaAfterTrackMention(t *testing.T) {
	delta, evidence, ok := messageLoopExtractSingleGainDelta("Do you want me to make the lead vocal feel more forward by lowering the competing Track 2 by 1 dB?")
	if !ok || delta != -1 || evidence != "1 dB" {
		t.Fatalf("delta=%v evidence=%q ok=%v", delta, evidence, ok)
	}
}

func TestMessageLoopExtractsGainDeltaFromChineseMetricsAndRepeatedConfirmation(t *testing.T) {
	reply := "Track 1 RMS is -9.0 dBFS and Track 2 is lower by 1.6 dB. I suggest lowering Track 2 by 1.0 dB."

	delta, evidence, ok := messageLoopExtractSingleGainDelta(reply)
	if !ok || delta != -1 || evidence != "1.0 dB" {
		t.Fatalf("delta=%v evidence=%q ok=%v", delta, evidence, ok)
	}
}

func TestMessageLoopExtractsActionableGainDeltaWhenReplyHasMetricAndHistoryDB(t *testing.T) {
	reply := "Overall peak is around 0 dBFS and RMS is around -10.6 dBFS. Track 1 was already raised by +1 dB, so it is more forward. Suggested next step: lower Track 2 by 1 dB to create headroom. Should I execute this -1 dB move?"

	delta, evidence, ok := messageLoopExtractSingleGainDelta(reply)
	if !ok || delta != -1 {
		t.Fatalf("delta=%v evidence=%q ok=%v", delta, evidence, ok)
	}
	if !strings.Contains(evidence, "1 dB") {
		t.Fatalf("evidence=%q", evidence)
	}
}

func TestMessageLoopPendingCandidateUsesRelationshipTracksAfterVocalClarification(t *testing.T) {
	reply := "Track 1 is the lead vocal. Track 1 RMS is about -9.0 dBFS and Track 2 is lower, but Track 2 is peaking near 0 dBFS and has the tightest headroom. I suggest lowering Track 2 by 1.0 dB first. Execute Track 2 down 1.0 dB?"
	state := &runState{
		input: Input{UserText: "Make the lead vocal more forward\n\nUser clarification: Track 1 is the lead vocal"},
		executed: []map[string]any{{
			"tool":         "mix.derive",
			"command_name": "mix_derive",
			"status":       "ok",
			"result": map[string]any{
				"status":         "ready",
				"observation_id": "obs_focus",
				"mix_session_id": "mix_focus",
				"relationship": map[string]any{
					"focus_track": map[string]any{
						"track_id":         "1009",
						"name":             "Track 1",
						"track_name":       "Track 1",
						"user_track_index": 1,
						"rms_dbfs":         -9.032,
						"peak_dbfs":        -6.021,
					},
					"peer_track": map[string]any{
						"track_id":         "1010",
						"name":             "Track 2",
						"track_name":       "Track 2",
						"user_track_index": 2,
						"volume_db":        0,
						"rms_dbfs":         -10.603,
						"peak_dbfs":        0,
					},
				},
			},
		}},
	}

	candidate := messageLoopPendingMixTickCandidateFromReply(state, reply)
	if candidate == nil {
		t.Fatalf("pending candidate missing")
	}
	if candidate.TrackID != "1010" || candidate.Operation != "track_gain_adjust" || candidate.DeltaDB != -1 {
		t.Fatalf("candidate = %+v", candidate)
	}
	if candidate.Fingerprint["track_count"] != 2 {
		t.Fatalf("fingerprint = %+v", candidate.Fingerprint)
	}
}

func TestMessageLoopTrackIDNearestToGainEvidenceIgnoresFocusMention(t *testing.T) {
	rows := []map[string]any{{
		"track_id":         "1007",
		"name":             "Lead Vocal",
		"track_name":       "Lead Vocal",
		"user_track_index": 1,
	}, {
		"track_id":         "1012",
		"name":             "Track 2",
		"track_name":       "Track 2",
		"user_track_index": 2,
	}}
	reply := "Do you want me to make the lead vocal feel more forward by lowering the competing Track 2 by 1 dB?"

	if got := messageLoopTrackIDNearestToEvidence(rows, reply, "1 dB"); got != "1012" {
		t.Fatalf("nearest track = %q, want 1012", got)
	}
	if got := messageLoopTrackIDMentionedOnce(rows, reply); got != "" {
		t.Fatalf("whole reply should remain ambiguous, got %q", got)
	}
}

func TestMessageLoopNaturalMixRequestDoesNotWriteAfterObservationWithoutExplicitConfirmation(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"I will observe the current audio first.","tool_calls":[{"id":"observe_mix","tool":"mix.request_observation","args":{"track_id":"1007"},"reason":"Observe before deciding"}]}`,
		`{"final":false,"reply":"鐟欏倸鐧傜€瑰本鍨氶敍灞惧灉閸戝棗顦幎濠囩叾闁插繐鐨獮鍛絹閸?2 dB閵?,"tool_calls":[{"id":"raise_volume","tool":"track.volume","args":{"track_id":"1007","db":2},"reason":"閹绘劙鐝崫宥呭"}]}`,
		`{"final":true,"reply":"閹存垵鍑＄€瑰本鍨氱憴鍌氱檪閿涘苯缂撶拋顔煎帥閹跺﹨绻栭弶陇寤洪柆鎾逛氦瀵邦喗褰佹禍顔藉灗婢х偟娉弫瀵告倞閿涙稑顩ч弸婊€缍樼涵顔款吇閿涘本鍨滈崘宥嗗⒔鐞涘苯鍙挎担鎾茬濮濄儯鈧?,"tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 5, MaxToolCalls: 3, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "鐢喗鍨滅紓鈺傝穿闁鑵戞潪銊╀壕",
		AllowedTools: []string{"track.volume", "mix.request_observation"},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(exec.calls) != 1 || !isTestMixObservationTool(exec.calls[0].Tool) {
		t.Fatalf("executor calls = %+v, want only mix observation before confirmation", exec.calls)
	}
	for _, call := range exec.calls {
		if call.Tool == "track.volume" {
			t.Fatalf("broad mix request should not write after observation without explicit confirmation: %+v", exec.calls)
		}
	}
	foundGate := false
	for _, event := range res.Trace {
		if event.Kind == "final_gate" && strings.Contains(event.Message, "explicit user confirmation") {
			foundGate = true
			break
		}
	}
	if !foundGate {
		t.Fatalf("expected post-observation confirmation gate; trace=%+v", res.Trace)
	}
}

func TestParseMessageLoopOutputRepairsBareNewlinesInReply(t *testing.T) {
	raw := "{\"final\":true,\"reply\":\"line one\nline two\",\"tool_calls\":[]}"
	out, err := parseMessageLoopOutput(raw)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if out.Reply != "line one\nline two" {
		t.Fatalf("reply = %q", out.Reply)
	}
}

func TestMessageLoopConfirmedTrackVolumeBecomesPendingTreatment(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"observe first","tool_calls":[{"id":"observe_mix","tool":"mix.request_observation","args":{"track_id":"1007"},"reason":"observe before action"}]}`,
		`{"final":false,"reply":"confirmed lower 1 dB","tool_calls":[{"id":"raise_volume","tool":"track.volume","args":{"track_id":"1007","db":-1},"reason":"lower 1 dB"}]}`,
		`{"final":true,"reply":"done","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{mixObservationResult: map[string]any{
		"status":   "ok",
		"track_id": "1007",
		"summary":  "ready",
		"tracks": []map[string]any{{
			"track_id":  "1007",
			"volume_db": 0,
		}},
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 5, MaxToolCalls: 3, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "绾喛顓婚幍褑顢戦梽宥勭秵 1 dB",
		AllowedTools: []string{"track.volume", "mix.request_observation", "mix.propose_tick", "mix.apply_tick"},
	})

	if res.Status != "completed" || res.StopReason != StopReasonDone {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	for _, call := range exec.calls {
		if call.Tool == "mix.propose_tick" {
			t.Fatalf("natural-language mix tick proposal should become pending treatment before generic confirmation, calls=%+v", exec.calls)
		}
		if call.Tool == "mix.apply_tick" {
			t.Fatalf("mix tick proposal must wait for explicit pending treatment confirmation before apply, calls=%+v", exec.calls)
		}
		if call.Tool == "track.volume" {
			t.Fatalf("primitive track.volume should be wrapped through mix tick proposal, calls=%+v", exec.calls)
		}
	}
	if len(exec.calls) != 1 || !isTestMixObservationTool(exec.calls[0].Tool) {
		t.Fatalf("executor calls = %+v, want only observation before pending treatment", exec.calls)
	}
	treatment := res.ExecutionMemory.PendingMixTreatment
	if treatment == nil || treatment.ActionKind != "gain_balance" || treatment.DeltaDB != -1 || treatment.TargetRef != "track:1007" {
		t.Fatalf("pending treatment = %+v", treatment)
	}
}

func TestMessageLoopImplicitLowerVolumeWithoutDBBecomesPendingTreatment(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"閹存垵鍣径鍥х毈楠炲懎甯囨担搴＄秼閸撳秷寤洪柆鎾扁偓?,"tool_calls":[{"id":"lower_volume","tool":"track.volume","args":{"track_id":"1007"},"reason":"閻劍鍩涚拠纾嬬箹閺壜ゅ缓闁挸銇婇崫宥忕礉缁嬪秴浜曢崢瀣╃秵娑撯偓閻?}]}`,
		`{"final":true,"reply":"done","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{mixObservationResult: map[string]any{
		"status":         "ok",
		"track_id":       "1007",
		"observation_id": "obs_test",
		"mix_session_id": "mix_test",
		"summary":        "ready",
		"observation": map[string]any{
			"target_ref": map[string]any{"kind": "track", "id": "1007", "label": "Track 1"},
		},
		"tracks": []map[string]any{{
			"track_id":  "1007",
			"volume_db": 0,
		}},
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 5, MaxToolCalls: 3, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "This track is too loud, lower it a bit",
		AllowedTools: []string{"track.volume", "mix.request_observation", "mix.propose_tick", "mix.apply_tick"},
		Context:      map[string]any{"selected_track_id": "1007"},
		State:        map[string]any{"selected_track_id": "1007"},
	})

	if res.Status != "completed" || res.StopReason != StopReasonDone {
		t.Fatalf("result = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	for _, call := range exec.calls {
		switch call.Tool {
		case "track.volume":
			t.Fatalf("implicit volume request must not execute primitive track.volume, calls=%+v", exec.calls)
		case "mix.propose_tick", "mix.apply_tick":
			t.Fatalf("implicit volume request should become pending treatment before executing mix tick, calls=%+v", exec.calls)
		}
	}
	if len(exec.calls) != 1 || !isTestMixObservationTool(exec.calls[0].Tool) {
		t.Fatalf("executor calls = %+v, want only observation before pending treatment", exec.calls)
	}
	treatment := res.ExecutionMemory.PendingMixTreatment
	if treatment == nil || treatment.ActionKind != "gain_balance" || treatment.TargetRef != "track:1007" {
		t.Fatalf("pending treatment = %+v", treatment)
	}
	if treatment.DeltaDB >= 0 || treatment.DeltaDB < -2 {
		t.Fatalf("pending treatment delta = %v, want a small negative gain move", treatment.DeltaDB)
	}
}

func TestMessageLoopImplicitLowerVolumeBecomesPendingAfterObservationPreflight(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"should not need model after deterministic observation","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{mixObservationResult: map[string]any{
		"status":         "ok",
		"track_id":       "1007",
		"observation_id": "obs_test",
		"mix_session_id": "mix_test",
		"summary":        "ready",
		"observation": map[string]any{
			"target_ref": map[string]any{"kind": "track", "id": "1007", "label": "Track 1"},
		},
		"tracks": []map[string]any{{
			"track_id":  "1007",
			"volume_db": 0,
		}},
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 5, MaxToolCalls: 3, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "This track is too loud, lower it a bit",
		AllowedTools: []string{"mix.observe", "mix.request_observation", "mix.propose_tick", "mix.apply_tick"},
		Context:      map[string]any{"selected_track_id": "1007"},
		State:        map[string]any{"selected_track_id": "1007"},
	})

	if res.Status != "completed" || res.StopReason != StopReasonDone {
		t.Fatalf("result = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	if len(client.calls) != 0 {
		t.Fatalf("deterministic gain pending should complete before another model turn, model calls=%d", len(client.calls))
	}
	if len(exec.calls) != 1 || !isTestMixObservationTool(exec.calls[0].Tool) {
		t.Fatalf("executor calls = %+v, want only mix observation", exec.calls)
	}
	treatment := res.ExecutionMemory.PendingMixTreatment
	if treatment == nil || treatment.ActionKind != "gain_balance" || treatment.TargetRef != "track:1007" {
		t.Fatalf("pending treatment = %+v", treatment)
	}
	if treatment.DeltaDB != -1.5 {
		t.Fatalf("pending treatment delta = %v, want -1.5", treatment.DeltaDB)
	}
	for _, want := range []string{"\u5f85\u786e\u8ba4\u52a8\u4f5c", "-1.50 dB"} {
		if !strings.Contains(res.Reply, want) {
			t.Fatalf("reply should describe pending confirmation %q, got %q", want, res.Reply)
		}
	}
}

func TestMessageLoopNaturalMixPrimitiveVolumeGuardBlocksDirectExecution(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"閹存垵鍘涢惄瀛樺复閸樺缍嗛妴?,"tool_calls":[{"id":"lower_volume","tool":"track.volume","args":{"track_id":"1007"},"reason":"閻劍鍩涚拠纾嬬箹閺壜ゅ缓闁挸銇婇崫宥忕礉缁嬪秴浜曢崢瀣╃秵娑撯偓閻?}]}`,
		`{"final":true,"reply":"閹存垳绱伴崗鍫㈢舶閸戝搫绶熺涵顔款吇閻ㄥ嫬鐨獮鍛寸叾闁插繐缂撶拋顔衡偓?,"tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 5, MaxToolCalls: 3, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "This track is too loud, lower it a bit",
		AllowedTools: []string{"track.volume"},
		Context:      map[string]any{"selected_track_id": "1007"},
		State:        map[string]any{"selected_track_id": "1007"},
		RecentObservation: &RecentObservation{
			Tool:        "mix.request_observation",
			CommandName: "mix_request_observation",
			Status:      "ok",
			Summary: map[string]any{
				"status":         "ok",
				"track_id":       "1007",
				"observation_id": "obs_test",
				"mix_session_id": "mix_test",
				"target_ref":     map[string]any{"kind": "track", "id": "1007", "label": "Track 1"},
			},
		},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(exec.calls) != 0 {
		t.Fatalf("direct primitive volume write should be blocked before executor, calls=%+v", exec.calls)
	}
	foundGate := false
	for _, event := range res.Trace {
		if event.Kind == "final_gate" && strings.Contains(event.Message, "direct track volume or pan writes") {
			foundGate = true
			break
		}
	}
	if !foundGate {
		t.Fatalf("expected primitive volume guard; trace=%+v", res.Trace)
	}
}

func TestMessageLoopAmbiguousContinueDoesNotAutoApplyMixTick(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"observe first","tool_calls":[{"id":"observe_mix","tool":"mix.request_observation","args":{"track_id":"1007"},"reason":"observe before action"}]}`,
		`{"final":false,"reply":"lower it","tool_calls":[{"id":"raise_volume","tool":"track.volume","args":{"track_id":"1007","db":-1},"reason":"lower 1 dB"}]}`,
	}}
	exec := &fakeMessageExecutor{mixObservationResult: map[string]any{
		"status":   "ok",
		"track_id": "1007",
		"summary":  "ready",
		"tracks": []map[string]any{{
			"track_id":  "1007",
			"volume_db": 0,
		}},
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 5, MaxToolCalls: 3, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "缂佈呯敾",
		AllowedTools: []string{"track.volume", "mix.request_observation", "mix.propose_tick", "mix.apply_tick"},
	})

	if res.Status != "waiting_confirmation" || res.StopReason != StopReasonNeedsConfirmation {
		t.Fatalf("result = status=%q stop=%q reply=%q error=%q", res.Status, res.StopReason, res.Reply, res.Error)
	}
	foundPropose := false
	for _, call := range exec.calls {
		switch call.Tool {
		case "mix.propose_tick":
			foundPropose = true
		case "mix.apply_tick":
			t.Fatalf("ambiguous continue should not auto-apply mix tick, calls=%+v", exec.calls)
		case "track.volume":
			t.Fatalf("primitive track.volume should be wrapped through mix tick proposal, calls=%+v", exec.calls)
		}
	}
	if !foundPropose {
		t.Fatalf("expected mix.propose_tick before waiting confirmation, got %+v", exec.calls)
	}
}

func TestMessageLoopMixObservationReplyAsksForExecution(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"I will observe the current audio first.","tool_calls":[{"id":"observe_mix","tool":"mix.request_observation","args":{"track_id":"1007"},"reason":"Observe before deciding"}]}`,
		`{"final":true,"reply":"瀹歌尪顫囩€?Track 2閿涙艾鍢查崐鑲╁ -6 dBFS閿涘MS 缁?-6.8 dBFS閿涘苯濮╅幀浣风稇闁插繗绻曢張澶屽 6 dB閵嗗倹娓剁€瑰鍙忛惃鍕瑓娑撯偓濮濄儲妲哥亸蹇撶畽閹绘劙鐝棅鎶藉櫤閿涘本鐦俊?+1 閸?+2 dB閵?,"tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 5, MaxToolCalls: 3, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "Make the current track feel more forward",
		AllowedTools: []string{"track.volume", "mix.request_observation"},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if !strings.Contains(res.Reply, "continue executing") {
		t.Fatalf("reply did not ask for execution confirmation: %q", res.Reply)
	}
}

func TestMessageLoopMixObservationFinalReplyStoresPendingTickCandidate(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"observe","tool_calls":[{"id":"observe_mix","tool":"mix.observe","args":{"track_id":"1007"},"reason":"observe"}]}`,
		`{"final":true,"reply":"鐟欏倸鐧傜€瑰本鍨氶敍灞界紦鐠侇喖鍘涢梽宥勭秵瑜版挸澧犳潪銊╀壕 1 dB閵?,"tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{mixObservationResult: map[string]any{
		"status":         "ok",
		"mix_session_id": "mix_test",
		"observation_id": "obs_test",
		"track_id":       "1007",
		"summary":        "ready",
		"observation": map[string]any{
			"target_ref": map[string]any{"kind": "track", "id": "1007", "label": "Vocal"},
		},
		"tracks": []map[string]any{{
			"track_id":  "1007",
			"volume_db": 0,
		}},
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 5, MaxToolCalls: 3, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "Help me mix the current track",
		AllowedTools: []string{"mix.observe", "mix.request_observation"},
		State: map[string]any{"tracks": []map[string]any{{
			"track_id":  "1007",
			"volume_db": 0,
		}}},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	candidate := res.ExecutionMemory.PendingMixTickCandidate
	if candidate == nil {
		t.Fatalf("pending candidate missing; memory=%+v reply=%q", res.ExecutionMemory, res.Reply)
	}
	if candidate.TrackID != "1007" || candidate.Operation != "track_gain_adjust" || candidate.DeltaDB != -1 {
		t.Fatalf("candidate = %+v", candidate)
	}
	if candidate.ObservationID != "obs_test" || candidate.Status != "pending_confirmation" {
		t.Fatalf("candidate metadata = %+v", candidate)
	}
}

func TestMessageLoopFullProjectObservationReplyDoesNotStorePendingTickCandidate(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"閺佺繝缍嬮惇瀣煂娴?2 閺夆剝婀侀弫鍫ョ叾妫版垼寤洪妴淇唕ack 1 娴ｆ瑩鍣虹€瑰鍙忛敍姹縭ack 2 瀹勬澘鈧厧鍑＄紒蹇撳煂 0 dBFS閿涘eadroom 娑?0 dB閵嗗倸缂撶拋顔荤瑓娑撯偓濮濄儱浠涙稉鈧稉顏勭发鐏忓繒娈戠€瑰鍙忛崝銊ょ稊閿涙碍濡?Track 2 闂勫秳缍嗙痪?1.5 dB閿涘苯鍘涚紒娆忎紣缁嬪鏆€閸戣桨绔撮悙鐟板槻閸婅偐鈹栭梻娣偓鍌濐洣閹存垹鎴风紒顓熷⒔鐞涘矁绻栨稉鈧銉ユ偋閿?,"tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{mixObservationResult: map[string]any{
		"status":         "ok",
		"mix_session_id": "mix_project",
		"observation_id": "obs_project",
		"digest": map[string]any{
			"scope": "full_project",
			"target": map[string]any{
				"kind": "project",
				"id":   "current",
			},
			"likely_first_attention_target": map[string]any{
				"reason": "headroom_risk",
				"track": map[string]any{
					"track_id":    "1012",
					"name":        "Track 2",
					"headroom_db": 0,
					"risk":        "high",
				},
			},
		},
		"observation": map[string]any{
			"target_ref": map[string]any{"kind": "project", "id": "current", "label": "Current project"},
			"project_package": map[string]any{
				"track_count":                 2,
				"active_acoustic_track_count": 2,
				"tracks": []map[string]any{{
					"track_id":         "1007",
					"name":             "Track 1",
					"track_name":       "Track 1",
					"user_label":       "Track 1",
					"user_track_index": 1,
					"volume_db":        0,
					"rms_dbfs":         -9.032,
					"peak_dbfs":        -6.021,
					"headroom_db":      6.021,
				}, {
					"track_id":         "1012",
					"name":             "Track 2",
					"track_name":       "Track 2",
					"user_label":       "Track 2",
					"user_track_index": 2,
					"volume_db":        0,
					"rms_dbfs":         -10.603,
					"peak_dbfs":        0,
					"headroom_db":      0,
				}},
				"headroom_risk": []map[string]any{{
					"track_id": "1012",
					"name":     "Track 2",
					"rank":     1,
					"risk":     "high",
					"value":    0,
				}},
				"likely_first_attention_target": map[string]any{
					"reason": "headroom_risk",
					"track": map[string]any{
						"track_id": "1012",
						"name":     "Track 2",
						"risk":     "high",
					},
				},
			},
		},
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 3, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "Help me inspect the overall mix",
		AllowedTools: []string{"mix.observe", "mix.request_observation"},
		State: map[string]any{"tracks": []map[string]any{{
			"track_id": "1007", "track_name": "Track 1", "volume_db": 0,
		}, {
			"track_id": "1012", "track_name": "Track 2", "volume_db": 0,
		}}},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if candidate := res.ExecutionMemory.PendingMixTickCandidate; candidate != nil {
		t.Fatalf("observation-only full-project reply created pending tick candidate: %+v", candidate)
	}
	if treatment := res.ExecutionMemory.PendingMixTreatment; treatment != nil {
		t.Fatalf("observation-only full-project reply created pending treatment: %+v", treatment)
	}
}

func TestMessageLoopFinalReplyStoresTreatmentPendingAndStripsMarker(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"observe","tool_calls":[{"id":"observe_mix","tool":"mix.observe","args":{"scope":"full_project"},"reason":"observe"}]}`,
		`{"final":true,"reply":"娴ｅ酣顣堕張澶夌昂缁绱濋幋鎴濈紦鐠侇喖鍘涢崙鍡楊槵娑撯偓娑?EQ 缁顦╅悶鍡樻煙閸氭埊绱濈涵顔款吇閸氬氦顔€ resolver 濡偓閺屻儲妲搁崥锕佸厴鐎瑰鍙忛幍褑顢戦妴淇搉mix_treatment_pending: {\"schema_version\":\"mix_treatment_pending.v0\",\"status\":\"pending_confirmation\",\"intent\":\"reduce low-end mud\",\"target_ref\":\"project\",\"action_kind\":\"plugin_treatment\",\"processor_type\":\"eq\",\"reasoning_summary\":\"low end sounds muddy from available observation\",\"confidence\":\"medium\",\"evidence_refs\":[\"observation.digest\"],\"needs_resolution\":[\"target_track\",\"plugin_instance\",\"plugin_profile\",\"exact_control\"],\"expires_after_context_change\":true}","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{mixObservationResult: map[string]any{
		"status":         "ok",
		"mix_session_id": "mix_project",
		"observation_id": "obs_treatment",
		"digest": map[string]any{
			"scope": "full_project",
		},
		"observation": map[string]any{
			"target_ref": map[string]any{"kind": "project", "id": "current"},
			"project_package": map[string]any{
				"track_count": 2,
			},
		},
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 5, MaxToolCalls: 3, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "The low end feels muddy, inspect how to adjust it",
		AllowedTools: []string{"mix.observe", "mix.read", "mix.request_observation"},
		State: map[string]any{"tracks": []map[string]any{{
			"track_id": "1007", "track_name": "Track 1",
		}, {
			"track_id": "1012", "track_name": "Track 2",
		}}},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if strings.Contains(res.Reply, "mix_treatment_pending") {
		t.Fatalf("reply leaked treatment marker: %q", res.Reply)
	}
	treatment := res.ExecutionMemory.PendingMixTreatment
	if treatment == nil {
		t.Fatalf("pending treatment missing; memory=%+v reply=%q", res.ExecutionMemory, res.Reply)
	}
	if treatment.SchemaVersion != "mix_treatment_pending.v0" || treatment.ActionKind != "plugin_treatment" || treatment.ProcessorType != "eq" {
		t.Fatalf("pending treatment = %+v", treatment)
	}
	if treatment.ObservationID != "obs_treatment" || treatment.Status != "pending_confirmation" {
		t.Fatalf("pending treatment metadata = %+v", treatment)
	}
}

func TestMessageLoopChineseActionPreflightObservesThenWaitsForConfirmation(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"閸忓牐顫囩€电喍缍嗘０鎴濇嫲婢规澘鍎氭笟婵囧祦閵?,"tool_calls":[{"id":"observe_mix","tool":"mix.observe","args":{"scope":"track","target_ref":{"kind":"track","id":"1007"},"mom_intent":"action_preflight_observation"},"reason":"閸忓牐顕伴崣?MOM observation 娴ｆ粈璐熸担搴暥婢跺嫮鎮婃笟婵囧祦"}]}`,
		`{"final":true,"reply":"娓氭繃宓佹潻娆愵偧 MOM 鐟欏倸鐧傞敍灞肩秵妫版垵顦╅悶鍡楀涧閼宠棄鍘涙担婊€璐熸穱婵嗙暓 EQ 閺傜懓鎮滈敍宀€鈥樼拋銈呭娑撳秳绱伴崘娆愬絻娴犺埖鍨ㄩ崣鍌涙殶閵嗕繐nmix_treatment_pending: {\"schema_version\":\"mix_treatment_pending.v0\",\"status\":\"pending_confirmation\",\"intent\":\"reduce low end slightly\",\"target_ref\":\"track:1007\",\"action_kind\":\"plugin_treatment\",\"processor_type\":\"eq\",\"reasoning_summary\":\"low-end reduction is proposed from MOM observation evidence; uncertainty remains until plugin/profile/control resolution\",\"confidence\":\"medium\",\"evidence_refs\":[\"observation:obs_action_preflight\",\"mix.read:track.1007.slow.band_energy.summary\"],\"needs_resolution\":[\"plugin_instance\",\"plugin_profile\",\"exact_control\"],\"expires_after_context_change\":true}","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{mixObservationResult: map[string]any{
		"status":         "ok",
		"mix_session_id": "mix_action_preflight",
		"observation_id": "obs_action_preflight",
		"observation": map[string]any{
			"target_ref": map[string]any{"kind": "track", "id": "1007", "label": "Track 1"},
			"mom_projection": map[string]any{
				"mom_version":    "v1.4",
				"intent":         "action_preflight_observation",
				"observation_id": "obs_action_preflight",
				"trust_quality": map[string]any{
					"overall_status":  "ready",
					"evidence_refs":   []any{"observation:obs_action_preflight", "mix.read:track.1007.slow.band_energy.summary"},
					"required_layers": []any{"project_structure", "action_relevant_mom_layer", "trust_quality", "evidence_refs"},
				},
				"llm_context": map[string]any{
					"summary_md":                 "Action preflight must cite MOM evidence first and wait for confirmation before mutation.",
					"do_not_include_raw_package": true,
					"evidence_refs":              []any{"observation:obs_action_preflight"},
				},
			},
		},
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 5, MaxToolCalls: 3, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "Help me tighten the low end a little, but explain the evidence first",
		AllowedTools: []string{"mix.observe", "mix.read", "plugin_grabber_apply_control", "rack.load_plugin", "plugin.set_parameter"},
		Context:      map[string]any{"selected_track_id": "1007"},
		State:        map[string]any{"selected_track_id": "1007", "tracks": []map[string]any{{"track_id": "1007", "track_name": "Track 1"}}},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(exec.calls) != 1 || !isTestMixObservationTool(exec.calls[0].Tool) {
		t.Fatalf("executor calls = %+v, want only MOM observation before pending confirmation", exec.calls)
	}
	for _, call := range exec.calls {
		switch call.Tool {
		case "plugin_grabber_apply_control", "rack.load_plugin", "plugin.set_parameter":
			t.Fatalf("action tool executed before confirmation: %+v", exec.calls)
		}
	}
	if strings.Contains(res.Reply, "mix_treatment_pending") {
		t.Fatalf("reply leaked treatment marker: %q", res.Reply)
	}
	if !strings.Contains(res.Reply, "MOM") && !strings.Contains(strings.ToLower(res.Reply), "evidence") {
		t.Fatalf("reply should explain evidence basis before confirmation: %q", res.Reply)
	}
	treatment := res.ExecutionMemory.PendingMixTreatment
	if treatment == nil || treatment.Status != "pending_confirmation" || treatment.ActionKind != "plugin_treatment" || treatment.ProcessorType != "eq" {
		t.Fatalf("pending treatment = %+v", treatment)
	}
	if treatment.ObservationID != "obs_action_preflight" || len(treatment.EvidenceRefs) == 0 {
		t.Fatalf("pending treatment should reference MOM observation evidence: %+v", treatment)
	}
}

func TestMessageLoopTreatmentMarkerWinsOverEQDBSuggestion(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"observe","tool_calls":[{"id":"observe_mix","tool":"mix.observe","args":{"scope":"track","target_ref":{"kind":"track","id":"1007"}},"reason":"observe"}]}`,
		`{"final":true,"reply":"I observed first. Track 1 low-mid cut -2 dB is only an EQ treatment idea.\nmix_treatment_pending: {\"schema_version\":\"mix_treatment_pending.v0\",\"status\":\"pending_confirmation\",\"intent\":\"reduce low-end mud\",\"target_ref\":\"track:1007\",\"action_kind\":\"plugin_treatment\",\"processor_type\":\"eq\",\"reasoning_summary\":\"low end sounds muddy from available observation\",\"confidence\":\"medium\",\"evidence_refs\":[\"observation.digest\"],\"needs_resolution\":[\"plugin_instance\",\"plugin_profile\",\"exact_control\"],\"expires_after_context_change\":true}\n\nIf you approve, should I continue?","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{mixObservationResult: map[string]any{
		"status":         "ok",
		"mix_session_id": "mix_track",
		"observation_id": "obs_treatment_marker",
		"digest": map[string]any{
			"scope":    "track",
			"track_id": "1007",
		},
		"observation": map[string]any{
			"target_ref": map[string]any{"kind": "track", "id": "1007"},
		},
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 5, MaxToolCalls: 3, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "Track 1 low end is muddy; use an EQ plugin and prepare parameters.",
		AllowedTools: []string{"mix.observe", "mix.read", "mix.request_observation"},
		State: map[string]any{"tracks": []map[string]any{{
			"track_id": "1007", "track_name": "Track 1",
		}}},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if strings.Contains(res.Reply, "mix_treatment_pending") {
		t.Fatalf("reply leaked treatment marker: %q", res.Reply)
	}
	if res.ExecutionMemory.PendingMixTickCandidate != nil {
		t.Fatalf("treatment marker was misread as gain tick: %+v", res.ExecutionMemory.PendingMixTickCandidate)
	}
	treatment := res.ExecutionMemory.PendingMixTreatment
	if treatment == nil || treatment.ActionKind != "plugin_treatment" || treatment.ProcessorType != "eq" {
		t.Fatalf("pending treatment = %+v memory=%+v", treatment, res.ExecutionMemory)
	}
}

func TestMessageLoopTreatmentPendingParsesNestedTarget(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"閸欘垯浜掗崙鍡楊槵娑撯偓娑擃亝妲戠涵顔藉絻娴犺埖甯堕崚璁圭礉绾喛顓婚崥搴ｆ暠 resolver 濡偓閺屻儲澧界悰灞烩偓淇搉mix_treatment_pending: {\"schema_version\":\"mix_treatment_pending.v0\",\"status\":\"pending_confirmation\",\"intent\":\"reduce low mids\",\"target_ref\":\"track:1007\",\"action_kind\":\"plugin_treatment\",\"processor_type\":\"eq\",\"plugin_id\":\"nova_1\",\"control\":\"control low mids with band 2\",\"target\":{\"freq_hz\":300,\"gain_db\":-1.5,\"q\":1.1},\"reasoning_summary\":\"explicit control from profile\",\"confidence\":\"high\",\"evidence_refs\":[\"profile.virtual_controls\"],\"needs_resolution\":[],\"expires_after_context_change\":true}","tool_calls":[]}`,
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: &fakeMessageExecutor{},
		Budget:   Budget{MaxTurns: 2, MaxToolCalls: 1, MaxConsecutiveErrors: 1},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "Low end is muddy, make a small adjustment using existing Nova controls",
		AllowedTools: []string{"mix.observe"},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	treatment := res.ExecutionMemory.PendingMixTreatment
	if treatment == nil {
		t.Fatalf("pending treatment missing; memory=%+v", res.ExecutionMemory)
	}
	if treatment.PluginID != "nova_1" || treatment.Control != "control low mids with band 2" {
		t.Fatalf("pending treatment = %+v", treatment)
	}
	if treatment.Target["freq_hz"].(float64) != 300 || treatment.Target["gain_db"].(float64) != -1.5 {
		t.Fatalf("target = %+v", treatment.Target)
	}
	if strings.Contains(res.Reply, "mix_treatment_pending") {
		t.Fatalf("reply leaked treatment marker: %q", res.Reply)
	}
}

func TestMessageLoopTreatmentPendingParsesGainDelta(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"Small gain move is pending.\nmix_treatment_pending: {\"schema_version\":\"mix_treatment_pending.v0\",\"status\":\"pending_confirmation\",\"intent\":\"bring vocal down slightly\",\"target_ref\":\"track:1007\",\"action_kind\":\"gain_balance\",\"processor_type\":\"utility\",\"delta_db\":-1.25,\"target\":{\"delta_db\":-1.25},\"reasoning_summary\":\"single explicit gain step\",\"confidence\":\"high\",\"evidence_refs\":[\"observation.digest\"],\"needs_resolution\":[],\"expires_after_context_change\":true}","tool_calls":[]}`,
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: &fakeMessageExecutor{},
		Budget:   Budget{MaxTurns: 2, MaxToolCalls: 1, MaxConsecutiveErrors: 1},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "bring the vocal down a little",
		AllowedTools: []string{"mix.observe"},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	treatment := res.ExecutionMemory.PendingMixTreatment
	if treatment == nil {
		t.Fatalf("pending treatment missing; memory=%+v", res.ExecutionMemory)
	}
	if treatment.ActionKind != "gain_balance" || treatment.DeltaDB != -1.25 {
		t.Fatalf("pending treatment = %+v", treatment)
	}
	if treatment.Target["delta_db"].(float64) != -1.25 {
		t.Fatalf("target = %+v", treatment.Target)
	}
	if strings.Contains(res.Reply, "mix_treatment_pending") {
		t.Fatalf("reply leaked treatment marker: %q", res.Reply)
	}
}

func TestMessageLoopTreatmentPendingParsesPanDelta(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"Pan move is pending.\nmix_treatment_pending: {\"schema_version\":\"mix_treatment_pending.v0\",\"status\":\"pending_confirmation\",\"intent\":\"move guitar left slightly\",\"target_ref\":\"track:1007\",\"action_kind\":\"pan_balance\",\"processor_type\":\"utility\",\"delta_pan\":-0.1,\"target\":{\"delta_pan\":-0.1},\"reasoning_summary\":\"single explicit pan step\",\"confidence\":\"high\",\"evidence_refs\":[\"observation.digest\"],\"needs_resolution\":[],\"expires_after_context_change\":true}","tool_calls":[]}`,
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: &fakeMessageExecutor{},
		Budget:   Budget{MaxTurns: 2, MaxToolCalls: 1, MaxConsecutiveErrors: 1},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "move the guitar left a little",
		AllowedTools: []string{"mix.observe"},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	treatment := res.ExecutionMemory.PendingMixTreatment
	if treatment == nil {
		t.Fatalf("pending treatment missing; memory=%+v", res.ExecutionMemory)
	}
	if treatment.ActionKind != "pan_balance" || treatment.DeltaPan != -0.1 || treatment.TargetPan != nil {
		t.Fatalf("pending treatment = %+v", treatment)
	}
	if treatment.Target["delta_pan"].(float64) != -0.1 {
		t.Fatalf("target = %+v", treatment.Target)
	}
	if strings.Contains(res.Reply, "mix_treatment_pending") {
		t.Fatalf("reply leaked treatment marker: %q", res.Reply)
	}
}

func TestMessageLoopTreatmentPendingAcceptsPendingOnlyReply(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"I can move Track 1 left after confirmation.\nmix_treatment_pending: {\"schema_version\":\"mix_treatment_pending.v0\",\"status\":\"pending_confirmation\",\"intent\":\"move current track left\",\"target_ref\":\"track:1007\",\"action_kind\":\"pan_balance\",\"processor_type\":\"utility\",\"delta_pan\":-0.1,\"confidence\":\"high\",\"evidence_refs\":[\"observation.digest\"],\"needs_resolution\":[],\"expires_after_context_change\":true}","tool_calls":[]}`,
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: &fakeMessageExecutor{},
		Budget:   Budget{MaxTurns: 2, MaxToolCalls: 1, MaxConsecutiveErrors: 1},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "pan the current track left",
		AllowedTools: []string{"mix.observe"},
		State: map[string]any{
			"tracks": []map[string]any{{"track_id": "1007", "track_name": "Track 1", "pan": 0.0}},
			"last_mix_observation": map[string]any{
				"observation_id": "obs_1",
				"status":         "ready",
			},
		},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q trace=%+v", res.Status, res.Reply, res.Error, res.Trace)
	}
	treatment := res.ExecutionMemory.PendingMixTreatment
	if treatment == nil || treatment.ActionKind != "pan_balance" || treatment.DeltaPan != -0.1 {
		t.Fatalf("pending treatment = %+v", treatment)
	}
	if strings.Contains(res.Reply, "mix_treatment_pending") {
		t.Fatalf("reply leaked treatment marker: %q", res.Reply)
	}
}

func TestMessageLoopImplicitPanReplyIgnoresStereoBalanceDBAsGain(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"瀹歌尪顫囩€?Track 2閿涘本娈忛張顏冩叏閺€鐟颁紣缁嬪鈧繐n\n瑜版挸澧?Track 2 瀹歌尙绮￠張澶夌閻愮懓浜稿锔肩窗瀹革箑褰搁獮瀹犮€€缁?+1.57 dB閿涘瞼濮搁幀浣规▔缁€?left-heavy閿涙稓鐝涙担鎾筹紣閻╃鍙ф惔锔惧 0.20閿涘本婀佹稉鈧€规氨娴夋担宥夘棑闂勨斂鈧倸娲滃銈咁洤閺嬫粎鎴风紒顓炵窔瀹革讣绱濋幋鎴濈紦鐠侇喖褰ч崑姘发鐏忓繋绔村銉窗閹?Track 2 婢规澘鍎氶崥鎴濅箯缁夎濮?0.10閵嗕繐n\n鐟曚焦鍨滈幍褑顢戦垾娣璻ack 2 婢规澘鍎氬锔拘?0.10閳ユ繂鎮ч敍?,"tool_calls":[]}`,
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: &fakeMessageExecutor{},
		Budget:   Budget{MaxTurns: 2, MaxToolCalls: 1, MaxConsecutiveErrors: 1},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "Track 2 \u58f0\u50cf\u5f80\u5de6\u4e00\u70b9",
		AllowedTools: []string{"mix.observe", "mix.propose_tick", "mix.apply_tick"},
		State: map[string]any{
			"tracks": []map[string]any{{"track_id": "1010", "track_name": "Track 2", "user_track_index": 2, "pan": 0.0}},
			"last_mix_observation": map[string]any{
				"observation_id":  "obs_pan",
				"status":          "ready",
				"track_id":        "1010",
				"target_track_id": "1010",
			},
		},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q trace=%+v", res.Status, res.Reply, res.Error, res.Trace)
	}
	if candidate := res.ExecutionMemory.PendingMixTickCandidate; candidate != nil {
		t.Fatalf("stereo balance dB was misread as gain tick: %+v", candidate)
	}
	treatment := res.ExecutionMemory.PendingMixTreatment
	if treatment == nil || treatment.ActionKind != "pan_balance" || treatment.TargetRef != "track:1010" || treatment.DeltaPan != -0.1 {
		t.Fatalf("pending treatment = %+v", treatment)
	}
	if treatment.DiagnosisContext["problem_kind"] != "pan_balance" {
		t.Fatalf("diagnosis context = %+v", treatment.DiagnosisContext)
	}
}

func TestMessageLoopExplicitChinesePanReplyPrefersTreatmentOverGainDBEvidence(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"鐟欏倸鐧傞崚?Track 2 閺堫剝闊╁鑼病閻ｃ儱浜稿锔肩礄缁?+2 dB left-heavy閿涘绱濋惄绋垮彠閹呭 0.33閿涘本婀佹稉鈧悙鍦祲娴?鐎硅棄瀹虫搴ㄦ珦閵嗗倹澧嶆禒銉ヮ洤閺嬫粏顩﹂幐澶夌稑閻ㄥ嫭鍏傚▔鏇炵窔瀹革讣绱濋幋鎴濈紦鐠侇喖褰ч崑姘发鐏忓繋绔村銉窗Track 2 婢规澘鍎氶崥鎴濅箯缁?0.10閵嗗倽顩﹂幋鎴炲⒔鐞涘矁绻栨稉顏勭毈鐠嬪啯鏆ｉ崥妤嬬吹","tool_calls":[]}`,
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: &fakeMessageExecutor{},
		Budget:   Budget{MaxTurns: 2, MaxToolCalls: 1, MaxConsecutiveErrors: 1},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "Pan Track 2 a little left",
		AllowedTools: []string{"mix.observe", "mix.propose_tick", "mix.apply_tick"},
		State: map[string]any{
			"tracks": []map[string]any{{"track_id": "1010", "track_name": "Track 2", "user_track_index": 2, "pan": 0.0}},
			"last_mix_observation": map[string]any{
				"observation_id":  "obs_pan",
				"status":          "ready",
				"track_id":        "1010",
				"target_track_id": "1010",
			},
		},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q trace=%+v", res.Status, res.Reply, res.Error, res.Trace)
	}
	if candidate := res.ExecutionMemory.PendingMixTickCandidate; candidate != nil {
		t.Fatalf("pan reply dB evidence was misread as gain tick: %+v", candidate)
	}
	treatment := res.ExecutionMemory.PendingMixTreatment
	if treatment == nil || treatment.ActionKind != "pan_balance" || treatment.TargetRef != "track:1010" || treatment.DeltaPan != -0.1 {
		t.Fatalf("pending treatment = %+v", treatment)
	}
}

func TestMessageLoopTreatmentPendingParsesTargetPan(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"Pan center is pending.\nmix_treatment_pending: {\"schema_version\":\"mix_treatment_pending.v0\",\"status\":\"pending_confirmation\",\"intent\":\"center the vocal\",\"target_ref\":\"track:1007\",\"action_kind\":\"pan_balance\",\"processor_type\":\"utility\",\"target\":{\"target_pan\":0},\"reasoning_summary\":\"explicit center pan step\",\"confidence\":\"high\",\"evidence_refs\":[\"observation.digest\"],\"needs_resolution\":[],\"expires_after_context_change\":true}","tool_calls":[]}`,
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: &fakeMessageExecutor{},
		Budget:   Budget{MaxTurns: 2, MaxToolCalls: 1, MaxConsecutiveErrors: 1},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "center the vocal",
		AllowedTools: []string{"mix.observe"},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	treatment := res.ExecutionMemory.PendingMixTreatment
	if treatment == nil || treatment.TargetPan == nil {
		t.Fatalf("pending treatment missing target pan; memory=%+v", res.ExecutionMemory)
	}
	if treatment.ActionKind != "pan_balance" || *treatment.TargetPan != 0 {
		t.Fatalf("pending treatment = %+v", treatment)
	}
	if strings.Contains(res.Reply, "mix_treatment_pending") {
		t.Fatalf("reply leaked treatment marker: %q", res.Reply)
	}
}

func TestMessageLoopImplicitPanConfirmationQuestionBecomesPendingTreatment(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"閸欘垯浜掗妴鍌氱秼閸撳秹鈧鑵戦惃鍕Ц Track 1閿涘苯褰叉禒銉﹀Ω鐎瑰啫鐣崗銊︽啘閸掓澘褰告潏鐧哥礄绾剙褰告竟鏉垮剼閿涘鈧倷绗夋潻鍥箹娴兼俺顔€閺佹挳顩诲灞炬閺勬儳浜搁崣绛圭礉閼拌櫕婧€闁插苯涔忔潏閫涚窗瀵板牏鈹栭妴鍌氼洤閺嬫粈缍樼涵顔肩暰鐟曚礁鐣崗銊ュ礁缂冾噯绱濋幋鎴濆讲娴犮儳鎴风紒顓熷Ω Track 1 鐠佹儳鍩岄張鈧崣鐐解偓?,"tool_calls":[]}`,
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: &fakeMessageExecutor{},
		Budget:   Budget{MaxTurns: 2, MaxToolCalls: 1, MaxConsecutiveErrors: 1},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "Pan Track 1 hard right",
		AllowedTools: []string{"mix.observe", "mix.propose_tick", "mix.apply_tick"},
		State: map[string]any{
			"tracks": []map[string]any{{"track_id": "1007", "track_name": "Track 1", "pan": 0.0}},
			"last_mix_observation": map[string]any{
				"observation_id": "obs_1",
				"status":         "ready",
			},
		},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q trace=%+v", res.Status, res.Reply, res.Error, res.Trace)
	}
	treatment := res.ExecutionMemory.PendingMixTreatment
	if treatment == nil || treatment.ActionKind != "pan_balance" || treatment.TargetPan == nil {
		t.Fatalf("pending treatment = %+v", treatment)
	}
	if *treatment.TargetPan != 1 {
		t.Fatalf("target pan = %v, want 1", *treatment.TargetPan)
	}
}

func TestMessageLoopNaturalPanSetProposalBecomesPendingTreatment(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"Completed: Track 1 has been set hard left.","tool_calls":[{"id":"propose_pan","tool":"mix.propose_tick","args":{"operation":"track_pan_set","track_id":"1007","pan":-1},"reason":"user asked to pan Track 1 hard left"}]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 2, MaxToolCalls: 1, MaxConsecutiveErrors: 1},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "\u53ef\u4ee5\u5e2e\u6211\u628a\u8fd9\u6761\u8f68\u9053\u5b8c\u5168\u6446\u5411\u5de6\u8fb9\u5417\uff1f",
		AllowedTools: []string{"mix.observe", "mix.propose_tick", "mix.apply_tick"},
		State: map[string]any{
			"tracks": []map[string]any{{"track_id": "1007", "track_name": "Track 1", "pan": 0.0}},
		},
	})

	if res.Status != "completed" || res.StopReason != StopReasonDone {
		t.Fatalf("result = status=%q stop=%q reply=%q error=%q trace=%+v", res.Status, res.StopReason, res.Reply, res.Error, res.Trace)
	}
	if len(exec.calls) != 1 || !isTestMixObservationTool(exec.calls[0].Tool) {
		t.Fatalf("executor calls = %+v, want only deterministic observation before pending confirmation", exec.calls)
	}
	for _, call := range exec.calls {
		if call.Tool == "mix.propose_tick" || call.Tool == "mix.apply_tick" {
			t.Fatalf("mix tick tools must not reach executor before pending confirmation, calls=%+v", exec.calls)
		}
	}
	treatment := res.ExecutionMemory.PendingMixTreatment
	if treatment == nil || treatment.ActionKind != "pan_balance" || treatment.TargetPan == nil {
		t.Fatalf("pending treatment = %+v", treatment)
	}
	if treatment.TargetRef != "track:1007" || *treatment.TargetPan != -1 {
		t.Fatalf("pending treatment = %+v", treatment)
	}
	if strings.Contains(strings.ToLower(res.Reply), "completed") || strings.Contains(strings.ToLower(res.Reply), "has been set") {
		t.Fatalf("reply should not claim execution before confirmation: %q", res.Reply)
	}
}

func TestMessageLoopImplicitPanRevisionProposalBecomesPendingTreatment(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"Completed: Track 1 has been set hard right.","tool_calls":[{"id":"propose_pan","tool":"mix.propose_tick","args":{"operation":"track_pan_set","track_id":"1007","target_pan":1},"reason":"user revised pending pan target hard right"}]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 2, MaxToolCalls: 1, MaxConsecutiveErrors: 1},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "Move it right",
		AllowedTools: []string{"mix.propose_tick", "mix.apply_tick"},
		State: map[string]any{
			"tracks": []map[string]any{{"track_id": "1007", "track_name": "Track 1", "pan": 0.0}},
			"last_mix_observation": map[string]any{
				"observation_id": "obs_1",
				"status":         "ready",
			},
		},
	})

	if res.Status != "completed" || res.StopReason != StopReasonDone || res.Continuation != nil {
		t.Fatalf("result = status=%q stop=%q continuation=%+v reply=%q trace=%+v", res.Status, res.StopReason, res.Continuation, res.Reply, res.Trace)
	}
	if len(exec.calls) != 0 {
		t.Fatalf("mix proposal should be converted to pending treatment before executor, calls=%+v", exec.calls)
	}
	treatment := res.ExecutionMemory.PendingMixTreatment
	if treatment == nil || treatment.ActionKind != "pan_balance" || treatment.TargetPan == nil {
		t.Fatalf("pending treatment = %+v", treatment)
	}
	if treatment.TargetRef != "track:1007" || *treatment.TargetPan != 1 {
		t.Fatalf("pending treatment = %+v", treatment)
	}
	if strings.Contains(strings.ToLower(res.Reply), "completed") || strings.Contains(strings.ToLower(res.Reply), "has been set") {
		t.Fatalf("reply should not claim execution before confirmation: %q", res.Reply)
	}
}

func TestMessageLoopImplicitPanRevisionPrimitivePanBecomesPendingTreatment(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"I will set it right now.","tool_calls":[{"id":"set_pan","tool":"track.pan","args":{"track_id":"1007","pan":1},"reason":"user asked for hard right pan"}]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 2, MaxToolCalls: 1, MaxConsecutiveErrors: 1},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "Pan it hard right",
		AllowedTools: []string{"track.pan", "mix.propose_tick", "mix.apply_tick"},
		State: map[string]any{
			"tracks": []map[string]any{{"track_id": "1007", "track_name": "Track 1", "pan": 0.0}},
			"last_mix_observation": map[string]any{
				"observation_id": "obs_1",
				"status":         "ready",
			},
		},
	})

	if res.Status != "completed" || res.StopReason != StopReasonDone || res.Continuation != nil {
		t.Fatalf("result = status=%q stop=%q continuation=%+v reply=%q trace=%+v", res.Status, res.StopReason, res.Continuation, res.Reply, res.Trace)
	}
	if len(exec.calls) != 0 {
		t.Fatalf("primitive pan should be converted to pending treatment before executor, calls=%+v", exec.calls)
	}
	treatment := res.ExecutionMemory.PendingMixTreatment
	if treatment == nil || treatment.ActionKind != "pan_balance" || treatment.TargetPan == nil || *treatment.TargetPan != 1 {
		t.Fatalf("pending treatment = %+v", treatment)
	}
}

func TestMessageLoopPanClarificationBecomesPendingTreatment(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"needs_clarification":true,"reply":"閸欘垯浜掗妴鍌氱秼閸撳秴褰ч張?Track 1閿涘本澧嶆禒銉⑩偓婊冪暊閳ユ繂绨茬拠銉︽Ц鏉╂瑦娼潪銊╀壕閿涙稐绲剧€瑰苯鍙忛幗鍡楀煂閸欏疇绔熼弰顖涚槷鏉堝啯鐎粩顖滄畱婢规澘鍎氱拋鍓х枂閵嗗倷缍樼憰浣瑰灉閻滄澘婀幎?Track 1 绾剙锛愰崓蹇撳煂閺堚偓閸欏疇绔熼崥妤嬬吹","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 2, MaxToolCalls: 1, MaxConsecutiveErrors: 1},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "Pan Track 1 hard right",
		AllowedTools: []string{"mix.observe", "mix.propose_tick", "mix.apply_tick"},
		State: map[string]any{
			"tracks": []map[string]any{{"track_id": "1007", "track_name": "Track 1", "pan": 0.0}},
		},
	})

	if res.Status != "completed" || res.NeedsClarification {
		t.Fatalf("result = status=%q needs=%v reply=%q error=%q trace=%+v", res.Status, res.NeedsClarification, res.Reply, res.Error, res.Trace)
	}
	if len(exec.calls) != 0 {
		t.Fatalf("clarification-to-pending must not execute tools, got %+v", exec.calls)
	}
	treatment := res.ExecutionMemory.PendingMixTreatment
	if treatment == nil || treatment.ActionKind != "pan_balance" || treatment.TargetPan == nil {
		t.Fatalf("pending treatment = %+v", treatment)
	}
	if treatment.TargetRef != "track:1007" || *treatment.TargetPan != 1 {
		t.Fatalf("pending treatment = %+v", treatment)
	}
}

func TestMessageLoopPendingTreatmentDoesNotDuplicateExecutionQuestion(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"Safer first step: pan Track 1 slightly right, for example pan +0.10.\nmix_treatment_pending: {\"schema_version\":\"mix_treatment_pending.v0\",\"status\":\"pending_confirmation\",\"intent\":\"small right pan\",\"target_ref\":\"track:1007\",\"action_kind\":\"pan_balance\",\"processor_type\":\"utility\",\"delta_pan\":0.1,\"confidence\":\"medium\",\"evidence_refs\":[\"observation.digest\"],\"needs_resolution\":[],\"expires_after_context_change\":true}","tool_calls":[]}`,
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: &fakeMessageExecutor{},
		Budget:   Budget{MaxTurns: 2, MaxToolCalls: 1, MaxConsecutiveErrors: 1},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "Pan Track 1 slightly right",
		AllowedTools: []string{"mix.observe", "mix.propose_tick", "mix.apply_tick"},
		State: map[string]any{
			"tracks": []map[string]any{{"track_id": "1007", "track_name": "Track 1", "pan": 0.0}},
			"last_mix_observation": map[string]any{
				"observation_id": "obs_1",
				"status":         "ready",
			},
		},
	})

	if strings.Count(res.Reply, "\u5f85\u786e\u8ba4\u52a8\u4f5c") != 1 || strings.Contains(res.Reply, "\u9700\u8981\u6211\u7ee7\u7eed\u6267\u884c\u5417") {
		t.Fatalf("reply duplicated execution prompt: %q", res.Reply)
	}
}

func TestMessageLoopPanPendingUsesExplicitUserTargetOverSaferSuggestion(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"Safer first step: pan Track 1 slightly right, for example pan +0.10.\nmix_treatment_pending: {\"schema_version\":\"mix_treatment_pending.v0\",\"status\":\"pending_confirmation\",\"intent\":\"small right pan\",\"target_ref\":\"track:1007\",\"action_kind\":\"pan_balance\",\"processor_type\":\"utility\",\"delta_pan\":0.1,\"confidence\":\"medium\",\"evidence_refs\":[\"observation.digest\"],\"needs_resolution\":[],\"expires_after_context_change\":true}","tool_calls":[]}`,
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: &fakeMessageExecutor{},
		Budget:   Budget{MaxTurns: 2, MaxToolCalls: 1, MaxConsecutiveErrors: 1},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "Pan Track 1 hard right",
		AllowedTools: []string{"mix.observe", "mix.propose_tick", "mix.apply_tick"},
		State: map[string]any{
			"tracks": []map[string]any{{"track_id": "1007", "track_name": "Track 1", "pan": 0.0}},
			"last_mix_observation": map[string]any{
				"observation_id": "obs_1",
				"status":         "ready",
			},
		},
	})

	treatment := res.ExecutionMemory.PendingMixTreatment
	if treatment == nil || treatment.TargetPan == nil {
		t.Fatalf("pending treatment = %+v", treatment)
	}
	if treatment.DeltaPan != 0 || *treatment.TargetPan != 1 {
		t.Fatalf("pending treatment = %+v, want target_pan=1 from user text", treatment)
	}
}

func TestMessageLoopTreatmentPendingInfersPanPlacementTargets(t *testing.T) {
	tests := []struct {
		name string
		text string
		want float64
	}{
		{name: "plain left placement", text: "pan the guitar left", want: -0.5},
		{name: "left percent placement", text: "pan the guitar 70% left", want: -0.7},
		{name: "hard left", text: "pan the guitar hard left", want: -1},
		{name: "hard right", text: "pan the guitar hard right", want: 1},
		{name: "fully right placement", text: "pan it fully right", want: 1},
		{name: "right placement", text: "pan the guitar right", want: 0.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delta, target := messageLoopTreatmentPanFromReply(tt.text)
			if delta != 0 || target == nil {
				t.Fatalf("pan parse = delta=%v target=%v", delta, target)
			}
			if *target != tt.want {
				t.Fatalf("target pan = %v, want %v", *target, tt.want)
			}
		})
	}
}

func TestMessageLoopTreatmentPendingKeepsSmallPanMovesAsDeltas(t *testing.T) {
	tests := []struct {
		text string
		want float64
	}{
		{text: "move the guitar left a little", want: -0.1},
		{text: "pan the guitar 10% right a little", want: 0.1},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			delta, target := messageLoopTreatmentPanFromReply(tt.text)
			if target != nil || delta != tt.want {
				t.Fatalf("pan parse = delta=%v target=%v, want delta %v", delta, target, tt.want)
			}
		})
	}
}

func TestMessageLoopTreatmentPendingInfersGainDeltaFromSuggestedMove(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"Track 1 RMS is about -9.03 dBFS, peak is -6.02 dBFS, and headroom is about 6 dB. Suggested first step: lower Track 2 by 2 dB to create headroom.\nmix_treatment_pending: {\"schema_version\":\"mix_treatment_pending.v0\",\"status\":\"pending_confirmation\",\"intent\":\"reduce peak risk\",\"target_ref\":\"track:1010\",\"action_kind\":\"gain_balance\",\"processor_type\":\"utility\",\"reasoning_summary\":\"single explicit gain step\",\"confidence\":\"high\",\"evidence_refs\":[\"observation.digest\"],\"needs_resolution\":[],\"expires_after_context_change\":true}","tool_calls":[]}`,
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: &fakeMessageExecutor{},
		Budget:   Budget{MaxTurns: 2, MaxToolCalls: 1, MaxConsecutiveErrors: 1},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "help me check the overall mix",
		AllowedTools: []string{"mix.observe"},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	treatment := res.ExecutionMemory.PendingMixTreatment
	if treatment == nil {
		t.Fatalf("pending treatment missing; memory=%+v", res.ExecutionMemory)
	}
	if treatment.ActionKind != "gain_balance" || treatment.DeltaDB != -2 {
		t.Fatalf("pending treatment = %+v", treatment)
	}
}

func TestMessageLoopLowMudPluginPrepSynthesizesPendingWhenReplyOmitsMarker(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":true,"reply":"I observed Track 1, but band_energy_summary is missing, so I cannot claim a measured low-frequency buildup. I will not load EQ or change volume yet.","tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{mixObservationResult: map[string]any{
		"status":         "partial",
		"mix_session_id": "mix_low",
		"observation_id": "obs_low",
		"scope":          "full_project",
		"track_id":       "1007",
		"acoustic_digest": map[string]any{
			"band_energy_status": "missing",
			"missing_metrics":    []any{"band_energy_summary"},
		},
		"observation": map[string]any{
			"target_ref": map[string]any{"kind": "track", "id": "1007", "label": "Track 1"},
			"mix_package": map[string]any{
				"missing_metrics": []any{"band_energy_summary"},
				"source_capabilities": map[string]any{
					"band_energy": "missing",
				},
			},
			"deep_package": map[string]any{
				"source_capabilities": map[string]any{
					"spectrogram_tiles": "missing",
				},
			},
		},
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 3, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "Track 1 low end is muddy. Do not change volume; use an EQ plugin for a low cut and low-mid treatment, prepare plugin parameters first.",
		AllowedTools: []string{"mix.observe", "mix.request_observation", "plugin.load_to_rack"},
		State: map[string]any{
			"tracks": []map[string]any{{"track_id": "1007", "track_name": "Track 1", "user_track_index": 1}},
		},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q trace=%+v", res.Status, res.Reply, res.Error, res.Trace)
	}
	if len(exec.calls) != 1 || !isTestMixObservationTool(exec.calls[0].Tool) {
		t.Fatalf("executor calls = %+v, want only deterministic mix observation", exec.calls)
	}
	if strings.Contains(res.Reply, "mix_treatment_pending") {
		t.Fatalf("reply leaked internal marker: %q", res.Reply)
	}
	treatment := res.ExecutionMemory.PendingMixTreatment
	if treatment == nil {
		t.Fatalf("pending treatment missing; memory=%+v", res.ExecutionMemory)
	}
	if treatment.ActionKind != "plugin_treatment" || treatment.ProcessorType != "eq" {
		t.Fatalf("pending treatment = %+v", treatment)
	}
	if treatment.TargetRef != "track:1007" || treatment.ObservationID != "obs_low" {
		t.Fatalf("pending treatment = %+v", treatment)
	}
	if len(treatment.NeedsResolution) == 0 || !messageLoopDiagnosisRefsContain(treatment.EvidenceRefs, treatment.DiagnosisContextID) {
		t.Fatalf("pending evidence/needs = refs=%+v needs=%+v diag=%q", treatment.EvidenceRefs, treatment.NeedsResolution, treatment.DiagnosisContextID)
	}
	if treatment.DiagnosisContext["problem_kind"] != "low_mud" {
		t.Fatalf("diagnosis context = %+v", treatment.DiagnosisContext)
	}
	recommendation := messageLoopMapValue(treatment.DiagnosisContext["recommendation"])
	if recommendation["strategy"] != "conservative_probe" {
		t.Fatalf("recommendation = %+v", recommendation)
	}
	if !messageLoopDiagnosisHasMissing(treatment.DiagnosisContext, "band_energy_summary") {
		t.Fatalf("missing evidence = %+v", treatment.DiagnosisContext["missing_evidence"])
	}
}

func TestMessageLoopUnavailableMixObservationDoesNotUnlockPluginLoad(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"閹存垵鍘涚憴鍌氱檪瑜版挸澧犻棅鎶筋暥閵?,"tool_calls":[{"id":"observe_mix","tool":"mix.request_observation","args":{"track_id":"1007"}}]}`,
		`{"final":false,"reply":"鐟欏倸鐧傜€瑰奔绨￠敍灞藉櫙婢跺洤濮炴潪?EQ閵?,"tool_calls":[{"id":"load_eq","tool":"plugin.load_to_rack","args":{"track_id":"1007","plugin_query":"TDR Nova","zone_id":"Z3"}}]}`,
		`{"final":true,"reply":"瑜版挸澧犵憴鍌氱檪缂佹挻鐏夋稉宥呭讲閻㈩煉绱濋幋鎴滅瑝娴兼艾濮炴潪鑺ュ絻娴犺绱遍棁鈧憰浣稿帥鐞涖儴鍐婚崣顖滄暏鐟欏倸鐧傞幋鏍唨娴ｇ姷鈥樼拋銈呭徔娴ｆ挻褰冩禒鑸垫惙娴ｆ嚎鈧?,"tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{mixObservationResult: map[string]any{
		"status": "unavailable",
		"mixboard": map[string]any{
			"status":        "unavailable",
			"open_blockers": []any{"audio_feature_request_blocked"},
			"package_status": map[string]any{
				"mix": "limited",
			},
		},
	}}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 5, MaxToolCalls: 3, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "Can you help me compress this audio?",
		AllowedTools: []string{"plugin.load_to_rack", "mix.request_observation"},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(exec.calls) != 1 || !isTestMixObservationTool(exec.calls[0].Tool) {
		t.Fatalf("executor calls = %+v, want only unavailable observation", exec.calls)
	}
	for _, call := range exec.calls {
		if call.Tool == "plugin.load_to_rack" {
			t.Fatalf("plugin load should remain blocked after unavailable observation: %+v", exec.calls)
		}
	}
}

func TestMessageLoopObservationPackageQuestionRoutesPluginParamsToMixRead(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"閹存垶鐓￠惇?observation 閺佺増宓侀崠鍛偓?,"tool_calls":[{"id":"read_params","tool":"plugin.get_parameters","args":{"track_id":"1007","plugin_id":"plugin_1"},"reason":"閺屻儳婀呴弫鐗堝祦閸?}]}`,
		`{"final":true,"reply":"閹存垼顕伴崣鏍畱閺?MixBoard 婢规澘顒熺憴鍌氱檪閸栧拑绱濇稉宥嗘Ц閹绘帊娆㈤崣鍌涙殶閵?,"tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "Observation cannot see the acoustic data package now; help me inspect why",
		AllowedTools: []string{"plugin.get_parameters", "mix.read"},
		RecentObservation: &RecentObservation{
			Tool:        "mix.observe",
			CommandName: "mix_request_observation",
			Status:      "ok",
			Summary: map[string]any{
				"observation_id": "obs_test",
				"mix_session_id": "mix_test",
			},
		},
		ExecutionMemory: ExecutionMemory{ActiveWorkTargetTrackID: "1007"},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(exec.calls) != 1 || exec.calls[0].Tool != "mix.read" {
		t.Fatalf("executor calls = %+v, want mix.read", exec.calls)
	}
	keys := fmt.Sprint(exec.calls[0].Args["keys"])
	for _, want := range []string{"observation.digest", "track.1007.fast.levels", "track.1007.slow.time_energy.summary"} {
		if !strings.Contains(keys, want) {
			t.Fatalf("mix.read keys missing %s: %+v", want, exec.calls[0].Args)
		}
	}
	if fmt.Sprint(exec.calls[0].Args["plugin_id"]) != "" && fmt.Sprint(exec.calls[0].Args["plugin_id"]) != "<nil>" {
		t.Fatalf("plugin args leaked into mix.read: %+v", exec.calls[0].Args)
	}
}

func TestMessageLoopNaturalMixRequestBlocksDirectWaveformBake(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"閹存垵鍘涢崙鍡楊槵濞夈垹鑸伴妴?,"tool_calls":[{"id":"bake","tool":"clip.warm_waveform_bake","args":{"track_id":"1007"},"reason":"閸戝棗顦憴鍌氱檪"}]}`,
		`{"final":false,"reply":"閹存垶鏁奸悽銊﹁穿闂婂疇顫囩€电喆鈧?,"tool_calls":[{"id":"observe_mix","tool":"mix.request_observation","args":{"track_id":"1007"},"reason":"缂佺喍绔寸憴鍌氱檪"}]}`,
		`{"final":true,"reply":"瀹歌尙绮＄€瑰本鍨氱憴鍌氱檪閵嗗倹鍨滄导姘帥閸╄桨绨憴鍌氱檪缂佹瑥鍤楦款唴閿涘奔绗夋导姘辨纯閹恒儱濮炴潪鑺ュ絻娴犺翰鈧?,"tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 5, MaxToolCalls: 3, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "鐢喗鍨滅紓鈺傝穿鏉╂瑦顔岄棅鎶筋暥",
		AllowedTools: []string{"clip.warm_waveform_bake", "mix.request_observation"},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(exec.calls) == 0 || !isTestMixObservationTool(exec.calls[0].Tool) {
		t.Fatalf("executor calls = %+v, want first call to be mix observation", exec.calls)
	}
	for _, call := range exec.calls {
		if call.Tool == "clip.warm_waveform_bake" {
			t.Fatalf("direct waveform bake should be blocked before executor: %+v", exec.calls)
		}
	}
	foundRewrite := false
	for _, event := range res.Trace {
		if event.Kind == "tool_call_rewritten" && strings.Contains(event.Message, "mix.observe") {
			foundRewrite = true
			break
		}
	}
	if !foundRewrite {
		t.Fatalf("expected waveform bake rewrite; trace=%+v", res.Trace)
	}
}

func TestMessageLoopAudioObservationRequestCoercesWaveformBake(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"閹存垵鍘涢崙鍡楊槵濞夈垹鑸伴妴?,"tool_calls":[{"id":"bake","tool":"clip.warm_waveform_bake","args":{"track_id":"1007"},"reason":"閸戝棗顦竟鏉款劅鐟欏倸鐧?}]}`,
		`{"final":true,"reply":"瀹告煡鈧俺绻冨ǎ鐑界叾鐟欏倸鐧傚銉ュ徔鐎瑰本鍨氭竟鏉款劅鐟欏倸鐧傞敍灞剧梾閺堝娲块幒銉ㄧ殶閻劋缍嗙仦鍌涘皾瑜般垻绱︾€涙ê浼愰崗鏋偓?,"tool_calls":[]}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "Please continue audio observation analysis",
		AllowedTools: []string{"mix.request_observation"},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(exec.calls) != 1 {
		t.Fatalf("executor calls = %+v, want one coerced observation", exec.calls)
	}
	if !isTestMixObservationTool(exec.calls[0].Tool) {
		t.Fatalf("executor call = %+v, want mix observation", exec.calls[0])
	}
	if exec.calls[0].ID != "observe_mix" {
		t.Fatalf("coerced call id = %q, want observe_mix", exec.calls[0].ID)
	}
	for _, call := range exec.calls {
		if call.Tool == "clip.warm_waveform_bake" {
			t.Fatalf("direct waveform bake reached executor: %+v", exec.calls)
		}
	}
}

func TestMessageLoopExplicitPluginLoadDoesNotRequireMixObservation(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"濮濓絽婀崝鐘烘祰閵?,"tool_calls":[{"id":"load_eq","tool":"plugin.load_to_rack","args":{"track_id":"1007","plugin_query":"TDR Nova","zone_id":"Z3"}}]}`,
		`{"final":true,"reply":"TDR Nova 瀹告彃濮炴潪濮愨偓?}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 2, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{
		UserText:     "Load TDR Nova plugin",
		AllowedTools: []string{"plugin.load_to_rack", "mix.request_observation"},
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(exec.calls) != 1 || exec.calls[0].Tool != "plugin.load_to_rack" {
		t.Fatalf("executor calls = %+v, want plugin load", exec.calls)
	}
}

func TestMessageLoopNaturalMixConfirmationObservesBeforePendingPluginLoad(t *testing.T) {
	client := &fakeMessageCompleter{responses: []string{
		`{"final":false,"reply":"I will observe the current audio first.","tool_calls":[{"id":"observe_mix","tool":"mix.request_observation","args":{"track_id":"1007"},"reason":"Observe before deciding"}]}`,
		`{"final":true,"reply":"瀹告彃鐣幋鎰潎鐎电噦绱濋崗鍫滅瑝閸旂姾娴囬幓鎺嶆閵?}`,
	}}
	exec := &fakeMessageExecutor{}
	loop := &MessageLoop{
		Client:   client,
		Config:   config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Executor: exec,
		Budget:   Budget{MaxTurns: 4, MaxToolCalls: 3, MaxConsecutiveErrors: 2},
	}
	pending := planner.ToolCall{
		ID:   "load_eq",
		Tool: "plugin.load_to_rack",
		Args: map[string]any{"track_id": "1007", "plugin_query": "TDR Nova", "zone_id": "Z3"},
	}

	res := loop.ResumeAfterConfirmation(context.Background(), Continuation{
		GoalID:          "goal_test",
		RunID:           "run_test",
		UserText:        "鐢喗鍨滅紓鈺傝穿瑜版挸澧犳潪銊╀壕",
		AllowedTools:    []string{"plugin.load_to_rack", "mix.request_observation"},
		PendingToolCall: &pending,
	})

	if res.Status != "completed" {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(exec.calls) != 1 || !isTestMixObservationTool(exec.calls[0].Tool) {
		t.Fatalf("executor calls = %+v, want only mix observation", exec.calls)
	}
	for _, call := range exec.calls {
		if call.Tool == "plugin.load_to_rack" {
			t.Fatalf("pending plugin load was executed: %+v", exec.calls)
		}
	}
	foundGate := false
	for _, event := range res.Trace {
		if event.Kind == "final_gate" && strings.Contains(event.Message, "mix.observe") {
			foundGate = true
			break
		}
	}
	if !foundGate {
		t.Fatalf("expected observe-first gate; trace=%+v", res.Trace)
	}
}

func TestMessageLoopMixObservationSummaryIncludesStructAcousticPackages(t *testing.T) {
	observation := mixboard.ObservationPacket{
		Status:        "ready",
		MixSessionID:  "mix_test",
		ObservationID: "obs_test",
		TargetRef: mixboard.TargetRef{
			Kind:  "track",
			ID:    "1007",
			Label: "Track 1",
		},
		TimeRuler: mixboard.TimeRuler{
			DurationSeconds: 219.384,
			SegmentSeconds:  2,
			FrameSeconds:    0.01,
		},
		MixPackage: map[string]any{
			"status": "baseline_ready",
			"current_metrics": map[string]any{
				"waveform": map[string]any{
					"status":      "ready",
					"peak_dbfs":   -0.109,
					"rms_dbfs":    -17.543,
					"headroom_db": 0.109,
					"crest_db":    17.434,
				},
				"time_energy": []map[string]any{
					{"start_seconds": 0.0, "end_seconds": 5.0, "rms_dbfs": -18.1, "peak_dbfs": -1.2},
				},
			},
			"source_capabilities": map[string]string{
				"waveform_envelope": "ready",
				"time_energy":       "ready",
			},
		},
	}
	result := map[string]any{
		"status":           "ready",
		"mix_session_id":   "mix_test",
		"observation_id":   "obs_test",
		"observation_path": "obs_test.json",
		"observation":      observation,
		"context_pack": mixboard.ContextPack{
			MixSessionID:      "mix_test",
			LatestObservation: mustMessageLoopMap(t, observation),
		},
	}

	summary := mixObservationPromptSummary(result)
	data, _ := json.Marshal(summary)
	text := string(data)
	for _, want := range []string{"peak_dbfs", "rms_dbfs", "headroom_db", "crest_db", "time_energy", "baseline_ready"} {
		if !strings.Contains(text, want) {
			t.Fatalf("summary missing %q: %s", want, text)
		}
	}
}

func TestMessageLoopGeneralObservationSummaryUsesMOMContextContract(t *testing.T) {
	result := map[string]any{
		"status":         "ok",
		"mix_session_id": "mix_general",
		"observation_id": "obs_general",
		"observation": map[string]any{
			"target_ref":     map[string]any{"kind": "track", "id": "1007", "label": "Track 1"},
			"global_summary": map[string]any{"peak_dbfs": -1.0},
			"mom_projection": map[string]any{
				"mom_version":    "v1.1",
				"intent":         "general_band_stereo_observation",
				"observation_id": "obs_general",
				"trust_quality":  map[string]any{"overall_status": "ready", "required_layers": []any{"project_structure", "timbre_frequency.l3_full_song", "space_stereo.l3_full_song"}},
				"llm_context": map[string]any{
					"summary_md":                 "General observation prioritizes L3 full-song band/stereo evidence.",
					"do_not_include_raw_package": true,
					"compact_facts": []any{
						map[string]any{"layer": "timbre_frequency.l3_full_song", "status": "ready"},
						map[string]any{"layer": "space_stereo.l3_full_song", "status": "ready"},
						map[string]any{"layer": "l2_realtime_status", "summary": "L2 realtime available only as status unless explicitly requested."},
					},
				},
			},
			"mix_package": map[string]any{
				"current_metrics": map[string]any{
					"band_energy":     map[string]any{"status": "ready", "source": "spectral_tile_derived", "bands": map[string]any{"bass": map[string]any{"energy_db": -12.0}}},
					"stereo_relation": map[string]any{"status": "ready", "balance_db": 0.2, "correlation_estimate": 0.82},
				},
				"realtime_metrics": map[string]any{
					"band_energy":     map[string]any{"status": "ready", "source": "live_level_meter_spectrum", "bands": map[string]any{"bass": map[string]any{"unit_energy": 0.88, "energy_db": -1.11}}},
					"stereo_relation": map[string]any{"status": "ready", "source": "live_level_meter_stereo", "balance_db": 1.234, "correlation_estimate": 0.654},
				},
			},
		},
	}

	summary := mixObservationPromptSummary(result)
	data, _ := json.Marshal(summary)
	text := string(data)
	for _, want := range []string{"mom_projection", "general_band_stereo_observation", "timbre_frequency.l3_full_song"} {
		if !strings.Contains(text, want) {
			t.Fatalf("summary missing MOM contract field %s in %s", want, text)
		}
	}
	for _, forbidden := range []string{"unit_energy", "-1.11", "1.234", "0.654", "realtime_metrics", "global_summary", "mix_package"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("summary leaked realtime/raw field %s in %s", forbidden, text)
		}
	}
}

func mustMessageLoopMap(t *testing.T, v any) map[string]any {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var row map[string]any
	if err := json.Unmarshal(data, &row); err != nil {
		t.Fatal(err)
	}
	return row
}

func TestMessageLoopPlanModePromptNamesReadOnlyMode(t *testing.T) {
	prompt := messageLoopSystemPrompt(&runState{
		input: Input{
			Context:      map[string]any{"agent_mode": "plan"},
			AllowedTools: []string{"track.list", "plugin.search"},
		},
	})
	for _, want := range []string{
		"Plan mode",
		"read-only",
		"Do not say the DAW lacks that capability",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("plan prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestParseMessageLoopOutputAcceptsFencedJSON(t *testing.T) {
	out, err := parseMessageLoopOutput("```json\n{\"final\":true,\"reply\":\"Done\",\"tool_calls\":[]}\n```")
	if err != nil {
		t.Fatalf("parseMessageLoopOutput: %v", err)
	}
	if !out.Final || out.Reply != "Done" {
		t.Fatalf("out = %+v", out)
	}
}

func TestParseMessageLoopOutputExtractsBalancedJSONObject(t *testing.T) {
	out, err := parseMessageLoopOutput("I will execute:\n{\"final\":false,\"reply\":\"Ready to write\",\"tool_calls\":[{\"tool\":\"midi.apply_note_patch\",\"args\":{\"clip_id\":\"clip_a\"},\"reason\":\"Write MIDI\"}]}\nPlease confirm.")
	if err != nil {
		t.Fatalf("parseMessageLoopOutput: %v", err)
	}
	if out.Final || len(out.ToolCalls) != 1 || out.ToolCalls[0].Tool != "midi.apply_note_patch" {
		t.Fatalf("out = %+v", out)
	}
}

func TestMessageLoopRepairsInvalidJSONOnce(t *testing.T) {
	t.Setenv("VIT_AGENT_MESSAGE_LOOP_DEBUG_PATH", "off")
	client := &fakeMessageCompleter{responses: []string{
		`{"final": false, "reply": "閸戝棗顦幍褑顢?, "tool_calls": [`,
		`{"final":true,"reply":"瀹稿弶浠径宥勮礋閸氬牊纭剁拋鈥冲灊閵?,"tool_calls":[]}`,
	}}
	loop := &MessageLoop{
		Client: client,
		Config: config.EngineConfig{BaseURL: "http://example.invalid", APIKey: "test", DefaultModel: "test"},
		Budget: Budget{MaxTurns: 4, MaxToolCalls: 4, MaxConsecutiveErrors: 2},
	}

	res := loop.Start(context.Background(), Input{UserText: "Test JSON repair"})
	if res.Status != "completed" || res.Reply != "Repaired into a valid plan." {
		t.Fatalf("result = status=%q reply=%q error=%q", res.Status, res.Reply, res.Error)
	}
	if len(client.calls) != 2 {
		t.Fatalf("LLM calls = %d, want initial + repair", len(client.calls))
	}
	if got := client.calls[1][0].Role; got != "system" {
		t.Fatalf("repair prompt first role = %q", got)
	}
}
