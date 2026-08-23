# One-shot generator for agent/internal/agentloop/testdata/free_state_replay/
# fixture family (docs/FREE_STATE_TEST_AND_REPLAY_MATRIX_V1.md §3).
# Sources: artifacts/free_state_open_intent_acceptance/<run>/transcript/00_response.json
import json, hashlib, io, os

def sha(p):
    return hashlib.sha256(open(p,'rb').read()).hexdigest()

BASE='artifacts/free_state_open_intent_acceptance'
def src(run, transform):
    p=f'{BASE}/{run}/transcript/00_response.json'
    return {"artifact": p, "sha256": sha(p), "transform": transform}

INTENT="检查一下当前工程有什么问题？"
LOOP={"schema_version":"free_state_reasoning_loop.v1","status":"reasoning","decision_phase":"processor_selection","original_intent":INTENT}
def ctx(extra_loop=None, extra=None):
    loop=dict(LOOP)
    if extra_loop: loop.update(extra_loop)
    c={"free_state_reasoning_loop": loop}
    if extra: c.update(extra)
    return c

def obs(views, status, oid, rev="rev-7", extra=None):
    d={"views":views,"status":status,"observation_id":oid,"project_revision":rev,"freshness":"current_snapshot"}
    if extra: d.update(extra)
    return d

def needs_observation(views, cid, summary):
    return json.dumps({"final":False,"reply":summary,"free_state":{"schema_version":"free_state_decision.v1","status":"needs_observation","evidence_status":"insufficient","summary":summary,"requested_view_ids":views},"tool_calls":[{"id":cid,"tool":"ccb.observation_request","args":{"view_ids":views}}]}, ensure_ascii=False)

def blocked(summary, limitations):
    return json.dumps({"final":True,"reply":summary,"free_state":{"schema_version":"free_state_decision.v1","status":"blocked","evidence_status":"insufficient","summary":summary,"limitations":limitations},"tool_calls":[]}, ensure_ascii=False)

F={}

F['valid.json']={
 "family":"valid","matrix_ids":["M11"],
 "source":src('20260823_145032',"envelope_to_model_turn_script: HTTP envelope (original_intent + observing/needs_observation loop state) re-expressed as deterministic model-turn JSON; view ids and revisions taken from the CCB contract identifiers, no new facts"),
 "intent":INTENT,
 "context":ctx(),
 "responses":[
   needs_observation(["project.structure"],"ccb-valid-1","Need the project layout before any bounded acoustic view."),
   needs_observation(["track.basic_energy"],"ccb-valid-2","Need one bounded target-level energy view."),
   blocked("The bounded diagnostic window reached its evidence boundary; no processor action is justified on the available views.",["observation budget consumed"]),
 ],
 "tool_results":[
   obs(["project.structure"],"ready","obs-replay-valid-1"),
   obs(["track.basic_energy"],"ready","obs-replay-valid-2"),
 ],
 "expected":{"terminal_status":"blocked","model_calls":3,"observation_calls":2},
}

F['malformed_truncated.json']={
 "family":"malformed","variant":"truncated_json","matrix_ids":["M12"],
 "source":src('20260823_120030',"synthetic_negative_variant: valid turn script truncated mid-JSON to exercise the parse-rejection path"),
 "intent":INTENT,
 "context":ctx(),
 "responses":[
   '{"final":false,"reply":"inspect.","free_state":{"schema_version":"free_state_decision.v1","status":"needs_observ',
   blocked("Malformed model output was rejected and repaired; the loop ends at the explicit boundary.",["protocol repair used"]),
 ],
 "tool_results":[],
 "expected":{"terminal_status":"blocked","protocol_repairs_min":1},
}

