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

Streaming ("stream": true): every model call is streamed from the gateway and its content is relayed as
chat.completion.chunk events as it arrives, including text the model writes before it asks for a tool. Tool calls are
assembled from their deltas and run between steps; each one is reported in a chunk with no choices and a
"gryvia": {"toolCall": ...} field. The last chunk before [DONE] has no choices and carries the summed usage and
"gryvia": {"steps", "toolCalls"}. An error after the first event is sent as a data event {"error": ...}.
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
from fastapi.responses import JSONResponse, StreamingResponse

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


def sse_events(lines):
    """The JSON objects of a server-sent event stream's data lines, up to [DONE]."""
    for line in lines:
        if not line.startswith("data:"):
            continue
        data = line[5:].strip()
        if data == "[DONE]":
            return
        try:
            event = json.loads(data)
        except ValueError:
            continue
        if isinstance(event, dict):
            yield event


def _merge_tool_call(calls, delta):
    slot = calls.setdefault(
        delta.get("index", len(calls)), {"id": "", "type": "function", "function": {"name": "", "arguments": ""}}
    )
    if delta.get("id"):
        slot["id"] = delta["id"]
    fn = delta.get("function") or {}
    slot["function"]["name"] += fn.get("name") or ""
    slot["function"]["arguments"] += fn.get("arguments") or ""


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

    def _body(self, messages, offer_tools):
        body = {"model": self.config["model"], "messages": messages}
        if offer_tools:
            body["tools"] = self.schemas
            body["tool_choice"] = "auto"
        return body

    def _model_stream(self, messages, offer_tools):
        """Yields the content deltas of one streamed model call and returns its message: content, assembled
        tool_calls, finish_reason and usage. Raises GatewayError before the first yield when the call fails."""
        body = self._body(messages, offer_tools)
        body["stream"] = True
        body["stream_options"] = {"include_usage": True}
        content, calls, finish, usage = [], {}, None, {}
        try:
            with self.client.stream(
                "POST", self.gateway + "/v1/chat/completions", json=body, headers=self._headers(), timeout=MODEL_TIMEOUT
            ) as resp:
                if resp.status_code != 200:
                    resp.read()
                    raise GatewayError(
                        resp.status_code, f"LLM gateway answered {resp.status_code}: {_error_message(resp)}"
                    )
                if not resp.headers.get("content-type", "").startswith("text/event-stream"):
                    resp.read()
                    data = resp.json()
                    choice = (data.get("choices") or [{}])[0]
                    msg = choice.get("message") or {}
                    if msg.get("content"):
                        yield msg["content"]
                    return {"content": msg.get("content") or "", "tool_calls": msg.get("tool_calls") or [],
                            "finish": choice.get("finish_reason"), "usage": data.get("usage") or {}}
                for event in sse_events(resp.iter_lines()):
                    if event.get("usage"):
                        usage = event["usage"]
                    for choice in event.get("choices") or []:
                        delta = choice.get("delta") or {}
                        if delta.get("content"):
                            content.append(delta["content"])
                            yield delta["content"]
                        for tc in delta.get("tool_calls") or []:
                            _merge_tool_call(calls, tc)
                        if choice.get("finish_reason"):
                            finish = choice["finish_reason"]
        except httpx.HTTPError as e:
            raise GatewayError(502, f"LLM gateway unreachable: {e}") from e
        return {"content": "".join(content), "tool_calls": [calls[k] for k in sorted(calls)], "finish": finish,
                "usage": usage}

    def _model(self, messages, offer_tools):
        body = self._body(messages, offer_tools)
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

    def _start(self, messages):
        msgs = list(messages)
        if self.config.get("systemPrompt"):
            msgs.insert(0, {"role": "system", "content": self.config["systemPrompt"]})
        return msgs

    def _call_tools(self, step, calls, msgs, trace):
        """Runs the tool calls of one step, appends their results to msgs and yields their trace entries."""
        for call in calls:
            fn = call.get("function") or {}
            name = fn.get("name", "")
            try:
                args = json.loads(fn.get("arguments") or "{}")
                result = self.run_tool(name, args)
            except ValueError:
                args, result = None, "error: arguments are not valid JSON"
            entry = {"step": step, "tool": name, "arguments": args, "ok": not result.startswith("error:")}
            trace.append(entry)
            msgs.append({"role": "tool", "tool_call_id": call.get("id", ""), "content": result})
            yield entry

    def chat_stream(self, messages):
        """The streaming form of chat: yields chat.completion.chunk objects."""
        msgs = self._start(messages)
        cid, created, model = "chatcmpl-" + uuid.uuid4().hex[:24], int(time.time()), self.config["name"]
        usage = {"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0}
        trace, started, step = [], False, 0

        def chunk(choices, **extra):
            return {"id": cid, "object": "chat.completion.chunk", "created": created, "model": model,
                    "choices": choices, **extra}

        while True:
            step += 1
            offer = bool(self.schemas) and step < self.config["maxSteps"]
            call = self._model_stream(msgs, offer)
            while True:
                try:
                    text = next(call)
                except StopIteration as done:
                    result = done.value
                    break
                delta = {"content": text}
                if not started:
                    delta["role"], started = "assistant", True
                yield chunk([{"index": 0, "delta": delta, "finish_reason": None}])
            for k in usage:
                usage[k] += int(result["usage"].get(k) or 0)
            calls = result["tool_calls"]
            if not calls or not offer:
                finish = "length" if calls else (result["finish"] or "stop")
                delta = {} if started else {"role": "assistant", "content": ""}
                yield chunk([{"index": 0, "delta": delta, "finish_reason": finish}])
                yield chunk([], usage=usage, gryvia={"steps": step, "toolCalls": trace})
                return
            msgs.append({"role": "assistant", "content": result["content"] or None, "tool_calls": calls})
            for entry in self._call_tools(step, calls, msgs, trace):
                yield chunk([], gryvia={"toolCall": entry})

    def chat(self, messages):
        msgs = self._start(messages)
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
            for _ in self._call_tools(step, calls, msgs, trace):
                pass

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


def gateway_status(e):
    return e.status if e.status in (401, 403, 404, 429, 503) else 502


_END = object()


def _next(gen):
    try:
        return next(gen)
    except StopIteration:
        return _END


def sse_body(first, gen):
    """Server-sent events: first, the rest of gen, an error event if it fails, then [DONE]."""
    try:
        if first is not _END:
            yield f"data: {json.dumps(first)}\n\n"
            for item in gen:
                yield f"data: {json.dumps(item)}\n\n"
    except GatewayError as e:
        yield f"data: {json.dumps({'error': {'message': e.message, 'type': 'gateway_error', 'code': e.status}})}\n\n"
    finally:
        gen.close()
    yield "data: [DONE]\n\n"


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
        messages = body.get("messages")
        if not isinstance(messages, list) or not messages:
            return error(400, "messages must be a non-empty list")
        for m in messages:
            if not isinstance(m, dict) or m.get("role") not in ROLES:
                return error(400, "each message needs a role of system, user, assistant or tool")
        if body.get("stream"):
            gen = agent.chat_stream(messages)
            try:
                first = await anyio.to_thread.run_sync(_next, gen)
            except GatewayError as e:
                return error(gateway_status(e), e.message, "gateway_error")
            return StreamingResponse(
                sse_body(first, gen),
                media_type="text/event-stream",
                headers={"Cache-Control": "no-cache", "X-Accel-Buffering": "no"},
            )
        try:
            return await anyio.to_thread.run_sync(agent.chat, messages)
        except GatewayError as e:
            return error(gateway_status(e), e.message, "gateway_error")

    return app


def main():
    import uvicorn

    conf = load_config(os.environ.get("AGENT_CONFIG", "/etc/agent/config.json"))
    agent = Agent(conf, os.environ["GATEWAY_URL"], os.environ["GRYVIA_LLM_KEY"])
    uvicorn.run(create_app(agent), host="0.0.0.0", port=int(os.environ.get("PORT", "8080")), log_level="info")


if __name__ == "__main__":
    main()
