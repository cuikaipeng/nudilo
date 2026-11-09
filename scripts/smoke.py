#!/usr/bin/env python3
"""Run a native desktop smoke test with isolated data/assistant homes and no real IM credentials."""
import base64
from datetime import datetime, timezone
import json
import os
from pathlib import Path
import socket
import subprocess
import sys
import tempfile
import time

binary = Path(sys.argv[1]).resolve()
root = Path(tempfile.mkdtemp(prefix="nudilo-smoke-"))
(root / "data").mkdir()
(root / "codex").mkdir()
(root / "claude").mkdir()
full = os.environ.get("NUDILO_NATIVE_CODEX") == "1"
rollout = root / "codex" / "sessions" / "rollout-existing.jsonl"
if full:
    rollout.parent.mkdir()
    rollout.write_text(json.dumps({"timestamp":"2000-01-01T00:00:00Z", "type":"session_meta", "payload":{"id":"existing", "cwd":"/fixture/existing", "source":"vscode"}}) + "\n" + json.dumps({"timestamp":"2000-01-01T00:00:00Z", "type":"event_msg", "payload":{"type":"task_complete", "turn_id":"historical"}}) + "\n")
(root / "data" / "settings.json").write_text(json.dumps({"language":"en", "theme":"light", "auto_connect":full, "claude_auto_connect":False, "automatic_updates":False}))
log = (root / "app.log").open("w")
process = subprocess.Popen([str(binary)], env={**os.environ,
    "CALLBELL_DATA_DIR": str(root / "data"), "CODEX_HOME": str(root / "codex"),
    "CLAUDE_CONFIG_DIR": str(root / "claude"),
    "MYGO_AUTOMATION": str(root / "automation.sock")}, stdout=log, stderr=log)
counter = 0

