import json,sys
d=json.load(open(sys.argv[1],encoding="utf-8"))
loop=d.get("persisted_loop") or {}
exp=loop.get("experiment") or {}
rounds=exp.get("rounds") or []
out={
 "status":d.get("status"),
 "error":d.get("error"),
 "conversation_id":d.get("conversation_id"),
 "responses_count":len(d.get("responses") or []),
 "continuations_count":len(d.get("continuations") or []),
 "continuation_timeline":d.get("continuation_timeline"),
 "terminal_causes":d.get("terminal_causes"),
 "last_runtime_status":d.get("last_runtime_status"),
 "fs_settle_terminal_park":d.get("fs_settle_terminal_park"),
 "persisted_loop":{
   "status":loop.get("status"),
   "phase":loop.get("phase"),
   "last_error":loop.get("last_error"),
   "latest_decision":loop.get("latest_decision"),
   "d1_receipt":{k:loop.get("d1_receipt",{}).get(k) for k in ("human_audition_ready","human_confirmed","settled","parameter_applied","readback_verified","forward_mutation_count")} if isinstance(loop.get("d1_receipt"),dict) else None,
   "experiment":{"status":exp.get("status"),"task_state":exp.get("task_state"),"turn_id":exp.get("turn_id")},
   "rounds":[{"round_id":r.get("round_id"),"status":r.get("status"),"phase":r.get("phase"),
              "has_materiality":r.get("materiality") is not None,
              "has_target_response":r.get("target_response") is not None,
              "observations":len(r.get("observations") or [])} for r in rounds],
 }
}
print(json.dumps(out,ensure_ascii=False,indent=2))
