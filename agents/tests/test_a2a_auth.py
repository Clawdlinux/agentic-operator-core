"""Tests for A2A bearer-token authentication (P0 launch fix)."""

from __future__ import annotations

import os
from unittest.mock import MagicMock

import pytest
from fastapi.testclient import TestClient

from agents.a2a.server import A2AServer


@pytest.fixture(autouse=True)
def _clean_env(monkeypatch):
    monkeypatch.delenv("A2A_AUTH_TOKEN", raising=False)
    monkeypatch.delenv("A2A_AUTH_DISABLED", raising=False)


def _server(token: str | None = "secret-token", **kwargs) -> A2AServer:
    store = MagicMock()
    return A2AServer(
        agent_name="test-agent",
        skills=[{"name": "echo"}],
        store=store,
        auth_token=token,
        **kwargs,
    )


def test_constructor_rejects_missing_token(monkeypatch):
    monkeypatch.delenv("A2A_AUTH_TOKEN", raising=False)
    monkeypatch.delenv("A2A_AUTH_DISABLED", raising=False)
    with pytest.raises(RuntimeError, match="auth_token"):
        _server(token=None)


def test_env_var_token_is_accepted(monkeypatch):
    monkeypatch.setenv("A2A_AUTH_TOKEN", "from-env")
    server = _server(token=None)
    assert server._auth_token == "from-env"


def test_unauthenticated_requests_are_rejected():
    client = TestClient(_server().app)

    # Missing header
    assert client.get("/a2a/agent-card").status_code == 401
    assert client.get("/a2a/tasks").status_code == 401
    assert (
        client.post(
            "/a2a/tasks",
            json={
                "skill": "echo",
                "input_data": {},
                "sender_agent": "x",
            },
        ).status_code
        == 401
    )

    # Wrong scheme
    assert (
        client.get("/a2a/agent-card", headers={"Authorization": "Basic abc"}).status_code
        == 401
    )

    # Wrong token
    assert (
        client.get(
            "/a2a/agent-card", headers={"Authorization": "Bearer wrong"}
        ).status_code
        == 401
    )


def test_authenticated_requests_succeed():
    client = TestClient(_server().app)
    resp = client.get(
        "/a2a/agent-card", headers={"Authorization": "Bearer secret-token"}
    )
    assert resp.status_code == 200
    assert resp.json()["name"] == "test-agent"


def test_healthz_is_public():
    client = TestClient(_server().app)
    assert client.get("/healthz").status_code == 200


def test_disabled_flag_bypasses_auth(monkeypatch):
    monkeypatch.setenv("A2A_AUTH_DISABLED", "1")
    server = _server(token=None)
    client = TestClient(server.app)
    assert client.get("/a2a/agent-card").status_code == 200
