import argparse
import json
import sys
import tempfile
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import semantic_processor_project_smoke_evaluate as evaluate


class SmokeEvaluatorTest(unittest.TestCase):
    def make_run(self, root: Path, family: str, target: str, complete: bool = True) -> Path:
        response = {
            "status": "completed",
            "free_state": {
                "target_ref": {"id": target},
                "semantic_processor_intent": {"family": family},
            },
        }
        if complete:
            response.update({key: [{"receipt_id": key}] for key in evaluate.EVIDENCE_FIELDS})
        response_path = root / "response.json"
        response_path.write_text(json.dumps(response), encoding="utf-8")
        checkpoint_path = root / "checkpoint.json"
        checkpoint_path.write_text(json.dumps({"history": [{"state": state} for state in (
            "fixture_ready", "project_importing", "dad_waiting", "agent_started", "model_observing",
            "candidate_query", "model_identifier_selection", "preload_pca_recheck", "postload_qualification",
            "control_planning", "parameter_confirmation", "typed_execution", "readback_and_snapshot_verification",
            "model_post_action_observation", "model_outcome", "project_terminal",
        )]}), encoding="utf-8")
        sealed = {
            "schema_version": "semantic_processor_agent_project_smoke_sealed_truth.v1",
            "contract_id": "synthetic_contract",
            "fixture_set_id": "synthetic_fixture",
            "warning": "evaluator-only",
            "source_inventory": [],
            "cases": [{
                "public_case_id": "opaque_project",
                "issue_assignments": [{"issue_id": "opaque_issue", "expected_family": "static_eq", "expected_target_track": "track-A"}],
            }],
        }
        sealed_path = root / "sealed_truth.json"
        sealed_path.write_text(json.dumps(sealed), encoding="utf-8")
        run = {
            "schema_version": "semantic_processor_agent_project_smoke_run_report.v1",
            "contract_id": sealed["contract_id"],
            "fixture_set_id": sealed["fixture_set_id"],
            "run_id": "unit",
            "status": "completed",
            "started_at": "2026-01-01T00:00:00Z",
            "ended_at": "2026-01-01T00:00:01Z",
            "elapsed_seconds": 1,
            "sealed_truth_opened_by_runner": False,
            "projects": [{
                "public_case_id": "opaque_project",
                "conversation_id": "conversation",
                "original_intent_hash": "hash",
                "project_lifecycle": "completed",
                "model_turns": 1,
                "model_outcomes": ["satisfied"],
                "agent_conformance": "pass",
                "execution_evidence": {"response_artifacts": [str(response_path)]},
                "timeouts": [],
                "checkpoint_history": str(checkpoint_path),
                "terminal_state": "project_terminal",
            }],
            "family_coverage": {},
            "partial_report": {},
            "resume_continuation": {},
        }
        run_path = root / "run.json"
        run_path.write_text(json.dumps(run), encoding="utf-8")
        return run_path

    def evaluate_run(self, run_path: Path, root: Path) -> dict:
        output = root / "evaluation.json"
        evaluate.evaluate(argparse.Namespace(
            run_report=str(run_path),
            sealed_truth=str(root / "sealed_truth.json"),
            output=str(output),
        ))
        return json.loads(output.read_text(encoding="utf-8"))

    def test_full_path_matches_expected_family(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            report = self.evaluate_run(self.make_run(root, "static_eq", "track-A"), root)
            issue = next(row for row in report["issues"] if row["issue_id"] == "opaque_issue")
            self.assertEqual(issue["classification"], "matched_expected_family")
            self.assertEqual(issue["agent_conformance"], "pass")
            self.assertEqual(issue["execution_evidence_summary"]["ab_results"], 1)

    def test_alternative_family_is_evaluator_disagreement(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            report = self.evaluate_run(self.make_run(root, "limiter", "track-A"), root)
            issue = next(row for row in report["issues"] if row["issue_id"] == "opaque_issue")
            self.assertEqual(issue["classification"], "evaluator_disagreement")

    def test_missing_receipt_is_agent_conformance_failure(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            report = self.evaluate_run(self.make_run(root, "static_eq", "track-A", complete=False), root)
            issue = next(row for row in report["issues"] if row["issue_id"] == "opaque_issue")
            self.assertEqual(issue["classification"], "agent_conformance_failure")
            self.assertEqual(issue["agent_conformance"], "fail")

    def test_infrastructure_scope_is_preserved_in_taxonomy_and_summary(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            run_path = self.make_run(root, "static_eq", "track-A")
            run = json.loads(run_path.read_text(encoding="utf-8"))
            run["status"] = "infrastructure_failure"
            run["infrastructure_failures"] = [{
                "scope": "agent_model_service",
                "public_case_id": "opaque_project",
                "turn": 0,
                "error": "provider unavailable",
            }]
            run["projects"][0]["model_outcomes"] = ["inconclusive"]
            run_path.write_text(json.dumps(run), encoding="utf-8")
            report = self.evaluate_run(run_path, root)
            issue = next(row for row in report["issues"] if row["issue_id"] == "opaque_issue")
            self.assertEqual(issue["failure_taxonomy"]["kind"], "agent_model_service_failure")
            self.assertEqual(report["agent_conformance_summary"]["reason"], "agent_model_service_failure")


if __name__ == "__main__":
    unittest.main()
