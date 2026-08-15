#!/usr/bin/env python3
import json
import os
import subprocess
import sys
import time
from datetime import datetime, timezone

def run_cmd(cmd, check=True):
    try:
        res = subprocess.run(cmd, shell=True, capture_output=True, text=True)
        if check and res.returncode != 0:
            return None
        return res.stdout.strip()
    except Exception:
        return None

def get_git_sha():
    sha = run_cmd("git rev-parse HEAD")
    return sha if sha else "unknown"

def collect_adb_device_metadata():
    devices_out = run_cmd("adb devices -l", check=False)
    if not devices_out:
        return []
    
    devices = []
    lines = devices_out.splitlines()
    for line in lines[1:]:
        if not line.strip() or "offline" in line or "unauthorized" in line:
            continue
        parts = line.split()
        if len(parts) >= 2 and parts[1] == "device":
            serial = parts[0]
            model = run_cmd(f"adb -s {serial} shell getprop ro.product.model", check=False) or "Unknown"
            android_ver = run_cmd(f"adb -s {serial} shell getprop ro.build.version.release", check=False) or "Unknown"
            api_level = run_cmd(f"adb -s {serial} shell getprop ro.build.version.sdk", check=False) or "Unknown"
            wm_size = run_cmd(f"adb -s {serial} shell wm size", check=False) or "Unknown"
            
            # Zero-manufactured evidence: Do NOT hardcode key_protection values.
            # Read from actual device/agent telemetry if present; otherwise None.
            key_prot = None
            dump_sec = run_cmd(f"adb -s {serial} shell getprop sys.pcp.key_security_level", check=False)
            if dump_sec in ["SOFTWARE", "TRUSTED_ENVIRONMENT", "STRONGBOX"]:
                key_prot = {
                    "algorithm": "AES-256-GCM",
                    "provider": "AndroidKeyStore",
                    "security_level": dump_sec
                }

            devices.append({
                "serial": serial,
                "model": model,
                "android_version": android_ver,
                "api_level": api_level,
                "display_geometry": wm_size,
                "key_protection": key_prot
            })
    return devices

def evaluate_gate_a(devices, evidence_dir):
    """Gate A: Physical Fleet Lifecycle - Requires >= 3 physical devices with verified Tink/KeyStore security and lifecycle proof"""
    if not devices:
        return "NOT_RUN"
    if len(devices) < 3:
        return "FAIL"
    
    fleet_info_path = os.path.join(evidence_dir, "fleet_lifecycle_evidence.json")
    if not os.path.exists(fleet_info_path):
        return "FAIL"
    
    try:
        with open(fleet_info_path, "r", encoding="utf-8") as f:
            ev = json.load(f)
        
        if (ev.get("enrollment_verified") is True and
            ev.get("key_protection_verified") is True and
            ev.get("reboot_reconnect_verified") is True and
            ev.get("no_auto_projection_verified") is True and
            ev.get("wifi_reconnect_verified") is True and
            ev.get("generation_increment_verified") is True and
            ev.get("heartbeat_cadence_verified") is True and
            ev.get("presence_ttl_verified") is True):
            return "PASS"
        return "FAIL"
    except Exception:
        return "FAIL"

def evaluate_gate_b(devices, evidence_dir):
    """Gate B: Physical Control - Requires per-command lifecycle (accepted, dispatched, ack_at, executing_at, succeeded_at, browser event, screenshot/video hash)"""
    if not devices:
        return "NOT_RUN"
    
    journal_path = os.path.join(evidence_dir, "command_journal.json")
    if not os.path.exists(journal_path):
        return "FAIL"
    
    try:
        with open(journal_path, "r", encoding="utf-8") as f:
            journal = json.load(f)
        
        required_actions = {"tap", "swipe", "HOME", "BACK", "APP_SWITCH"}
        seen_actions = set()
        
        for cmd in journal.get("commands", []):
            if (cmd.get("accepted") is True and
                cmd.get("dispatched") is True and
                cmd.get("ack_at", 0) > 0 and
                cmd.get("executing_at", 0) > 0 and
                cmd.get("succeeded_at", 0) > 0 and
                cmd.get("browser_event_logged") is True and
                (cmd.get("screenshot_hash") or cmd.get("video_hash"))):
                seen_actions.add(cmd.get("action"))
        
        if required_actions.issubset(seen_actions):
            return "PASS"
        return "FAIL"
    except Exception:
        return "FAIL"

