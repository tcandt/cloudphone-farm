#!/usr/bin/env python3
import os
import shutil
import json
import unittest
from collect_evidence import (
    evaluate_gate_a, evaluate_gate_b, evaluate_gate_c,
    evaluate_gate_d, evaluate_gate_e, evaluate_gate_f, evaluate_gate_g
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

    def test_devices_present_without_evidence_returns_fail(self):
        devices = [{"serial": "dev1", "model": "Pixel 6", "android_version": "14", "api_level": "34"}]
        # Devices attached alone MUST NEVER return PASS
        self.assertEqual(evaluate_gate_a(devices, self.test_dir), "FAIL") # Requires 3 devices
        self.assertEqual(evaluate_gate_b(devices, self.test_dir), "FAIL") # Missing command_journal.json
        self.assertEqual(evaluate_gate_c(devices, self.test_dir), "FAIL") # Missing webrtc_stats.json
        self.assertEqual(evaluate_gate_d(devices, self.test_dir), "FAIL") # Missing webrtc_stats.json & turn stats
        self.assertEqual(evaluate_gate_e(devices, self.test_dir), "FAIL") # Missing security_evidence.json
        self.assertEqual(evaluate_gate_f(devices, self.test_dir), "FAIL") # Requires 3 devices + scale_evidence.json
        self.assertEqual(evaluate_gate_g(devices, self.test_dir), "FAIL") # Missing required artifact package

    def test_valid_webrtc_h264_multi_sample_passes_gate_c(self):
        devices = [{"serial": "dev1"}]
        stats_data = {
            "samples": [
                {"codecMimeType": "video/H264", "framesDecoded": 10, "bytesReceived": 10000, "first_frame_rendered": True},
                {"codecMimeType": "video/H264", "framesDecoded": 150, "bytesReceived": 204800, "first_frame_rendered": True}
            ]
        }
        with open(os.path.join(self.test_dir, "webrtc_stats.json"), "w") as f:
            json.dump(stats_data, f)

        self.assertEqual(evaluate_gate_c(devices, self.test_dir), "PASS")

    def test_single_sample_returns_fail_for_gate_c(self):
        devices = [{"serial": "dev1"}]
        stats_data = {
            "samples": [
                {"codecMimeType": "video/H264", "framesDecoded": 150, "bytesReceived": 204800, "first_frame_rendered": True}
            ]
        }
        with open(os.path.join(self.test_dir, "webrtc_stats.json"), "w") as f:
            json.dump(stats_data, f)

        self.assertEqual(evaluate_gate_c(devices, self.test_dir), "FAIL") # Requires >= 2 time samples with progression

    def test_valid_dual_sample_passes_gate_d(self):
        devices = [{"serial": "dev1"}]
        direct_data = {"candidateType": "direct", "bytesReceived": 102400}
        turn_data = {"candidateType": "relay", "bytesReceived": 102400, "framesDecoded": 100}

        with open(os.path.join(self.test_dir, "webrtc_stats.json"), "w") as f:
            json.dump(direct_data, f)
        with open(os.path.join(self.test_dir, "webrtc_turn_stats.json"), "w") as f:
            json.dump(turn_data, f)

        self.assertEqual(evaluate_gate_d(devices, self.test_dir), "PASS")

if __name__ == "__main__":
    unittest.main()
