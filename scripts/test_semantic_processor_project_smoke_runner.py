import sys
import unittest
import json
import tempfile
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import semantic_processor_project_smoke_runner as runner


class SmokeRunnerStatusTest(unittest.TestCase):
    def test_evidence_counts_reads_persisted_free_state_action_receipts(self):
        response = {
            "workflow_data": {
                "free_state_reasoning_loop": {
                    "actions": [{
                        "receipt": {
                            "executed_kernel_reply": [
                                {"tool": "mix.propose_tick", "status": "ok"},
                                {"tool": "mix.apply_tick", "status": "ok"},
                                {"tool": "mix.observe", "status": "ok"},
                            ],
                        },
                    }],
                },
            },
        }
        counts = runner.evidence_counts([response])
        self.assertEqual(counts["transaction_receipts"], 2)
        self.assertEqual(counts["post_action_observation_receipts"], 1)

    def test_diagnostic_transport_timeout_exceeds_agent_loop_budget(self):
        # The CLI diagnostic override must leave response headroom above the
        # Agent's bounded 210-second loop; otherwise urllib cancels the final
        # provider retry and loses the checkpoint response.
        runtime = runner.diagnostic_runtime_contract(Path(runner.__file__).with_name("semantic_processor_project_smoke_contract.json"))
        self.assertTrue(runtime["diagnostic_only"])
        self.assertEqual(runtime["limits"]["maximum_model_turns_per_project"], 6)
        self.assertGreater(runtime["limits"]["model_turn_timeout_seconds"], 210)

    def test_l3_proposal_runtime_is_bounded_and_neutral(self):
        runtime = runner.l3_proposal_runtime_contract(Path(runner.__file__).with_name("semantic_processor_project_smoke_contract.json"))
        self.assertTrue(runtime["l3_proposal_only"])
        self.assertEqual(runtime["limits"]["maximum_model_turns_per_project"], 6)
        self.assertIn("needs_experiment", runtime["initial_prompt"])

    def test_l3_full_workflow_runtime_continues_past_admission(self):
        runtime = runner.l3_full_workflow_runtime_contract(Path(runner.__file__).with_name("semantic_processor_project_smoke_contract.json"))
        self.assertTrue(runtime["l3_full_workflow"])
        self.assertFalse(runtime["l3_proposal_only"])
        self.assertEqual(runtime["limits"]["maximum_model_turns_per_project"], 18)
        self.assertIn("受治理", runtime["initial_prompt"])
        self.assertIn("局部改善机会", runtime["initial_prompt"])
        self.assertIn("不要求覆盖全工程", runtime["initial_prompt"])

    def test_recommended_interaction_submission_preserves_product_payload(self):
        response = {
            "needs_confirmation": True,
            "interaction_requests": [{
                "id": "interaction-1",
                "payload": {"proposal_id": "proposal-1", "request_context": {"goal_id": "goal-1"}},
                "actions": [
                    {"id": "cancel", "style": "secondary"},
                    {"id": "approve", "recommended": True, "style": "primary"},
                ],
            }],
        }
        submission, issue = runner.recommended_interaction_submission(response)
        self.assertEqual(issue, "")
        self.assertEqual(submission["interaction_id"], "interaction-1")
        self.assertEqual(submission["decision"], "approve")
        self.assertEqual(submission["payload"], response["interaction_requests"][0]["payload"])

    def test_interaction_without_recommended_non_cancel_action_stops(self):
        submission, issue = runner.recommended_interaction_submission({
            "needs_confirmation": True,
            "interaction_requests": [{"id": "interaction-1", "actions": [{"id": "cancel"}]}],
        })
        self.assertIsNone(submission)
        self.assertIn("recommended non-cancel", issue)

    def test_diagnostic_only_requires_evidence_backed_terminal_shape(self):
        response = {"free_state": {"diagnostic": {
            "schema_version": "free_state_diagnostic.v1", "status": "confirmed",
            "findings": [{"statement": "measurable difference", "evidence_refs": ["obs-1"]}],
        }}}
        self.assertEqual(runner.diagnostic_response_issue(response), "")
        response["free_state"]["diagnostic"]["findings"][0]["evidence_refs"] = []
        self.assertIn("evidence_refs", runner.diagnostic_response_issue(response))

    def test_diagnostic_only_rejects_family_selection(self):
        response = {"free_state": {
            "processor_type": "compressor",
            "diagnostic": {"schema_version": "free_state_diagnostic.v1", "status": "unresolved"},
        }}
        self.assertIn("processor family", runner.diagnostic_response_issue(response))

    def test_diagnostic_reads_real_chat_response_loop_and_requires_returned_evidence(self):
        observation = self.observation_response("ready")
        observation["executed_kernel_reply"][0]["result"] = {
            "bundle": {
                "status": "ready", "observation_id": "obs-1",
                "evidence_refs": ["evidence://mix"],
            },
        }
        terminal = {
            "goal_status": "completed",
            "workflow_data": {"free_state_reasoning_loop": {"latest_decision": {
                "status": "satisfied",
                "diagnostic": {
                    "schema_version": "free_state_diagnostic.v1", "status": "confirmed",
                    "findings": [{"statement": "bounded finding", "evidence_refs": ["obs-1", "evidence://mix"]}],
                },
            }}},
        }
        self.assertEqual(runner.diagnostic_completion_issue(terminal, [observation, terminal]), "")
        terminal["workflow_data"]["free_state_reasoning_loop"]["latest_decision"]["diagnostic"]["findings"][0]["evidence_refs"] = ["receipt-hidden"]
        self.assertIn("not returned", runner.diagnostic_completion_issue(terminal, [observation, terminal]))

    def test_diagnostic_completion_requires_observation_and_zero_control_activity(self):
        terminal = {"free_state": {"diagnostic": {
            "schema_version": "free_state_diagnostic.v1", "status": "unresolved",
        }}}
        self.assertIn("observation receipt", runner.diagnostic_completion_issue(terminal, [terminal]))
        observation = self.observation_response("ready")
        observation["executed_kernel_reply"][0]["result"]["observation_id"] = "obs-1"
        terminal["transaction_receipts"] = [{"receipt_id": "tx-1"}]
        self.assertIn("read-only boundary", runner.diagnostic_completion_issue(terminal, [observation, terminal]))

    def test_diagnostic_rejects_family_in_persisted_latest_decision(self):
        response = {"workflow_data": {"free_state_reasoning_loop": {"latest_decision": {
            "processor_type": "compressor",
            "diagnostic": {"schema_version": "free_state_diagnostic.v1", "status": "unresolved"},
        }}}}
        self.assertIn("processor family", runner.diagnostic_response_issue(response))

    def test_diagnostic_rejects_unknown_processor_declaration(self):
        response = {"workflow_data": {"free_state_reasoning_loop": {"latest_decision": {
            "processor_type": "future_unregistered_processor",
            "diagnostic": {"schema_version": "free_state_diagnostic.v1", "status": "unresolved"},
        }}}}
        self.assertIn("processor family", runner.diagnostic_response_issue(response))

    def test_godot_lifecycle_receipt_requires_all_owned_children_and_ports(self):
        receipt = {
            "schema_version": "semantic_processor_agent_project_smoke_godot_lifecycle_receipt.v1",
            "status": "passed",
            "product_lifecycle": "godot_project",
            "godot_autostart": [{"role": role} for role in ("kernel", "hub", "agent")],
            "ports": [{"port": port} for port in (5555, 5556, 7878, 8787)],
        }
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "godot_lifecycle_receipt.json"
            path.write_text(json.dumps(receipt), encoding="utf-8")
            self.assertEqual(runner.inspect_godot_lifecycle(path)["status"], "passed")
    def observation_response(self, result_status="ready"):
        return {
            "goal_status": "completed",
            "stop_reason": "done",
            "executed_kernel_reply": [{
                "tool": "ccb.observation_request",
                "status": "ok",
                "result": {
                    "status": result_status,
                    "audit_receipt": {"status": result_status},
                },
            }],
        }

    def test_blocked_free_state_wins_over_completed_goal(self):
        response = self.observation_response("partial")
        response["free_state"] = {"status": "blocked"}
        self.assertEqual(runner.response_status(response), "blocked")

    def test_partial_observation_is_model_blocked_not_satisfied(self):
        self.assertEqual(runner.response_status(self.observation_response("partial")), "model_blocked")

    def test_diagnostic_only_limit_without_conclusion_is_inconclusive(self):
        response = {
            "goal_status": "waiting_continue",
            "stop_reason": "max_turns",
            "free_state": {"status": "needs_observation"},
            "executed_kernel_reply": [{
                "tool": "ccb.observation_request",
                "status": "ok",
                "result": {"status": "ready"},
            }],
        }
        self.assertEqual(runner.response_status(response), "inconclusive")

    def test_ready_observation_without_mutation_is_model_no_op(self):
        self.assertEqual(runner.response_status(self.observation_response("ready")), "model_no_op")

    def test_completed_with_governed_mutation_remains_completed(self):
        response = self.observation_response("ready")
        response["transaction_receipts"] = [{"receipt_id": "tx-1"}]
        self.assertEqual(runner.response_status(response), "completed")

    def test_project_state_only_completion_is_model_blocked(self):
        response = {
            "goal_status": "completed",
            "stop_reason": "done",
            "executed_kernel_reply": [{
                "tool": "project.state",
                "command_name": "get_project_state",
                "status": "ok",
                "result": {"status": "ok", "track_count": 6},
            }],
        }
        self.assertEqual(runner.response_status(response), "model_blocked")

    def test_satisfied_without_model_requested_ccb_observation_is_blocked(self):
        response = {
            "goal_status": "completed",
            "stop_reason": "done",
            "free_state": {"status": "satisfied", "evidence_status": "sufficient"},
            "executed_kernel_reply": [{
                "tool": "project.state",
                "status": "ok",
                "result": {"status": "ok"},
            }],
        }
        self.assertEqual(runner.response_status(response), "model_blocked")

    def test_agent_model_service_failure_is_not_model_evidence(self):
        response = {
            "goal_status": "failed",
            "stop_reason": "failed",
            "error": "LLM HTTP error 502: error code: 502",
        }
        self.assertEqual(runner.agent_model_service_failure(response), "LLM HTTP error 502: error code: 502")

    def test_windows_provider_read_timeout_is_infrastructure_failure(self):
        error = (
            'Post "https://opencode.ai/zen/go/v1/chat/completions": read tcp '
            "192.168.1.181:53530->172.65.90.20:443: wsarecv: A connection attempt "
            "failed because the connected host has failed to respond"
        )
        response = {"goal_status": "failed", "stop_reason": "failed", "error": error}
        self.assertEqual(runner.agent_model_service_failure(response), error)

    def test_agent_model_resource_budget_failure_is_not_model_evidence(self):
        response = {
            "goal_status": "failed",
            "stop_reason": "failed",
            "error": "当前请求体并发内存预算不足，请稍后重试",
        }
        self.assertEqual(runner.agent_model_service_failure(response), response["error"])

    def test_transient_llm_stop_is_infrastructure_failure(self):
        self.assertEqual(
            runner.agent_model_service_failure({"stop_reason": "transient_llm_error"}),
            "transient_llm_error",
        )

    def test_agent_transport_failure_is_infrastructure_failure(self):
        self.assertEqual(
            runner.agent_model_service_failure({"stop_reason": "transport_failure"}),
            "transport_failure",
        )

    def test_failed_goal_status_is_terminal_error(self):
        self.assertEqual(
            runner.response_status({"goal_status": "failed", "reply": "governed router failed"}),
            "failed",
        )

    def test_agent_model_service_resume_is_bounded_by_contract_limit(self):
        self.assertTrue(runner.agent_model_service_resume_allowed("transient_llm_error", 0, 2))
        self.assertTrue(runner.agent_model_service_resume_allowed("transient_llm_error", 1, 2))
        self.assertFalse(runner.agent_model_service_resume_allowed("transient_llm_error", 2, 2))
        self.assertFalse(runner.agent_model_service_resume_allowed("", 0, 2))

    def test_candidate_frontier_requires_selection_and_execution_direction(self):
        response = {"workflow_data": {"minimal_audio_closure": {
            "hypothesis_frontier": {"candidates": [{"id": "candidate-1", "track_ids": ["1007"]}]},
            "settlement": {"reason": "round_limit"},
        }}}
        metrics = runner.minimal_closure_metrics([response])
        self.assertTrue(metrics["candidate_discovered"])
        self.assertFalse(metrics["execution_direction_reached"])
        self.assertEqual(metrics["failure_reason"], "terminated_without_execution_direction")
        self.assertEqual(runner.preliminary_agent_conformance([self.observation_response("ready"), response], "model_blocked", 0, []), "fail")

    def test_selected_candidate_and_needs_action_forms_execution_direction(self):
        response = {"workflow_data": {"minimal_audio_closure": {
            "hypothesis_frontier": {"candidate_id": "candidate-1", "candidates": [{"id": "candidate-1", "track_ids": ["1007"]}]},
        }, "free_state_reasoning_loop": {"latest_decision": {"status": "needs_action"}}}}
        metrics = runner.minimal_closure_metrics([response])
        self.assertTrue(metrics["candidate_selected"])
        self.assertTrue(metrics["needs_action"])
        self.assertTrue(metrics["execution_direction_reached"])

    def test_needs_experiment_forms_l3_execution_direction(self):
        response = {"workflow_data": {"minimal_audio_closure": {
            "hypothesis_frontier": {"candidate_id": "candidate-1", "candidates": [{"id": "candidate-1", "track_ids": ["1007"]}]},
        }, "free_state_reasoning_loop": {"latest_decision": {"status": "needs_experiment"}}}}
        metrics = runner.minimal_closure_metrics([response])
        self.assertTrue(metrics["needs_experiment"])
        self.assertTrue(metrics["execution_direction_reached"])

    def test_l3_improvement_proposal_requires_pending_confirmation_and_zero_mutation(self):
        response = {
            "needs_confirmation": True,
            "workflow_data": {"typed_state": {"candidate_type": "improvement_proposal"}},
            "free_state": {
                "status": "needs_experiment",
                "improvement_proposal": {
                    "schema_version": "improvement_proposal.v1",
                    "target": {"kind": "track", "id": "vocals"},
                    "evidence_refs": ["obs-1"],
                    "improvement_intent": "bring the vocal forward",
                    "hypothesis": "a bounded change may improve front-back perception",
                    "expected_effect": "clearer vocal placement",
                    "action_domain": "track_gain",
                    "action_kind": "bounded_gain_adjustment",
                },
            },
        }
        metrics = runner.improvement_proposal_metrics([response])
        self.assertTrue(metrics["admitted"])
        response["transaction_receipts"] = [{"receipt_id": "tx-1"}]
        self.assertFalse(runner.improvement_proposal_metrics([response])["admitted"])

    def test_selected_candidate_and_blocked_summary_forms_preflight_boundary(self):
        response = {"workflow_data": {"minimal_audio_closure": {
            "hypothesis_frontier": {"candidate_id": "candidate-1", "candidates": [{"id": "candidate-1", "track_ids": ["1007"]}]},
            "settlement": {"reason": "insufficient_evidence"},
        }, "free_state_reasoning_loop": {"latest_decision": {
            "status": "blocked", "evidence_status": "insufficient", "summary": "Target evidence is insufficient for a safe action.",
        }}}}
        metrics = runner.minimal_closure_metrics([response])
        self.assertTrue(metrics["candidate_selected"])
        self.assertTrue(metrics["action_preflight_boundary"])
        self.assertTrue(metrics["execution_direction_reached"])


if __name__ == "__main__":
    unittest.main()
