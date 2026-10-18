import json,sys
p=sys.argv[1]
d=json.load(open(p,encoding="utf-8"))
v=d.get("validation") or {}
park=d.get("fs_settle_terminal_park") or {}
mq=d.get("material_qualification") or {}
exp=((d.get("agent_runtime") or {}).get("free_state_loops") or {})
loops=list(exp.values()) if isinstance(exp,dict) else []
loop=loops[0] if loops else {}
out={
 "status":d.get("status"),"error":d.get("error"),"reason":d.get("reason"),
 "started_at":d.get("started_at"),"finished_at":d.get("finished_at"),
 "conversation_id":d.get("conversation_id"),"prompt_flavor":d.get("prompt_flavor"),"expect_domain":d.get("expect_domain"),
 "project_setup":{k:(d.get("project_setup") or {}).get(k) for k in ("reused_live_binding","dad_fact_status","dad_fact_ready_count","dad_fact_total_count")},
 "validation":{"status":v.get("status"),"action_domain":v.get("action_domain"),"target_ref":v.get("target_ref"),
   "before_revision":v.get("before_revision"),"after_revision":v.get("after_revision"),
   "transaction_id":v.get("transaction_id"),"readback_value":v.get("readback_value"),
   "post_action_observation_id":v.get("post_action_observation_id"),
   "forward_mutation_count":v.get("forward_mutation_count"),"human_audition_ready":v.get("human_audition_ready"),
   "human_confirmed":v.get("human_confirmed")},
 "fs_settle_terminal_park":park,
 "material_qualification_status":mq.get("status"),
 "free_state_loop":{"status":loop.get("status"),"last_error":loop.get("last_error"),
   "latest_decision":loop.get("latest_decision")},
}
print(json.dumps(out,ensure_ascii=False,indent=2))