F['malformed_unknown_status.json']={
 "family":"malformed","variant":"unknown_status","matrix_ids":["M12"],
 "source":src('20260823_120030',"synthetic_negative_variant: decision status replaced with a non-production value"),
 "intent":INTENT,
 "context":ctx(),
 "responses":[
   json.dumps({"final":True,"reply":"seems ready.","free_state":{"schema_version":"free_state_decision.v1","status":"maybe_ready","evidence_status":"sufficient","summary":"unknown status"},"tool_calls":[]}, ensure_ascii=False),
   blocked("Unknown decision status was rejected by schema validation; the loop ends at the explicit boundary.",["protocol repair used"]),
 ],
 "tool_results":[],
 "expected":{"terminal_status":"blocked","protocol_repairs_min":1},
}

F['malformed_bad_view_id.json']={
 "family":"malformed","variant":"bad_view_id","matrix_ids":["M12"],
 "source":src('20260823_120030',"synthetic_negative_variant: requested view id replaced with a value outside the CCB catalog"),
 "intent":INTENT,
 "context":ctx(),
 "responses":[
   needs_observation(["track.nonexistent_view"],"ccb-badview-1","Request a view outside the catalog."),
   blocked("The requested view id is not in the CCB catalog; the rejection is recorded with its receipt and the loop ends at the boundary.",["unknown view id rejected"]),
 ],
 "tool_results":[],
 "expected":{"terminal_status":"blocked","observation_calls":1,"rejected_observation":True},
}

F['repetitive.json']={
 "family":"repetitive","matrix_ids":["M13"],
 "source":src('20260823_125822',"synthetic_repetitive_variant: the same usable view set requested twice to exercise the M04 repeat rejection"),
 "intent":INTENT,
 "context":ctx(),
 "responses":[
   needs_observation(["project.structure"],"ccb-rep-1","Need the project layout once."),
   needs_observation(["project.structure"],"ccb-rep-2","Request the same usable view set again."),
   needs_observation(["track.basic_energy"],"ccb-rep-3","Switch to a different bounded view after the repeat rejection."),
   blocked("The repeat request was rejected; one usable view per view set stands and the loop ends at the boundary.",["repeat rejected"]),
 ],
 "tool_results":[
   obs(["project.structure"],"ready","obs-replay-rep-1"),
   obs(["track.basic_energy"],"ready","obs-replay-rep-2"),
 ],
 "expected":{"terminal_status":"blocked","observation_calls":2,"repeat_rejection":"already returned usable evidence"},
}

F['partial.json']={
 "family":"partial","matrix_ids":["M14"],
 "source":src('20260823_145032',"envelope_to_model_turn_script with partial bundle: usable-but-partial view returned, readiness must stay un-upgraded"),
 "intent":INTENT,
 "context":ctx(),
 "responses":[
   needs_observation(["track.timbre_frequency"],"ccb-partial-1","Need the timbre view."),
   blocked("The partial view is preserved as-is; no readiness upgrade is inferred from an incomplete bundle.",["partial evidence preserved"]),
 ],
 "tool_results":[
   obs(["track.timbre_frequency"],"partial","obs-replay-partial-1",extra={"limitations":["frequency band coverage incomplete"]}),
 ],
 "expected":{"terminal_status":"blocked","observation_status":"partial","observation_calls":1},
}

stale_loop_ctx={"observation_ledger":{"schema_version":"free_state_observation_ledger.v1","receipts":[
  {"status":"ready","observation_id":"obs-replay-stale-target","requested_views":["track.timbre_frequency"],
   "target_ref":{"kind":"track","id":"1007"},"evidence_refs":["obs-replay-stale-target"],
   "project_revision":"rev-old","freshness":{"status":"stale","project_revision":"rev-old"}}],
 "available_views":{"track:1007::track.timbre_frequency":{"view_id":"track.timbre_frequency","status":"ready","observation_id":"obs-replay-stale-target","target_ref":{"kind":"track","id":"1007"},"evidence_refs":["obs-replay-stale-target"],"project_revision":"rev-old","freshness":{"status":"stale","project_revision":"rev-old"}}}}}
