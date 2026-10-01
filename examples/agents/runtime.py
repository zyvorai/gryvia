"""Reference GryviaAgent runtime.

Serves an OpenAI-compatible POST /v1/chat/completions. Each request runs a tool-calling loop against the agent's
model through the Gryvia LLM gateway (with the agent's own key), for at most maxSteps model calls; the last step
offers no tools, so the model has to answer.

Environment (set by the GryviaAgent controller):
  AGENT_CONFIG    path of the JSON config (name, model, systemPrompt, maxSteps, tools)
  GATEWAY_URL     LLM gateway base URL
  GRYVIA_LLM_KEY  the agent's gateway key
  PORT            listen port (8080)

Tools:
  retrieval  POST {GATEWAY_URL}/v1/retrieve on a GryviaVectorIndex
  http       GET or POST a URL the model picks, which must match one of the allowlisted prefixes
"""

import json
import os
import posixpath
import time
import uuid
from urllib.parse import urlsplit

import anyio.to_thread
import httpx
from fastapi import FastAPI, Request
from fastapi.responses import JSONResponse

MAX_TOOL_RESULT = 8000
MAX_HTTP_BODY = 65536
MODEL_TIMEOUT = 120.0
TOOL_TIMEOUT = 10.0
DEFAULT_PORTS = {"http": 80, "https": 443}
ROLES = {"system", "user", "assistant", "tool"}


class GatewayError(Exception):
    def __init__(self, status, message):
        super().__init__(message)
        self.status = status
        self.message = message


def load_config(path):
    with open(path, encoding="utf-8") as f:
        conf = json.load(f)
    conf.setdefault("tools", [])
    conf["maxSteps"] = max(1, int(conf.get("maxSteps") or 5))
    return conf


def _split(url):
    u = urlsplit(url)
    scheme = (u.scheme or "").lower()
    try:
        port = u.port or DEFAULT_PORTS.get(scheme)
    except ValueError:
        return None
    return scheme, (u.hostname or "").lower(), port, u.path or "/", u


def url_allowed(url, prefixes):
    """True when url has a prefix's scheme, host and port and its normalized path starts with the prefix path."""
    got = _split(url)
    if not got:
        return False
    scheme, host, port, path, parts = got
    if scheme not in DEFAULT_PORTS or not host or parts.username or parts.password:
        return False
    norm = posixpath.normpath(path)
    if path.endswith("/") and norm != "/":
        norm += "/"
    for prefix in prefixes:
        want = _split(prefix)
        if not want or want[:3] != (scheme, host, port):
            continue
        base = want[3]
        if base in ("", "/") or norm == base.rstrip("/") or norm.startswith(base if base.endswith("/") else base + "/"):
            return True
    return False


def tool_schemas(tools):
    out = []
    for t in tools:
        if t["type"] == "retrieval":
            desc = (
                t.get("description") or f"Search the {t['index']} knowledge base and return the most relevant passages."
            )
            params = {
                "type": "object",
                "properties": {"query": {"type": "string", "description": "What to search for."}},
                "required": ["query"],
            }
        else:
            desc = t.get("description") or "Call an HTTP endpoint."
            desc += " Allowed URL prefixes: " + ", ".join(t["urls"])
            props = {"url": {"type": "string", "description": "The full URL to call."}}
            if t.get("method", "GET") == "POST":
                props["body"] = {"type": "string", "description": "Request body (JSON text)."}
            params = {"type": "object", "properties": props, "required": ["url"]}
        out.append({"type": "function", "function": {"name": t["name"], "description": desc, "parameters": params}})
    return out


def _cap(text):
    return text if len(text) <= MAX_TOOL_RESULT else text[:MAX_TOOL_RESULT] + "\n[truncated]"


def _error_message(resp):
    try:
        err = resp.json().get("error", {})
        return err.get("message") if isinstance(err, dict) else str(err)
    except ValueError:
        return resp.text[:300]


