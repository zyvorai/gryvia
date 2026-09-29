import pytest

NS = "default"


def obj(name, spec=None, status=None):
    o = {"metadata": {"name": name}, "spec": spec or {}}
    if status is not None:
        o["status"] = status
    return o


@pytest.fixture
def client(make_client):
    return make_client("network")


GRAPH = obj("g1", {}, {
    "lastUpdated": "2026-01-02T00:00:00Z",
    "nodes": [{"name": "api", "health": "Healthy"}, {"name": "db", "health": "critical"}],
    "edges": [{"source": "api", "destination": "db", "protocol": "tcp", "port": 5432,
               "latencyP99": "3ms", "throughput": "10MB/s", "verdict": "forwarded"},
              {"source": "api", "destination": "cache", "protocol": "tcp", "port": 6379,
               "verdict": "dropped"}],
})


@pytest.mark.parametrize("path", ["flows", "policies", "anomalies", "traces"])
def test_empty_lists(client, path):
    r = client.get(f"/api/network/{path}")
    assert r.status_code == 200 and r.json() == {"items": []}


def test_empty_insights_graph_costs(client):
    assert client.get("/api/network/insights").json() == {
        "activeFlows": 0, "policiesEnforced": 0, "anomaliesDetected": 0, "traceSessions": 0}
    assert client.get("/api/network/graph").json() == {"nodes": [], "edges": []}
    assert client.get("/api/network/costs").json() == {
        "reports": [], "costPerGB": {"sameZone": 0, "crossZone": 0, "internetEgress": 0}}


def test_flows_and_graph_from_service_graph(client, fake_k8s):
    fake_k8s.add("fabricservicegraphs", GRAPH, NS)
    flows = client.get("/api/network/flows").json()["items"]
    assert len(flows) == 2
    assert flows[0]["spec"] == {"timestamp": "2026-01-02T00:00:00Z", "source": "api", "destination": "db",
                                "protocol": "TCP", "port": 5432, "bytes": "10MB/s", "latency": "3ms",
                                "verdict": "FORWARDED"}
    assert flows[1]["spec"]["verdict"] == "DROP"
    g = client.get("/api/network/graph").json()
    by_id = {n["id"]: n for n in g["nodes"]}
    assert set(by_id) == {"api", "db", "cache"}
    assert by_id["api"]["flowCount"] == 2 and by_id["api"]["health"] == "healthy"
    assert by_id["cache"]["health"] == "unknown"
    assert g["edges"][0] == {"source": "api", "target": "db", "protocol": "tcp", "latency": "3ms",
                             "verdict": "forwarded"}


def test_policies_list_and_shape(client, fake_k8s):
    fake_k8s.add("fabricflowpolicies", obj("p1", {
        "source": {"service": "a"}, "destination": {"service": "b", "port": 80},
        "protocol": "tcp", "action": "allow", "intent": "secure"},
        {"phase": "Enforced", "enforced": True, "matchedFlows": 7}), NS)
    p = client.get("/api/network/policies").json()["items"][0]
    assert p["spec"] == {"sourceService": "a", "destinationService": "b", "port": 80, "protocol": "tcp",
                         "action": "allow", "intent": "secure"}
    assert p["status"] == {"phase": "Enforced", "matchedFlows": 7}


def test_create_policy(client, fake_k8s):
    body = {"sourceService": "web", "destinationService": "api", "port": 8080, "protocol": "TCP",
            "action": "deny", "intent": "secure"}
    r = client.post("/api/network/policies", json=body)
    assert r.status_code == 201
    j = r.json()
    assert j["spec"]["sourceService"] == "web" and j["spec"]["protocol"] == "tcp" and j["spec"]["port"] == 8080
    stored = fake_k8s.store[("fabricflowpolicies", NS, j["metadata"]["name"])]
    assert stored["spec"]["destination"] == {"service": "api", "port": 8080}
    assert client.post("/api/network/policies", json=body).status_code == 409


@pytest.mark.parametrize("patch", [
    {"sourceService": "Bad_Name"}, {"destinationService": ""}, {"port": 0}, {"port": 70000},
    {"protocol": "sctp"}, {"action": "maybe"}, {"intent": ""}, {"intent": "<script>"},
    {"name": "-x"}, {"sourceService": "a" * 64},
])
def test_create_policy_validation(client, patch):
    body = {"sourceService": "web", "destinationService": "api", "port": 80, "protocol": "tcp",
            "action": "allow", "intent": "secure", **patch}
    assert client.post("/api/network/policies", json=body).status_code == 422