F['stale.json']={
 "family":"stale","matrix_ids":["M15"],
 "source":src('20260823_125822',"synthetic_stale_variant: ledger receipt bound to an older project revision, proposal cites it"),
 "intent":INTENT,
 "context":ctx(stale_loop_ctx,{"task_contract":{"kind":"improvement","project_uuid":"proj-replay","project_revision":"rev-7"},
   "minimal_audio_closure":{"project_uuid":"proj-replay","project_revision":"rev-7","hypothesis_frontier":{"candidate_id":"candidate:stale","candidates":[{"id":"candidate:stale","track_ids":["1007"]}]}},
   "free_state_capacity_assessment":{"schema_version":"free_state_capacity_assessment.v1","capacity_level":"within_free_state","selected_capability":"free_state_reasoning"}}),
 "responses":[
   json.dumps({"final":True,"reply":"bounded experiment.","free_state":{"schema_version":"free_state_decision.v1","status":"needs_experiment","evidence_status":"plausible","summary":"bounded experiment on stale refs",
     "improvement_proposal":{"schema_version":"improvement_proposal.v1","target":{"kind":"track","id":"1007"},"evidence_refs":["obs-replay-stale-target"],
       "improvement_intent":"clarify the relationship","hypothesis":"a small bounded change may help","expected_effect":"easier comparison",
       "action_domain":"track_gain","action_kind":"bounded_gain_adjustment","parameter_bounds":{"delta_db":-0.5},"confidence":0.5}},"tool_calls":[]}, ensure_ascii=False),
   needs_observation(["track.timbre_frequency"],"ccb-stale-2","Gate rejected the stale refs; request a fresh revision-bound view instead."),
   blocked("The stale refs were rejected by the admission gate; a fresh bounded view was requested and the loop ends at the boundary.",["stale refs rejected"]),
 ],
 "tool_results":[
   obs(["track.timbre_frequency"],"ready","obs-replay-stale-2"),
 ],
 "expected":{"terminal_status":"blocked","gate_rejection":"G7_fresh_revision_bound_refs","observation_calls":1},
}

F['contradictory.json']={
 "family":"contradictory","matrix_ids":["M16"],
 "source":src('20260823_145032',"synthetic_contradiction_variant: two rounds disagree; the replay must revisit with a discriminating view or close the candidate, never pass silently"),
 "intent":INTENT,
 "context":ctx(),
 "responses":[
   needs_observation(["track.basic_energy"],"ccb-contra-1","First round: energy view."),
   needs_observation(["track.peak_structure"],"ccb-contra-2","The two rounds conflict; revisit with a discriminating peak-structure view."),
   blocked("The contradiction stays recorded: the conflicting evidence is revisited, the candidate is not silently confirmed.",["contradiction revisited"]),
 ],
 "tool_results":[
   obs(["track.basic_energy"],"ready","obs-replay-contra-1",extra={"limitations":["conflicting level reports between adjacent views"]}),
   obs(["track.peak_structure"],"partial","obs-replay-contra-2",extra={"limitations":["peak structure incomplete over the conflicting window"]}),
 ],
 "expected":{"terminal_status":"blocked","observation_calls":2,"no_silent_pass":True},
}

F['transient_error.json']={
 "family":"transient-error","matrix_ids":["M17"],
 "source":src('20260823_120030',"synthetic_transient_variant: completer transport error after the first observation, then recovery resume"),
 "intent":INTENT,
 "context":ctx(),
 "responses":[
   needs_observation(["project.structure"],"ccb-trans-1","Need the project layout once."),
   blocked("Recovered after the transient transport failure; the already-executed observation was not repeated.",["transient recovery"]),
 ],
 "completer_error_at_turn":[1],
 "tool_results":[
   obs(["project.structure"],"ready","obs-replay-trans-1"),
 ],
 "expected":{"terminal_status":"blocked","observation_calls":1,"transient_stop_reason":"transient_llm_error"},
}

out='agent/internal/agentloop/testdata/free_state_replay'
os.makedirs(out, exist_ok=True)
for name,doc in F.items():
    io.open(os.path.join(out,name),'w',encoding='utf-8',newline='').write(json.dumps(doc,ensure_ascii=False,indent=2)+"\n")
print("wrote", len(F))