class Agent:
    def __init__(self, config, gateway_url, key, client=None):
        self.config = config
        self.gateway = gateway_url.rstrip("/")
        self.key = key
        self.client = client or httpx.Client()
        self.tools = {t["name"]: t for t in config["tools"]}
        self.schemas = tool_schemas(config["tools"])

    def _headers(self):
        return {"Authorization": f"Bearer {self.key}"}

    def _model(self, messages, offer_tools):
        body = {"model": self.config["model"], "messages": messages}
        if offer_tools:
            body["tools"] = self.schemas
            body["tool_choice"] = "auto"
        try:
            resp = self.client.post(
                self.gateway + "/v1/chat/completions", json=body, headers=self._headers(), timeout=MODEL_TIMEOUT
            )
        except httpx.HTTPError as e:
            raise GatewayError(502, f"LLM gateway unreachable: {e}") from e
        if resp.status_code != 200:
            raise GatewayError(resp.status_code, f"LLM gateway answered {resp.status_code}: {_error_message(resp)}")
        return resp.json()

    def run_tool(self, name, args):
        tool = self.tools.get(name)
        if tool is None:
            return f"error: unknown tool {name}"
        if not isinstance(args, dict):
            return "error: arguments must be a JSON object"
        if tool["type"] == "retrieval":
            return self._retrieve(tool, args)
        return self._http(tool, args)

    def _retrieve(self, tool, args):
        query = args.get("query")
        if not isinstance(query, str) or not query.strip():
            return "error: query is required"
        body = {"index": tool["index"], "query": query, "topK": tool.get("topK") or 4}
        try:
            resp = self.client.post(
                self.gateway + "/v1/retrieve", json=body, headers=self._headers(), timeout=TOOL_TIMEOUT * 3
            )
        except httpx.HTTPError as e:
            return f"error: retrieval failed: {e}"
        if resp.status_code != 200:
            return f"error: retrieval answered {resp.status_code}: {_error_message(resp)}"
        hits = resp.json().get("data", [])
        if not hits:
            return "No matching passages."
        lines = [f"[{i}] ({h.get('source', '?')}) {h.get('text', '')}" for i, h in enumerate(hits, 1)]
        return _cap("\n\n".join(lines))

    def _http(self, tool, args):
        url = args.get("url")
        if not isinstance(url, str) or not url_allowed(url, tool["urls"]):
            return "error: url is not allowed; it must start with one of: " + ", ".join(tool["urls"])
        method = tool.get("method", "GET")
        kwargs = {"timeout": TOOL_TIMEOUT, "follow_redirects": False}
        if method == "POST":
            kwargs["content"] = str(args.get("body", ""))
            kwargs["headers"] = {"Content-Type": "application/json"}
        try:
            with self.client.stream(method, url, **kwargs) as resp:
                data = b""
                for chunk in resp.iter_bytes():
                    data += chunk
                    if len(data) >= MAX_HTTP_BODY:
                        break
                status = resp.status_code
        except httpx.HTTPError as e:
            return f"error: request failed: {e}"
        return _cap(f"HTTP {status}\n" + data[:MAX_HTTP_BODY].decode("utf-8", "replace"))

    def chat(self, messages):
        msgs = list(messages)
        if self.config.get("systemPrompt"):
            msgs.insert(0, {"role": "system", "content": self.config["systemPrompt"]})
        usage = {"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0}
        trace = []
        max_steps = self.config["maxSteps"]
        step = 0
        while True:
            step += 1
            offer = bool(self.schemas) and step < max_steps
            data = self._model(msgs, offer)
            for k in usage:
                usage[k] += int((data.get("usage") or {}).get(k) or 0)
            choice = (data.get("choices") or [{}])[0]
            msg = choice.get("message") or {}
            calls = msg.get("tool_calls") or []
            if not calls or not offer:
                finish = "length" if calls else choice.get("finish_reason", "stop")
                return self._completion(msg.get("content") or "", finish, usage, step, trace)
            msgs.append({"role": "assistant", "content": msg.get("content"), "tool_calls": calls})
            for call in calls:
                fn = call.get("function") or {}
                name = fn.get("name", "")
                try:
                    args = json.loads(fn.get("arguments") or "{}")
                    result = self.run_tool(name, args)
                except ValueError:
                    args, result = None, "error: arguments are not valid JSON"
                trace.append({"step": step, "tool": name, "arguments": args, "ok": not result.startswith("error:")})
                msgs.append({"role": "tool", "tool_call_id": call.get("id", ""), "content": result})

    def _completion(self, content, finish, usage, steps, trace):
        return {
            "id": "chatcmpl-" + uuid.uuid4().hex[:24],
            "object": "chat.completion",
            "created": int(time.time()),
            "model": self.config["name"],
            "choices": [{"index": 0, "message": {"role": "assistant", "content": content}, "finish_reason": finish}],
            "usage": usage,
            "gryvia": {"steps": steps, "toolCalls": trace},
        }


def error(status, message, kind="invalid_request_error"):
    return JSONResponse({"error": {"message": message, "type": kind}}, status_code=status)


def create_app(agent):
    app = FastAPI(title=f"gryvia agent {agent.config['name']}", docs_url=None, redoc_url=None)

    @app.get("/healthz")
    def healthz():
        return {"status": "ok"}

    @app.get("/v1/models")
    def models():
        return {"object": "list", "data": [{"id": agent.config["name"], "object": "model", "owned_by": "gryvia"}]}

    @app.post("/v1/chat/completions")
    async def completions(request: Request):
        try:
            body = await request.json()
        except ValueError:
            return error(400, "body must be JSON")
        if not isinstance(body, dict):
            return error(400, "body must be a JSON object")
        if body.get("stream"):
            return error(400, "streaming is not supported by this agent")
        messages = body.get("messages")
        if not isinstance(messages, list) or not messages:
            return error(400, "messages must be a non-empty list")
        for m in messages:
            if not isinstance(m, dict) or m.get("role") not in ROLES:
                return error(400, "each message needs a role of system, user, assistant or tool")
        try:
            return await anyio.to_thread.run_sync(agent.chat, messages)
        except GatewayError as e:
            status = e.status if e.status in (401, 403, 404, 429, 503) else 502
            return error(status, e.message, "gateway_error")

    return app


def main():
    import uvicorn

    conf = load_config(os.environ.get("AGENT_CONFIG", "/etc/agent/config.json"))
    agent = Agent(conf, os.environ["GATEWAY_URL"], os.environ["GRYVIA_LLM_KEY"])
    uvicorn.run(create_app(agent), host="0.0.0.0", port=int(os.environ.get("PORT", "8080")), log_level="info")


if __name__ == "__main__":
    main()
