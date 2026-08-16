#!/usr/bin/env python3
import os
import shutil
import json
import unittest
import hashlib
from collect_evidence import (
    evaluate_gate_a, evaluate_gate_b, evaluate_gate_c,
    evaluate_gate_d, evaluate_gate_e, evaluate_gate_f, evaluate_gate_g,
    generate_file_hashes, compute_sha256
)

class TestFalsePassPrevention(unittest.TestCase):
    def setUp(self):
        self.test_dir = os.path.join("scratch", "test_evidence")
        os.makedirs(self.test_dir, exist_ok=True)

    def tearDown(self):
        if os.path.exists(self.test_dir):
            shutil.rmtree(self.test_dir)

    def test_zero_devices_returns_not_run(self):
        devices = []
        self.assertEqual(evaluate_gate_a(devices, self.test_dir), "NOT_RUN")
        self.assertEqual(evaluate_gate_b(devices, self.test_dir), "NOT_RUN")
        self.assertEqual(evaluate_gate_c(devices, self.test_dir), "NOT_RUN")
        self.assertEqual(evaluate_gate_d(devices, self.test_dir), "NOT_RUN")
        self.assertEqual(evaluate_gate_e(devices, self.test_dir), "NOT_RUN")
        self.assertEqual(evaluate_gate_f(devices, self.test_dir), "NOT_RUN")
        self.assertEqual(evaluate_gate_g(devices, self.test_dir), "NOT_RUN")

    def test_gate_b_valid_artifacts_pass(self):
        devices = [{"serial": "dev1"}]
        sc_file = os.path.join(self.test_dir, "sc_tap.png")
        with open(sc_file, "wb") as f:
            f.write(b"PNG_DUMMY_TAP_SCREENSHOT_DATA")
        sc_hash = compute_sha256(sc_file)

        vid_file = os.path.join(self.test_dir, "vid_app.mp4")
        with open(vid_file, "wb") as f:
            f.write(b"MP4_DUMMY_APP_SWITCH_VIDEO_DATA")
        vid_hash = compute_sha256(vid_file)

        commands = []
        actions = ["tap", "swipe", "HOME", "BACK", "APP_SWITCH"]
        for act in actions:
            cmd = {
                "action": act,
                "accepted": True,
                "dispatched": True,
                "ack_at": 100,
                "executing_at": 105,
                "succeeded_at": 110,
                "browser_event_logged": True,
                "screenshot_path": "sc_tap.png",
                "screenshot_hash": sc_hash,
                "video_path": "vid_app.mp4",
                "video_hash": vid_hash
            }
            commands.append(cmd)

        with open(os.path.join(self.test_dir, "command_journal.json"), "w") as f:
            json.dump({"commands": commands}, f)

        self.assertEqual(evaluate_gate_b(devices, self.test_dir), "PASS")

    def test_gate_b_negative_mutation_missing_file_fails(self):
        devices = [{"serial": "dev1"}]
        commands = [{
            "action": "tap",
            "accepted": True,
            "dispatched": True,
            "ack_at": 100,
            "executing_at": 105,
            "succeeded_at": 110,
            "browser_event_logged": True,
            "screenshot_path": "non_existent_sc.png",
            "screenshot_hash": "a" * 64
        }]
        with open(os.path.join(self.test_dir, "command_journal.json"), "w") as f:
            json.dump({"commands": commands}, f)

        self.assertEqual(evaluate_gate_b(devices, self.test_dir), "FAIL")

    def test_gate_b_negative_mutation_hash_mismatch_fails(self):
        devices = [{"serial": "dev1"}]
        sc_file = os.path.join(self.test_dir, "sc_tap.png")
        with open(sc_file, "wb") as f:
            f.write(b"PNG_DUMMY_TAP_SCREENSHOT_DATA")

        commands = [{
            "action": "tap",
            "accepted": True,
            "dispatched": True,
            "ack_at": 100,
            "executing_at": 105,
            "succeeded_at": 110,
            "browser_event_logged": True,
            "screenshot_path": "sc_tap.png",
            "screenshot_hash": "wrong_sha256_hash"
        }]
        with open(os.path.join(self.test_dir, "command_journal.json"), "w") as f:
            json.dump({"commands": commands}, f)

        self.assertEqual(evaluate_gate_b(devices, self.test_dir), "FAIL")

    def test_gate_b_negative_mutation_path_escape_fails(self):
        devices = [{"serial": "dev1"}]
        commands = [{
            "action": "tap",
            "accepted": True,
            "dispatched": True,
            "ack_at": 100,
            "executing_at": 105,
            "succeeded_at": 110,
            "browser_event_logged": True,
            "screenshot_path": "../../../etc/passwd",
            "screenshot_hash": "a" * 64
        }]
        with open(os.path.join(self.test_dir, "command_journal.json"), "w") as f:
            json.dump({"commands": commands}, f)

        self.assertEqual(evaluate_gate_b(devices, self.test_dir), "FAIL")

    def test_gate_d_valid_candidate_pair_and_progression_passes(self):
        devices = [{"serial": "dev1"}]
        direct_data = {
            "samples": [
                {
                    "bytesReceived": 1000,
                    "framesDecoded": 10,
                    "reports": {
                        "transport_1": {"type": "transport", "selectedCandidatePairId": "pair_1"},
                        "pair_1": {"type": "candidate-pair", "localCandidateId": "cand_host_1"},
                        "cand_host_1": {"type": "local-candidate", "candidateType": "host"}
                    }
                },
                {
                    "bytesReceived": 50000,
                    "framesDecoded": 200,
                    "reports": {
                        "transport_1": {"type": "transport", "selectedCandidatePairId": "pair_1"},
                        "pair_1": {"type": "candidate-pair", "localCandidateId": "cand_host_1"},
                        "cand_host_1": {"type": "local-candidate", "candidateType": "host"}
                    }
                }
            ]
        }
        turn_data = {
            "samples": [
                {
                    "bytesReceived": 1000,
                    "framesDecoded": 10,
                    "reports": {
                        "transport_1": {"type": "transport", "selectedCandidatePairId": "pair_turn"},
                        "pair_turn": {"type": "candidate-pair", "localCandidateId": "cand_relay_1"},
                        "cand_relay_1": {"type": "local-candidate", "candidateType": "relay"}
                    }
                },
                {
                    "bytesReceived": 50000,
                    "framesDecoded": 200,
                    "reports": {
                        "transport_1": {"type": "transport", "selectedCandidatePairId": "pair_turn"},
                        "pair_turn": {"type": "candidate-pair", "localCandidateId": "cand_relay_1"},
                        "cand_relay_1": {"type": "local-candidate", "candidateType": "relay"}
                    }
                }
            ]
        }

        with open(os.path.join(self.test_dir, "webrtc_stats.json"), "w") as f:
            json.dump(direct_data, f)
        with open(os.path.join(self.test_dir, "webrtc_turn_stats.json"), "w") as f:
            json.dump(turn_data, f)

        self.assertEqual(evaluate_gate_d(devices, self.test_dir), "PASS")

    def test_gate_d_negative_mutation_synthetic_direct_string_fails(self):
        devices = [{"serial": "dev1"}]
        direct_data = {
            "samples": [
                {"candidateType": "direct", "bytesReceived": 100, "framesDecoded": 1},
                {"candidateType": "direct", "bytesReceived": 1000, "framesDecoded": 10}
            ]
        }
        turn_data = {
            "samples": [
                {"candidateType": "relay", "bytesReceived": 100, "framesDecoded": 1},
                {"candidateType": "relay", "bytesReceived": 1000, "framesDecoded": 10}
            ]
        }

        with open(os.path.join(self.test_dir, "webrtc_stats.json"), "w") as f:
            json.dump(direct_data, f)
        with open(os.path.join(self.test_dir, "webrtc_turn_stats.json"), "w") as f:
            json.dump(turn_data, f)

        # Synthetic "direct" candidate type is REJECTED
        self.assertEqual(evaluate_gate_d(devices, self.test_dir), "FAIL")

    def test_gate_d_negative_mutation_single_sample_fails(self):
        devices = [{"serial": "dev1"}]
        direct_data = {
            "samples": [
                {
                    "bytesReceived": 50000,
                    "framesDecoded": 200,
                    "reports": {
                        "transport_1": {"type": "transport", "selectedCandidatePairId": "pair_1"},
                        "pair_1": {"type": "candidate-pair", "localCandidateId": "cand_host_1"},
                        "cand_host_1": {"type": "local-candidate", "candidateType": "host"}
                    }
                }
            ]
        }
        turn_data = {
            "samples": [
                {
                    "bytesReceived": 50000,
                    "framesDecoded": 200,
                    "reports": {
                        "transport_1": {"type": "transport", "selectedCandidatePairId": "pair_turn"},
                        "pair_turn": {"type": "candidate-pair", "localCandidateId": "cand_relay_1"},
                        "cand_relay_1": {"type": "local-candidate", "candidateType": "relay"}
                    }
                }
            ]
        }

        with open(os.path.join(self.test_dir, "webrtc_stats.json"), "w") as f:
            json.dump(direct_data, f)
        with open(os.path.join(self.test_dir, "webrtc_turn_stats.json"), "w") as f:
            json.dump(turn_data, f)

        self.assertEqual(evaluate_gate_d(devices, self.test_dir), "FAIL")

    def test_gate_g_passes_with_complete_raw_package(self):
        devices = [{"serial": "dev1"}]
        files = [
            "manifest.json", "PHASE-1.7-ACCEPTANCE.md", "webrtc_stats.json",
            "webrtc_turn_stats.json", "command_journal.json", "fleet_lifecycle_evidence.json",
            "security_evidence.json", "scale_evidence.json", "logcat_dev1.log"
        ]
        for fn in files:
            with open(os.path.join(self.test_dir, fn), "wb") as f:
                f.write(b'{"test": true}')

        generate_file_hashes(self.test_dir)
        self.assertEqual(evaluate_gate_g(devices, self.test_dir), "PASS")

    def test_gate_g_negative_mutation_missing_logcat_fails(self):
        devices = [{"serial": "dev1"}]
        files = [
            "manifest.json", "PHASE-1.7-ACCEPTANCE.md", "webrtc_stats.json",
            "webrtc_turn_stats.json", "command_journal.json", "fleet_lifecycle_evidence.json",
            "security_evidence.json", "scale_evidence.json"
            # Missing logcat_dev1.log
        ]
        for fn in files:
            with open(os.path.join(self.test_dir, fn), "wb") as f:
                f.write(b'{"test": true}')

        generate_file_hashes(self.test_dir)
        self.assertEqual(evaluate_gate_g(devices, self.test_dir), "FAIL")

    def test_gate_g_negative_mutation_corrupt_hash_fails(self):
        devices = [{"serial": "dev1"}]
        files = [
            "manifest.json", "PHASE-1.7-ACCEPTANCE.md", "webrtc_stats.json",
            "webrtc_turn_stats.json", "command_journal.json", "fleet_lifecycle_evidence.json",
            "security_evidence.json", "scale_evidence.json", "logcat_dev1.log"
        ]
        for fn in files:
            with open(os.path.join(self.test_dir, fn), "wb") as f:
                f.write(b'{"test": true}')

        generate_file_hashes(self.test_dir)

        # Corrupt security_evidence.json artifact
        with open(os.path.join(self.test_dir, "security_evidence.json"), "wb") as f:
            f.write(b'{"tampered": true}')

        self.assertEqual(evaluate_gate_g(devices, self.test_dir), "FAIL")

if __name__ == "__main__":
    unittest.main()
