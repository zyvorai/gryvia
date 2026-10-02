"""Tests of the reference agent runtime (pytest examples/agents)."""

import json

import httpx
import pytest
from fastapi.testclient import TestClient

import runtime

GATEWAY = "http://gw:8080"


def config(**over):
    conf = {
        "name": "helper",
        "namespace": "tenant-a",
        "model": "chat",
        "systemPrompt": "Be brief.",
        "maxSteps": 3,
        "tools": [
            {"name": "search", "type": "retrieval", "index": "handbook", "topK": 2},
            {"name": "status", "type": "http", "urls": ["https://status.example.com/api/"], "method": "GET"},
        ],
    }
    conf.update(over)
    return conf


def tool_call(name, args, call_id="call-1"):
    return {"id": call_id, "type": "function", "function": {"name": name, "arguments": json.dumps(args)}}


def completion(content=None, calls=None, prompt=10, completion_tokens=5):
    msg = {"role": "assistant", "content": content}
    if calls:
        msg["tool_calls"] = calls
    return {
        "choices": [{"index": 0, "message": msg, "finish_reason": "tool_calls" if calls else "stop"}],
        "usage": {
            "prompt_tokens": prompt,
            "completion_tokens": completion_tokens,
            "total_tokens": prompt + completion_tokens,
        },
    }


class Fake:
    """A scripted gateway and tool host behind httpx.MockTransport."""

    def __init__(self, replies, retrieve=None, http=None):
        self.replies = list(replies)
        self.retrieve = retrieve or {"data": [{"text": "Quotas reset monthly.", "source": "kb.md", "score": 0.9}]}
        self.http = http or (200, "all green")
        self.model_bodies = []
        self.requests = []

    def __call__(self, request):
        self.requests.append(request)
        url = str(request.url)
        if url == GATEWAY + "/v1/chat/completions":
            assert request.headers["authorization"] == "Bearer gk-test"
            self.model_bodies.append(json.loads(request.content))
            reply = self.replies.pop(0)
            if isinstance(reply, httpx.Response):
                return reply
            return httpx.Response(200, json=reply)
        if url == GATEWAY + "/v1/retrieve":
            self.retrieve_body = json.loads(request.content)
            if isinstance(self.retrieve, httpx.Response):
                return self.retrieve
            return httpx.Response(200, json=self.retrieve)
        status, text = self.http
        return httpx.Response(status, text=text)


def make(fake, **over):
    agent = runtime.Agent(config(**over), GATEWAY, "gk-test", httpx.Client(transport=httpx.MockTransport(fake)))
    return TestClient(runtime.create_app(agent))


def chat(client, text="How do quotas work?"):
    return client.post("/v1/chat/completions", json={"messages": [{"role": "user", "content": text}]})


def test_answer_without_tools():
    fake = Fake([completion("Hi.")])
    resp = chat(make(fake))
    assert resp.status_code == 200
    body = resp.json()
    assert body["object"] == "chat.completion" and body["model"] == "helper"
    assert body["choices"][0]["message"]["content"] == "Hi."
    assert body["gryvia"] == {"steps": 1, "toolCalls": []}
    sent = fake.model_bodies[0]
    assert sent["model"] == "chat" and sent["messages"][0] == {"role": "system", "content": "Be brief."}
    assert [t["function"]["name"] for t in sent["tools"]] == ["search", "status"]


def test_retrieval_loop_sums_usage():
    fake = Fake([completion(calls=[tool_call("search", {"query": "quotas"})]), completion("Monthly.")])
    body = chat(make(fake)).json()
    assert body["choices"][0]["message"]["content"] == "Monthly."
    assert body["usage"] == {"prompt_tokens": 20, "completion_tokens": 10, "total_tokens": 30}
    assert body["gryvia"]["toolCalls"] == [{"step": 1, "tool": "search", "arguments": {"query": "quotas"}, "ok": True}]
    assert fake.retrieve_body == {"index": "handbook", "query": "quotas", "topK": 2}
    second = fake.model_bodies[1]["messages"]
    assert second[-2]["tool_calls"][0]["id"] == "call-1"
    assert second[-1] == {"role": "tool", "tool_call_id": "call-1", "content": "[1] (kb.md) Quotas reset monthly."}


def test_last_step_offers_no_tools():
    calls = [tool_call("search", {"query": "q"})]
    fake = Fake([completion(calls=calls), completion(calls=calls), completion(calls=calls)])
    body = chat(make(fake)).json()
    assert len(fake.model_bodies) == 3
    assert "tools" in fake.model_bodies[1] and "tools" not in fake.model_bodies[2]
    assert body["choices"][0]["finish_reason"] == "length" and body["gryvia"]["steps"] == 3


