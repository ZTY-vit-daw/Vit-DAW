package mom

import "testing"

func TestBuildStaticLevelRelationshipFromProjectPackage(t *testing.T) {
	proj := Build(Input{
		ObservationID: "obs-1", Intent: IntentProjectMultitrackObservation,
		ProjectPackage: map[string]any{
			"status": "ready", "project_state_hash": "cut-1",
			"static_level_relationship_inputs": map[string]any{
				"schema_version": StaticLevelRelationshipSchema, "status": "ready", "project_cut_ref": "cut-1",
				"tracks": []map[string]any{{
					"track_id": "track-1", "status": "ready", "metric": "effective_static_rms_dbfs",
					"tap_point": "derived_static_control_model", "source_rms_dbfs": -24.0,
					"effective_static_rms_dbfs": -21.0, "effective_static_peak_dbfs": -5.0,
					"fader_db": 0.0, "included_clip_ids": []string{"clip-a", "clip-b"},
					"aggregation_method": "duration_weighted_linear_energy_source_plus_clip_gain_plus_fader",
					"evidence_refs":      []string{"dad.track_waveform_envelope:track-1:clip-a"},
				}},
			},
		},
	})
	relation := proj.StaticLevelRelationship
	if relation == nil || relation.Status != StatusReady || relation.ProjectCutRef != "cut-1" || len(relation.Tracks) != 1 {
		t.Fatalf("static relationship = %#v", relation)
	}
	track := relation.Tracks[0]
	if track.EffectiveStaticRMSDBFS == nil || *track.EffectiveStaticRMSDBFS != -21.0 || len(track.IncludedClipIDs) != 2 {
		t.Fatalf("static track = %#v", track)
	}
	ctx := ContextProjection(proj)
	if mapValue(ctx["static_level_relationship"])["project_cut_ref"] != "cut-1" {
		t.Fatalf("context projection omitted static relationship: %#v", ctx)
	}
}

func TestBuildStaticLevelRelationshipDoesNotCountStaleOrSuspectMetricsAsUsable(t *testing.T) {
	for _, status := range []string{StatusStale, StatusSuspect} {
		t.Run(status, func(t *testing.T) {
			proj := Build(Input{
				ObservationID: "obs-untrusted", Intent: IntentProjectMultitrackObservation,
				ProjectPackage: map[string]any{
					"project_state_hash": "cut-untrusted",
					"static_level_relationship_inputs": map[string]any{
						"schema_version": StaticLevelRelationshipSchema, "status": status, "project_cut_ref": "cut-untrusted",
						"tracks": []map[string]any{{
							"track_id": "track-1", "status": status, "metric": "effective_static_rms_dbfs",
							"tap_point": "derived_static_control_model", "effective_static_rms_dbfs": -21.0,
						}},
					},
				},
			})
			relation := proj.StaticLevelRelationship
			if relation == nil || relation.Status != status || relation.Freshness != status {
				t.Fatalf("%s relationship state was not preserved: %#v", status, relation)
			}
			if got := relation.Coverage["usable_track_count"]; got != 0 {
				t.Fatalf("%s metric counted as usable: coverage=%#v", status, relation.Coverage)
			}
			if len(relation.Tracks) != 1 || relation.Tracks[0].Status != status || relation.Tracks[0].EffectiveStaticRMSDBFS == nil {
				t.Fatalf("%s track evidence was not preserved for audit: %#v", status, relation.Tracks)
			}
		})
	}
}