try:
    deadline = time.monotonic() + 20
    while not (root / "automation.sock").exists():
        if process.poll() is not None or time.monotonic() > deadline:
            raise RuntimeError("App did not start: " + (root / "app.log").read_text())
        time.sleep(0.1)
    sock = socket.socket(socket.AF_UNIX)
    sock.settimeout(20)
    sock.connect(str(root / "automation.sock"))
    stream = sock.makefile("rwb")

    def call(method, params=None):
        global counter
        counter += 1
        stream.write((json.dumps({"id": counter, "method": method, "params": params or {}}) + "\n").encode())
        stream.flush()
        result = json.loads(stream.readline())
        if "error" in result:
            raise RuntimeError(result["error"])
        return result["result"]

    def tree():
        call("idle", {"timeout": 5000})
        return call("tree")["nodes"]

    def press(label=None, key=None):
        until = time.monotonic() + 8
        while time.monotonic() < until:
            nodes = tree()
            matches = [n for n in nodes if (n.get("key") == key if key else n.get("label") == label) and "press" in n.get("actions", [])]
            if matches and "disabled" not in matches[0].get("states", []):
                call("act", {"id": matches[0]["id"], "action": "press"})
                return
            time.sleep(0.1)
        raise AssertionError("Control not ready: " + str(label or key) + " " + str([(n.get("label"), n.get("key"), n.get("states")) for n in nodes if n.get("actions")]))

    def wait_for(text):
        until = time.monotonic() + 15
        while time.monotonic() < until:
            try:
                if any(text in n.get("label", "") or text in str(n.get("value", "")) for n in tree()):
                    return
            except RuntimeError as error:
                if "no_window" not in str(error): raise
            time.sleep(0.2)
        raise AssertionError("Missing UI text: " + text)

    wait_for("Listening for Codex" if full else "Get ready in three steps")
    assert call("windows")["windows"][0]["kind"] == "content", "UI is not native"
    captures = Path("build/screenshots")
    captures.mkdir(parents=True, exist_ok=True)

    def capture(name):
        tree()
        (captures / (name + ".png")).write_bytes(base64.b64decode(call("capture")["png"]))

    capture("overview")
    press("Channels")
    for channel in ("dingtalk", "lark", "telegram", "slack", "feishu"):
        press(key="channel-" + channel)
        press("Personal DM")
        field = {"feishu":"App Secret", "lark":"App Secret", "dingtalk":"Client Secret (AppSecret)", "slack":"Bot User OAuth Token", "telegram":"Bot Token"}[channel]
        wait_for(field)
        assert not any(n.get("label") == "Webhook URL" for n in tree()), "Private mode still requests a group Webhook"
        assert any(n.get("role") == "textField" and "password" in n.get("states", []) for n in tree())
        wait_for("Pair account")
        assert not any(n.get("label") in ("Robot Code", "Member ID", "Private Chat ID", "Recipient email", "Manual setup", "Show credentials", "Load saved credentials") for n in tree()), "Removed configuration controls returned"
        capture(channel + "-private")
        press("Group / channel")
    press(key="save-test")
    wait_for("HTTPS")
    press(key="channel-telegram")
    press("Personal DM")
    press("Pair account")
    wait_for("Enter the complete Bot Token from BotFather.")
    capture("telegram")
    press(key="channel-feishu")
    capture("feishu")

    press("Settings")
    press("Jump to")
    press("Appearance")
    press("Dark")
    wait_for("Dark")
    until = time.monotonic() + 5
    while json.loads((root / "data" / "settings.json").read_text()).get("theme") != "dark":
        if time.monotonic() > until: raise AssertionError("Dark theme not persisted")
        time.sleep(0.1)
    capture("settings-dark-en")
    press(key="language")
    press("简体中文")
    wait_for("外观")
    capture("settings-dark-zh")
    call("menu", {"path": ["文件", "打开 Nudilo"]})
    press("浅色")
    until = time.monotonic() + 5
    while json.loads((root / "data" / "settings.json").read_text()).get("theme") != "light":
        if time.monotonic() > until: raise AssertionError("Light theme not persisted")
        time.sleep(0.1)
    press(key="language")
    press("English")
    wait_for("Appearance")
    capture("settings-light-en")
    if full:
        helper = root / "data" / "bin" / ("callbell-hook.exe" if os.name == "nt" else "callbell-hook")
        fixtures = [
            {"hook_event_name": "Stop"},
            {"hook_event_name": "PreToolUse", "tool_name": "functions.request_user_input_async", "tool_input": {"questions": [{"title": "private fixture"}]}},
            {"hook_event_name": "PermissionRequest", "tool_name": "exec_command"},
        ]
        for fixture in fixtures + fixtures[:1]:
            payload = {"session_id": "smoke", "turn_id": "turn1", "cwd": "/fixture/sample-project", **fixture}
            result = subprocess.run([str(helper), "--codex-event", "--data-dir", str(root / "data")],
                input=json.dumps(payload), text=True, capture_output=True, timeout=3, check=True)
            assert result.stdout.strip() == "{}", "Helper interfered with Codex"
        press("Activity")
        wait_for("3 notifications")
        capture("history")
        records = json.loads((root / "data" / "history.json").read_text())
        assert len(records) == 3 and all(not r["deliveries"] for r in records), "Unconfigured channel sent a message"
        assert all("summary" not in r["event"] for r in records), "Private summary stored"
        with rollout.open("a") as file:
            file.write(json.dumps({"timestamp":datetime.now(timezone.utc).isoformat(), "type":"event_msg", "payload":{"type":"task_complete", "turn_id":"existing-new-turn", "last_agent_message":"private fixture"}}) + "\n")
        wait_for("4 notifications")
        records = json.loads((root / "data" / "history.json").read_text())
        assert any(r["event"].get("turn_id") == "existing-new-turn" for r in records), "Existing chat not monitored"
        assert not any(r["event"].get("turn_id") == "historical" or "summary" in r["event"] for r in records), "History replayed or summary leaked"
        press("Overview")
        wait_for("Last event:")
        press(key="pause")
        wait_for("Paused")
        press(key="pause")
        wait_for("Pause")
        press("Settings")
        press("Disconnect")
        wait_for("Nudilo disconnected.")
        assert "callbell-hook" not in (root / "codex" / "hooks.json").read_text()
        press("Reconnect")
        wait_for("Listening for Codex")
        capture("settings")
    call("menu", {"path": ["File", "Open Nudilo"]})
    call("menu", {"path": ["File", "Close window"]})
    until = time.monotonic() + 3
    while call("windows")["windows"][0]["visible"] and time.monotonic() < until:
        time.sleep(0.1)
    assert not call("windows")["windows"][0]["visible"], "Window did not hide: " + str(call("windows"))
    assert process.poll() is None, "Closing the window stopped the worker"
    if full:
        payload = {"hook_event_name": "Stop", "session_id": "smoke", "turn_id": "background", "cwd": "/fixture/background"}
        subprocess.run([str(helper), "--codex-event", "--data-dir", str(root / "data")], input=json.dumps(payload), text=True, capture_output=True, timeout=3, check=True)
        until = time.monotonic() + 8
        while time.monotonic() < until:
            if len(json.loads((root / "data" / "history.json").read_text())) == 5:
                break
            time.sleep(0.2)
        else:
            raise AssertionError("Background worker did not ingest the event")
    call("menu", {"path": ["File", "Open Nudilo"]})
    assert call("windows")["windows"][0]["visible"], "Window did not reopen"
    press("Settings")
    press("Jump to")
    press("Appearance")
    press("Dark")
    press(key="language")
    press("简体中文")
    wait_for("外观")
    until = time.monotonic() + 5
    while True:
        saved = json.loads((root / "data" / "settings.json").read_text())
        if saved.get("language") == "zh-Hans" and saved.get("theme") == "dark": break
        if time.monotonic() > until: raise AssertionError("Appearance preferences not saved")
        time.sleep(0.1)
    stream.close(); sock.close()
    process.terminate(); process.wait(timeout=8)
    process = subprocess.Popen([str(binary)], env={**os.environ,
        "CALLBELL_DATA_DIR": str(root / "data"), "CODEX_HOME": str(root / "codex"),
    "CLAUDE_CONFIG_DIR": str(root / "claude"),
        "MYGO_AUTOMATION": str(root / "automation.sock")}, stdout=log, stderr=log)
    deadline = time.monotonic() + 20
    while True:
        sock = socket.socket(socket.AF_UNIX); sock.settimeout(20)
        try:
            sock.connect(str(root / "automation.sock")); break
        except (FileNotFoundError, ConnectionRefusedError):
            sock.close()
            if process.poll() is not None or time.monotonic() > deadline: raise RuntimeError("Restart failed")
            time.sleep(0.1)
    stream = sock.makefile("rwb")
    wait_for("概览")
    press("设置")
    wait_for("简体中文")
    capture("restart-dark-zh")
    print("Native smoke passed: five personal/group channel modes, validation, English/Chinese menus, live light/dark switching, preference persistence after restart and close-to-tray." + (" Codex hooks and background delivery checks also passed." if full else ""))
    print("Evidence:", captures.resolve())
    print("Isolated data:", root)
finally:
    process.terminate()
    try:
        process.wait(timeout=8)
    except subprocess.TimeoutExpired:
        process.kill()
        process.wait()
    log.close()
