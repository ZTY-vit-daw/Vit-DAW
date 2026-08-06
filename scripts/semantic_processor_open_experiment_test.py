from __future__ import annotations

import unittest
from unittest import mock

import semantic_processor_open_experiment as experiment


class SemanticProcessorOpenExperimentTest(unittest.TestCase):
    @mock.patch.object(experiment, "request_json")
    def test_active_free_state_waiting_continue_resumes_same_conversation(self, request_json: mock.Mock) -> None:
        request_json.return_value = {"goal_status": "completed"}
        response = {
            "goal_id": "goal-1",
            "run_id": "run-1",
            "goal_status": "waiting_continue",
            "stop_reason": "transient_llm_error",
            "workflow_data": {
                "free_state_reasoning_loop": {
                    "schema_version": "free_state_reasoning_loop.v1",
                    "status": "observing",
                    "original_intent": "make the vocal steadier and more forward",
                }
            },
        }

        next_response, record = experiment.advance(
            "http://127.0.0.1:7878",
            "conversation-1",
            {"selected_track_id": "track-vocal"},
            response,
            10.0,
        )

        self.assertEqual(next_response, {"goal_status": "completed"})
        self.assertEqual(record["transition"], "continuation")
        self.assertEqual(record["stop_reason"], "transient_llm_error")
        request_json.assert_called_once_with(
            "POST",
            "http://127.0.0.1:7878/agent/chat",
            {
                "conversation_id": "conversation-1",
                "message": experiment.CONTINUE_PROMPT,
                "context": {
                    "selected_track_id": "track-vocal",
                    "goal_id": "goal-1",
                    "run_id": "run-1",
                },
            },
            10.0,
        )

    @mock.patch.object(experiment, "request_json")
    def test_terminal_free_state_is_not_resumed(self, request_json: mock.Mock) -> None:
        response = {
            "goal_status": "waiting_continue",
            "workflow_data": {
                "free_state_reasoning_loop": {
                    "schema_version": "free_state_reasoning_loop.v1",
                    "status": "blocked",
                }
            },
        }

        next_response, record = experiment.advance(
            "http://127.0.0.1:7878", "conversation-1", {}, response, 10.0
        )

        self.assertIsNone(next_response)
        self.assertEqual(record["transition"], "terminal")
        request_json.assert_not_called()

    @mock.patch.object(experiment, "request_json")
    def test_non_transient_waiting_continue_is_not_auto_resumed(self, request_json: mock.Mock) -> None:
        response = {
            "goal_status": "waiting_continue",
            "stop_reason": "limit_reached",
            "workflow_data": {
                "free_state_reasoning_loop": {
                    "schema_version": "free_state_reasoning_loop.v1",
                    "status": "observing",
                    "original_intent": "make the vocal steadier",
                }
            },
        }

        next_response, record = experiment.advance(
            "http://127.0.0.1:7878", "conversation-1", {}, response, 10.0
        )

        self.assertIsNone(next_response)
        self.assertEqual(record["transition"], "paused")
        self.assertEqual(record["stop_reason"], "limit_reached")
        request_json.assert_not_called()


if __name__ == "__main__":
    unittest.main()
