"""GoMiro API / WS smoke. Cost ¥0. Run inside compose qa container."""
from __future__ import annotations

import json
import os
import time
import urllib.error
import urllib.request
from urllib.parse import quote

import pytest

BASE = os.environ.get("API_URL", "http://api-1:8080").rstrip("/")


def _req(method: str, path: str, body=None, timeout=8):
    data = None
    headers = {}
    if body is not None:
        data = json.dumps(body).encode("utf-8")
        headers["Content-Type"] = "application/json"
    req = urllib.request.Request(BASE + path, data=data, method=method, headers=headers)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            raw = resp.read()
            return resp.status, json.loads(raw) if raw else {}
    except urllib.error.HTTPError as e:
        raw = e.read()
        try:
            payload = json.loads(raw) if raw else {}
        except json.JSONDecodeError:
            payload = {"raw": raw.decode("utf-8", "replace")}
        return e.code, payload


def test_healthz():
    code, body = _req("GET", "/healthz")
    assert code == 200
    assert body.get("status") == "ok"
    assert "time" in body


def test_readyz_real_deps():
    code, body = _req("GET", "/readyz")
    assert code == 200
    assert body.get("db") is True
    assert body.get("redis") is True


def test_metrics_text():
    req = urllib.request.Request(BASE + "/metrics")
    with urllib.request.urlopen(req, timeout=8) as resp:
        text = resp.read().decode()
    assert "gomiro_connections" in text


def test_board_crud_and_unlock():
    stamp = str(int(time.time() * 1000))
    title = "烟测白板-" + stamp
    code, created = _req("POST", "/api/v1/boards", {"title": title, "passcode": "ink"})
    assert code == 201, created
    bid = created["id"]
    code, listed = _req("GET", "/api/v1/boards")
    assert code == 200
    assert any(it["id"] == bid for it in listed.get("items", []))
    code, got = _req("GET", f"/api/v1/boards/{quote(bid)}")
    assert code == 200 and got["hasPass"] is True
    code, _ = _req("POST", f"/api/v1/boards/{quote(bid)}/unlock", {"passcode": "wrong"})
    assert code == 403
    code, unlocked = _req("POST", f"/api/v1/boards/{quote(bid)}/unlock", {"passcode": "ink"})
    assert code == 200 and unlocked.get("ok") is True
    code, patched = _req("PATCH", f"/api/v1/boards/{quote(bid)}", {"title": title + "-改"})
    assert code == 200 and patched["title"].endswith("-改")
    code, _ = _req("DELETE", f"/api/v1/boards/{quote(bid)}")
    assert code == 204
    code, _ = _req("GET", f"/api/v1/boards/{quote(bid)}")
    assert code == 404


def test_unknown_board():
    code, body = _req("GET", "/api/v1/boards/nope_xx")
    assert code == 404
    assert body.get("code") == "not_found"


@pytest.mark.skipif(
    os.environ.get("SKIP_WS") == "1",
    reason="optional websocket extra",
)
def test_ws_join_and_incremental_op():
    websocket = pytest.importorskip("websocket")
    code, created = _req("POST", "/api/v1/boards", {"title": "ws-" + str(int(time.time() * 1000))})
    assert code == 201
    bid = created["id"]
    url = BASE.replace("http://", "ws://").replace("https://", "wss://") + "/ws"

    def client(nick: str):
        ws = websocket.create_connection(url, timeout=8)
        ws.send(
            json.dumps(
                {
                    "v": 1,
                    "type": "join",
                    "payload": {
                        "boardId": bid,
                        "nickname": nick,
                        "color": "#c45c26",
                        "lastSeq": 0,
                        "protoVersion": 1,
                    },
                }
            )
        )
        raw = ws.recv()
        env = json.loads(raw)
        assert env["type"] == "joined", env
        return ws, env

    a, ja = client("Ada")
    b, jb = client("Bea")
    assert ja["payload"]["selfId"] != jb["payload"]["selfId"]
    shape = {
        "id": "shp_aabbccddee",
        "kind": "rect",
        "x": 12,
        "y": 20,
        "w": 80,
        "h": 40,
        "rotation": 0,
        "stroke": "#1c1915",
        "fill": "#f3eadc",
        "strokeW": 2,
        "dash": "solid",
        "opacity": 1,
        "z": 1,
    }
    a.send(
        json.dumps(
            {
                "v": 1,
                "type": "op",
                "payload": {
                    "clientOpId": "op_aabbccddee",
                    "lamport": 1,
                    "baseVersion": 0,
                    "opKind": "shape.create",
                    "targetId": shape["id"],
                    "patch": {"shape": shape},
                },
            }
        )
    )
    seen_bcast = False
    deadline = time.time() + 5
    while time.time() < deadline:
        raw = b.recv()
        if isinstance(raw, bytes):
            continue
        env = json.loads(raw)
        if env.get("type") == "op_bcast":
            assert env["payload"]["opKind"] == "shape.create"
            seen_bcast = True
            break
    a.close()
    b.close()
    assert seen_bcast


def test_malformed_json_does_not_500():
    req = urllib.request.Request(
        BASE + "/api/v1/boards", data=b"{", method="POST", headers={"Content-Type": "application/json"}
    )
    try:
        urllib.request.urlopen(req, timeout=8)
        raise AssertionError("expected 400")
    except urllib.error.HTTPError as e:
        assert e.code == 400