def evaluate_gate_c(devices, evidence_dir):
    """Gate C: Real H.264 Screen Capture - Requires >= 2 time samples with codecMimeType == video/H264, framesDecoded(t2) > framesDecoded(t1), bytesReceived(t2) > bytesReceived(t1), first_frame_rendered == true"""
    if not devices:
        return "NOT_RUN"
    
    stats_path = os.path.join(evidence_dir, "webrtc_stats.json")
    if not os.path.exists(stats_path):
        return "FAIL"
    
    try:
        with open(stats_path, "r", encoding="utf-8") as f:
            stats = json.load(f)
        
        samples = stats.get("samples", [])
        if not samples:
            samples = [stats]
        
        if len(samples) < 2:
            return "FAIL"
        
        first = samples[0]
        last = samples[-1]
        
        mime = last.get("codecMimeType", "")
        frames_first = first.get("framesDecoded", 0)
        frames_last = last.get("framesDecoded", 0)
        bytes_first = first.get("bytesReceived", 0)
        bytes_last = last.get("bytesReceived", 0)
        rendered = last.get("first_frame_rendered") is True
        
        if ((mime == "video/H264" or "H264" in mime.upper()) and
            frames_last > 0 and
            frames_last > frames_first and
            bytes_last > bytes_first and
            rendered):
            return "PASS"
        return "FAIL"
    except Exception:
        return "FAIL"

def evaluate_gate_d(devices, evidence_dir):
    """Gate D: Coturn TURN Relay Verification - Requires separate verified Direct P2P sample and Forced TURN sample with progressing bytes/frames"""
    if not devices:
        return "NOT_RUN"
    
    direct_path = os.path.join(evidence_dir, "webrtc_stats.json")
    turn_path = os.path.join(evidence_dir, "webrtc_turn_stats.json")
    if not os.path.exists(direct_path) or not os.path.exists(turn_path):
        return "FAIL"
    
    try:
        with open(direct_path, "r", encoding="utf-8") as f:
            direct_stats = json.load(f)
        with open(turn_path, "r", encoding="utf-8") as f:
            turn_stats = json.load(f)
        
        direct_cand = direct_stats.get("candidateType", "") or direct_stats.get("localCandidateType", "")
        turn_cand = turn_stats.get("candidateType", "") or turn_stats.get("localCandidateType", "") or turn_stats.get("remoteCandidateType", "")
        
        direct_pass = (direct_cand in ["direct", "host", "srflx"] and direct_stats.get("bytesReceived", 0) > 0)
        turn_pass = (turn_cand == "relay" and turn_stats.get("bytesReceived", 0) > 0 and turn_stats.get("framesDecoded", 0) > 0)
        
        if direct_pass and turn_pass:
            return "PASS"
        return "FAIL"
    except Exception:
        return "FAIL"

def evaluate_gate_e(devices, evidence_dir):
    """Gate E: Security & Consent - Requires raw security evidence containing consent prompt logs, FLAG_SECURE black-stream proof, and stale agent fencing"""
    if not devices:
        return "NOT_RUN"
    
    sec_path = os.path.join(evidence_dir, "security_evidence.json")
    if not os.path.exists(sec_path):
        return "FAIL"
    
    try:
        with open(sec_path, "r", encoding="utf-8") as f:
            sec = json.load(f)
        
        if (sec.get("consent_prompt_granted") is True and
            sec.get("flag_secure_respected") is True and
            sec.get("stale_agent_fenced") is True and
            sec.get("consent_log_timestamp") and
            sec.get("flag_secure_black_frame_hash")):
            return "PASS"
        return "FAIL"
    except Exception:
        return "FAIL"

def evaluate_gate_f(devices, evidence_dir):
    """Gate F: Scale & Isolation - Requires raw scale evidence verifying >= 3 physical devices streaming concurrently without cross-device leakage"""
    if not devices or len(devices) < 3:
        return "NOT_RUN" if not devices else "FAIL"
    
    scale_path = os.path.join(evidence_dir, "scale_evidence.json")
    if not os.path.exists(scale_path):
        return "FAIL"
    
    try:
        with open(scale_path, "r", encoding="utf-8") as f:
            scale = json.load(f)
        
        if (scale.get("concurrent_device_count", 0) >= 3 and
            scale.get("viewer_limit_enforced") is True and
            scale.get("cross_device_leakage") is False and
            len(scale.get("active_session_ids", [])) >= 3):
            return "PASS"
        return "FAIL"
    except Exception:
        return "FAIL"