def test_apply_policy(client, fake_k8s):
    fake_k8s.add("fabricflowpolicies", obj("p1", {"source": {"service": "a"}}), NS)
    r = client.post("/api/network/policies/p1/apply")
    assert r.status_code == 200
    ann = fake_k8s.store[("fabricflowpolicies", NS, "p1")]["metadata"]["annotations"]
    assert "gryvia.io/apply-requested-at" in ann


def test_apply_policy_404(client):
    assert client.post("/api/network/policies/missing/apply").status_code == 404
    assert client.post("/api/network/policies/Bad_Name/apply").status_code == 404


def test_anomalies_and_insights(client, fake_k8s):
    fake_k8s.add("fabricnetworkanomalies", obj("an1", {"targetService": "api"}, {"anomalies": [
        {"type": "latency", "severity": "High", "detected": "2026-01-02T00:00:00Z", "description": "slow"}]}), NS)
    fake_k8s.add("fabrictrafficinsights", obj("ti1", {"service": "db"}, {"anomalies": [
        {"type": "drops", "severity": "low", "detected": "2026-01-03T00:00:00Z"}]}), NS)
    fake_k8s.add("fabricservicegraphs", GRAPH, NS)
    fake_k8s.add("fabricflowpolicies", obj("p1", {}, {"enforced": True}), NS)
    fake_k8s.add("fabricflowpolicies", obj("p2", {}, {"phase": "Pending"}), NS)
    fake_k8s.add("fabrictracesessions", obj("t1", {"service": "api"}), NS)
    items = client.get("/api/network/anomalies").json()["items"]
    assert [i["spec"]["service"] for i in items] == ["db", "api"]
    assert items[1]["spec"]["severity"] == "high" and items[1]["metadata"]["name"] == "an1-0"
    assert client.get("/api/network/insights").json() == {
        "activeFlows": 2, "policiesEnforced": 1, "anomaliesDetected": 2, "traceSessions": 1}


def test_costs(client, fake_k8s):
    fake_k8s.add("fabricnetworkcosts", obj("c1", {"costPerGB": {"sameZone": 0, "crossZone": 0.01,
                                                                 "internetEgress": 0.09}}, {"reports": [
        {"period": "2026-01-01", "namespace": "ml", "team": "t", "sameZoneBytes": 1, "crossZoneBytes": 2,
         "externalBytes": 3, "totalCostUSD": 1.5}]}), NS)
    j = client.get("/api/network/costs").json()
    assert j["costPerGB"] == {"sameZone": 0, "crossZone": 0.01, "internetEgress": 0.09}
    assert j["reports"][0]["totalCostUSD"] == 1.5 and j["reports"][0]["team"] == "t"


def test_traces_create_get_list(client, fake_k8s):
    body = {"targetService": "api", "duration": "5m", "captureLevel": "L7", "namespace": "ml"}
    r = client.post("/api/network/traces", json=body)
    assert r.status_code == 201
    name = r.json()["metadata"]["name"]
    assert r.json()["spec"] == {"targetService": "api", "duration": "5m", "captureLevel": "l7",
                                "namespace": "ml"}
    assert fake_k8s.store[("fabrictracesessions", NS, name)]["spec"]["service"] == "api"
    fake_k8s.store[("fabrictracesessions", NS, name)]["status"] = {"phase": "active", "flowsCaptured": 4}
    got = client.get(f"/api/network/traces/{name}").json()
    assert got["status"] == {"phase": "Active", "flowsCaptured": 4}
    assert len(client.get("/api/network/traces").json()["items"]) == 1


@pytest.mark.parametrize("patch", [
    {"duration": "forever"}, {"duration": "0m"}, {"captureLevel": "l9"}, {"targetService": "A B"},
    {"namespace": ""},
])
def test_trace_validation(client, patch):
    body = {"targetService": "api", "duration": "5m", "captureLevel": "l4", "namespace": "ml", **patch}
    assert client.post("/api/network/traces", json=body).status_code == 422


def test_trace_404(client):
    assert client.get("/api/network/traces/nope").status_code == 404
    assert client.get("/api/network/traces/Bad_Name").status_code == 404
