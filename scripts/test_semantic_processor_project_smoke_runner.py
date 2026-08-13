import sys
import unittest
import json
import tempfile
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import semantic_processor_project_smoke_runner as runner


class SmokeRunnerStatusTest(unittest.TestCase):
    def test_diagnostic_transport_timeout_exceeds_agent_loop_budget(self):
        # The CLI diagnostic override must leave response headroom above the
        # Agent's bounded 210-second loop; otherwise urllib cancels the final
        # provider retry and loses the checkpoint response.
        runtime = runner.diagnostic_runtime_contract(Path(runner.__file__).with_name("semantic_processor_project_smoke_contract.json"))
        self.assertTrue(runtime["diagnostic_only"])
        self.assertEqual(runtime["limits"]["maximum_model_turns_per_project"], 6)
        self.assertGreater(runtime["limits"]["model_turn_timeout_seconds"], 210)

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

    def test_agent_model_service_resume_is_bounded_by_contract_limit(self):
        self.assertTrue(runner.agent_model_service_resume_allowed("transient_llm_error", 0, 2))
        self.assertTrue(runner.agent_model_service_resume_allowed("transient_llm_error", 1, 2))
        self.assertFalse(runner.agent_model_service_resume_allowed("transient_llm_error", 2, 2))
        self.assertFalse(runner.agent_model_service_resume_allowed("", 0, 2))


if __name__ == "__main__":
    unittest.main()