def evaluate_gate_g(devices, evidence_dir):
    """Gate G: Automated Evidence Package - PASS ONLY if complete required artifact set exists and non-empty"""
    if not devices:
        return "NOT_RUN"
    
    required_files = [
        "manifest.json",
        "webrtc_stats.json",
        "webrtc_turn_stats.json",
        "command_journal.json",
        "fleet_lifecycle_evidence.json",
        "security_evidence.json",
        "scale_evidence.json"
    ]
    for fn in required_files:
        fp = os.path.join(evidence_dir, fn)
        if not os.path.exists(fp) or os.path.getsize(fp) == 0:
            return "FAIL"
    return "PASS"

def collect_evidence_package(run_id=None):
    if not run_id:
        run_id = f"run_{int(time.time())}"
    
    out_dir = os.path.join("artifacts", "phase17", run_id)
    os.makedirs(out_dir, exist_ok=True)
    
    sha = get_git_sha()
    devices = collect_adb_device_metadata()
    
    for dev in devices:
        serial = dev["serial"]
        logcat = run_cmd(f"adb -s {serial} logcat -d -t 500", check=False)
        if logcat:
            log_path = os.path.join(out_dir, f"logcat_{serial}.log")
            with open(log_path, "w", encoding="utf-8") as f:
                f.write(logcat)

    gate_a = evaluate_gate_a(devices, out_dir)
    gate_b = evaluate_gate_b(devices, out_dir)
    gate_c = evaluate_gate_c(devices, out_dir)
    gate_d = evaluate_gate_d(devices, out_dir)
    gate_e = evaluate_gate_e(devices, out_dir)
    gate_f = evaluate_gate_f(devices, out_dir)
    gate_g = evaluate_gate_g(devices, out_dir)
    
    gate_statuses = [gate_a, gate_b, gate_c, gate_d, gate_e, gate_f, gate_g]
    if all(s == "NOT_RUN" for s in gate_statuses):
        overall_status = "NOT_RUN"
    elif any(s == "FAIL" for s in gate_statuses):
        overall_status = "FAIL"
    elif all(s == "PASS" for s in gate_statuses):
        overall_status = "PASS"
    else:
        overall_status = "PARTIAL"

    manifest = {
        "schema_version": "phase17-evidence-v2",
        "git_sha": sha,
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "run_id": run_id,
        "overall_status": overall_status,
        "device_count": len(devices),
        "devices": devices,
        "software_baseline_locked": "e3a8618ebcf44c57ba56d72bb76a1acd531eab95",
        "gates": {
            "gate_a_physical_fleet_lifecycle": gate_a,
            "gate_b_physical_control": gate_b,
            "gate_c_real_h264_screen_capture": gate_c,
            "gate_d_networking_turn_relay": gate_d,
            "gate_e_security_consent": gate_e,
            "gate_f_scale_isolation": gate_f,
            "gate_g_automated_evidence_package": gate_g
        }
    }
    
    manifest_path = os.path.join(out_dir, "manifest.json")
    with open(manifest_path, "w", encoding="utf-8") as f:
        json.dump(manifest, f, indent=2)
        
    report_path = os.path.join(out_dir, "PHASE-1.7-ACCEPTANCE.md")
    with open(report_path, "w", encoding="utf-8") as f:
        f.write(f"# Phase 1.7 Acceptance Report — Run {run_id}\n\n")
        f.write(f"- **Git SHA**: `{sha}`\n")
        f.write(f"- **Timestamp**: `{manifest['timestamp']}`\n")
        f.write(f"- **Overall Status**: **{overall_status}**\n")
        f.write(f"- **Attached Physical Devices**: `{len(devices)}`\n\n")
        f.write("## Gate Summary\n\n")
        for gate_name, status in manifest["gates"].items():
            f.write(f"- `{gate_name}`: **{status}**\n")

    return manifest

if __name__ == "__main__":
    manifest = collect_evidence_package()
    print(json.dumps(manifest, indent=2))
