def test_empty_state(make_client):
    c = make_client("ai")
    r = c.get("/api/ai/training/insight")
    assert r.status_code == 200
    assert r.json() == {"rankStats": [], "commPattern": "", "commComputeRatio": 0, "stragglers": [], "bottleneck": ""}
    assert c.get("/api/ai/training/nccl").json() == {"operations": []}
    assert c.get("/api/gpu/memory").json() == {
        "h2dBytes": 0, "d2hBytes": 0, "d2dBytes": 0, "h2dCount": 0, "d2hCount": 0, "d2dCount": 0}


def test_insight_without_status_is_empty(make_client, fake_k8s):
    fake_k8s.add("fabrictraininginsights", {"metadata": {"name": "a"}, "spec": {}}, namespace="default")
    assert make_client("ai").get("/api/ai/training/insight").json()["rankStats"] == []


def test_populated_insight_uses_latest(make_client, fake_k8s):
    fake_k8s.add("fabrictraininginsights", {
        "metadata": {"name": "old"},
        "status": {"bottleneck": "compute", "lastAnalysis": "2026-01-01T00:00:00Z"},
    }, namespace="default")
    fake_k8s.add("fabrictraininginsights", {
        "metadata": {"name": "new"},
        "status": {
            "bottleneck": "communication", "commPattern": "ring_allreduce", "commComputeRatio": 0.42,
            "lastAnalysis": "2026-02-01T00:00:00Z",
            "rankStats": [{"rank": 0, "avgLatencyNs": 100, "totalBytes": 2048, "isStraggler": False},
                          {"rank": 1, "avgLatencyNs": 300, "totalBytes": 2048, "isStraggler": True}],
            "stragglers": [{"rank": 1, "slowdownFactor": 3.0, "reason": "slow link"}],
        },
    }, namespace="ml")
    body = make_client("ai").get("/api/ai/training/insight").json()
    assert body["bottleneck"] == "communication"
    assert body["commPattern"] == "ring_allreduce"
    assert body["commComputeRatio"] == 0.42
    assert body["lastAnalysis"] == "2026-02-01T00:00:00Z"
    assert [r["rank"] for r in body["rankStats"]] == [0, 1]
    assert body["rankStats"][1]["isStraggler"] is True
    assert body["stragglers"] == [{"rank": 1, "slowdownFactor": 3.0, "reason": "slow link"}]
