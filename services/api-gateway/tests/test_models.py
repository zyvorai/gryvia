import pytest

NS = "default"


def seed(fake_k8s, name="bert", stage="dev", **spec):
    fake_k8s.add("gryviamodelregistries", {
        "metadata": {"name": name},
        "spec": {"modelName": "bert", "version": "v1", "stage": stage,
                 "source": {"jobRef": "train-1"},
                 "artifacts": {"s3Path": "s3://b/m", "pvcName": "pvc1", "subPath": "out"}, **spec},
        "status": {"servingEndpoint": "http://svc"},
    }, namespace=NS)


def test_list_empty(make_client):
    r = make_client("models").get("/api/models")
    assert r.status_code == 200 and r.json() == {"items": []}


def test_list_and_get(make_client, fake_k8s):
    seed(fake_k8s)
    c = make_client("models")
    item = c.get("/api/models").json()["items"][0]
    assert item["spec"] == {"version": "v1", "stage": "dev", "sourceJob": "train-1",
                            "artifacts": ["s3://b/m", "pvc://pvc1/out"]}
    assert item["status"] == {"servingEndpoint": "http://svc"}
    assert c.get("/api/models/bert").json()["metadata"]["name"] == "bert"


def test_no_source_omits_field(make_client, fake_k8s):
    fake_k8s.add("gryviamodelregistries", {"metadata": {"name": "m"}, "spec": {"version": "v2"}}, namespace=NS)
    spec = make_client("models").get("/api/models/m").json()["spec"]
    assert "sourceJob" not in spec and spec["artifacts"] == []


def test_get_404(make_client):
    assert make_client("models").get("/api/models/nope").status_code == 404


@pytest.mark.parametrize("cur,target", [("dev", "staging"), ("staging", "production"),
                                        ("production", "archived")])
def test_promote(make_client, fake_k8s, cur, target):
    seed(fake_k8s, stage=cur)
    r = make_client("models").post("/api/models/bert/promote", json={"targetStage": target})
    assert r.status_code == 200 and r.json()["spec"]["stage"] == target
    assert fake_k8s.store[("gryviamodelregistries", NS, "bert")]["spec"]["stage"] == target


@pytest.mark.parametrize("cur,target", [("dev", "production"), ("staging", "dev"), ("archived", "dev"),
                                        ("dev", "dev")])
def test_promote_invalid_transition(make_client, fake_k8s, cur, target):
    seed(fake_k8s, stage=cur)
    r = make_client("models").post("/api/models/bert/promote", json={"targetStage": target})
    assert r.status_code == 409
    assert fake_k8s.store[("gryviamodelregistries", NS, "bert")]["spec"]["stage"] == cur


def test_promote_validation(make_client, fake_k8s):
    seed(fake_k8s)
    c = make_client("models")
    assert c.post("/api/models/bert/promote", json={"targetStage": "bogus"}).status_code == 422
    assert c.post("/api/models/bert/promote", json={}).status_code == 422


def test_promote_404(make_client):
    r = make_client("models").post("/api/models/nope/promote", json={"targetStage": "staging"})
    assert r.status_code == 404
