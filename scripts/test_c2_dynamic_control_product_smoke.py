import tempfile
import unittest
from pathlib import Path

import c2_dynamic_control_product_smoke as smoke


class C2DynamicControlProductSmokeTest(unittest.TestCase):
    def test_authoritative_project_change_requires_changed_entities(self) -> None:
        self.assertTrue(smoke.authoritative_project_change({
            "authoritative": True,
            "changed_entities": [{"kind": "node", "id": "project"}],
        }))
        self.assertFalse(smoke.authoritative_project_change({"authoritative": True, "changed_entities": []}))
        self.assertFalse(smoke.authoritative_project_change({"authoritative": False, "changed_entities": [{}]}))

    def test_file_sha256_changes_with_content(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "project.vit"
            path.write_bytes(b"before")
            before = smoke.file_sha256(path)
            path.write_bytes(b"after")
            self.assertNotEqual(before, smoke.file_sha256(path))

    def test_validate_leaf_receipts_accepts_a_multi_leaf_batch(self) -> None:
        smoke.validate_leaf_receipts([
            {"status": "executed", "track_id": "kick", "plugin_id": "p1",
             "controller_result": {"status": "exact", "restore_ref": "r1"},
             "parameter_audit": {"status": "pass"}},
            {"status": "executed", "track_id": "bass", "plugin_id": "p2",
             "controller_result": {"status": "exact", "restore_ref": "r2"}},
        ])

    def test_validate_post_action_observations_accepts_multi_target_receipts(self) -> None:
        smoke.validate_post_action_observations([
            {"status": "observed", "requires_fresh_observation": False, "observation_id": "obs-1",
             "audit_receipt": {"status": "executed", "freshness": {"status": "ready"}}},
            {"status": "observed", "requires_fresh_observation": False, "observation_id": "obs-2",
             "audit_receipt": {"status": "partial", "freshness": {"status": "ready"}}},
        ])


if __name__ == "__main__":
    unittest.main()