def test_http_tool_allowed_and_refused():
    calls = [
        tool_call("status", {"url": "https://status.example.com/api/health"}, "a"),
        tool_call("status", {"url": "https://status.example.com.evil.io/api/x"}, "b"),
        tool_call("status", {"url": "https://status.example.com/api/../admin"}, "c"),
    ]
    fake = Fake([completion(calls=calls), completion("ok")])
    body = chat(make(fake)).json()
    tools = [m for m in fake.model_bodies[1]["messages"] if m["role"] == "tool"]
    assert tools[0]["content"] == "HTTP 200\nall green"
    assert tools[1]["content"].startswith("error: url is not allowed")
    assert tools[2]["content"].startswith("error: url is not allowed")
    hosts = [r.url.host for r in fake.requests if r.url.host != "gw"]
    assert hosts == ["status.example.com"]
    assert [c["ok"] for c in body["gryvia"]["toolCalls"]] == [True, False, False]


def test_tool_errors_go_back_to_the_model():
    calls = [
        tool_call("nope", {}, "a"),
        {"id": "b", "type": "function", "function": {"name": "search", "arguments": "{not json"}},
        tool_call("search", {"query": ""}, "c"),
    ]
    fake = Fake([completion(calls=calls), completion("sorry")])
    chat(make(fake))
    tools = [m["content"] for m in fake.model_bodies[1]["messages"] if m["role"] == "tool"]
    assert tools == ["error: unknown tool nope", "error: arguments are not valid JSON", "error: query is required"]


def test_retrieval_error_is_reported():
    fake = Fake(
        [completion(calls=[tool_call("search", {"query": "q"})]), completion("x")],
        retrieve=httpx.Response(404, json={"error": {"message": "index not found"}}),
    )
    chat(make(fake))
    tool = fake.model_bodies[1]["messages"][-1]["content"]
    assert tool == "error: retrieval answered 404: index not found"


@pytest.mark.parametrize("status,want", [(429, 429), (401, 401), (500, 502)])
def test_gateway_errors(status, want):
    fake = Fake([httpx.Response(status, json={"error": {"message": "nope"}})])
    resp = chat(make(fake))
    assert resp.status_code == want
    assert resp.json()["error"]["type"] == "gateway_error"


@pytest.mark.parametrize(
    "body,msg",
    [
        ({"messages": []}, "non-empty"),
        ({"messages": [{"role": "robot", "content": "x"}]}, "role"),
        ({"messages": [{"role": "user", "content": "x"}], "stream": True}, "streaming"),
    ],
)
def test_bad_requests(body, msg):
    resp = make(Fake([])).post("/v1/chat/completions", json=body)
    assert resp.status_code == 400 and msg in resp.json()["error"]["message"]


def test_health_and_models():
    client = make(Fake([]))
    assert client.get("/healthz").json() == {"status": "ok"}
    assert client.get("/v1/models").json()["data"][0]["id"] == "helper"


def test_no_tools_configured():
    fake = Fake([completion("plain")])
    chat(make(fake, tools=[], systemPrompt=""))
    sent = fake.model_bodies[0]
    assert "tools" not in sent and sent["messages"][0]["role"] == "user"


@pytest.mark.parametrize(
    "url,ok",
    [
        ("https://status.example.com/api/", True),
        ("https://status.example.com/api", True),
        ("https://STATUS.example.com:443/api/x?y=1", True),
        ("http://status.example.com/api/x", False),
        ("https://status.example.com:8443/api/x", False),
        ("https://status.example.com/apix", False),
        ("https://user:pw@status.example.com/api/x", False),
        ("ftp://status.example.com/api/x", False),
        ("https://status.example.com:bad/api/", False),
    ],
)
def test_url_allowed(url, ok):
    assert runtime.url_allowed(url, ["https://status.example.com/api/"]) is ok


def test_url_allowed_host_only_prefix():
    assert runtime.url_allowed("http://inv.shop.svc:9000/items/3", ["http://inv.shop.svc:9000"])
    assert not runtime.url_allowed("http://inv.shop.svc:9001/items/3", ["http://inv.shop.svc:9000"])


def test_post_tool_schema_has_body(tmp_path):
    conf = config(tools=[{"name": "hook", "type": "http", "urls": ["http://x/"], "method": "POST"}], maxSteps=0)
    path = tmp_path / "c.json"
    path.write_text(json.dumps(conf))
    loaded = runtime.load_config(str(path))
    assert loaded["maxSteps"] == 5
    params = runtime.tool_schemas(loaded["tools"])[0]["function"]["parameters"]
    assert set(params["properties"]) == {"url", "body"}
