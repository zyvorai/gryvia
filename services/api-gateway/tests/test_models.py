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


CREATE = {"name": "bert-v2", "version": "v2", "artifacts": {"s3Path": "s3://bucket/bert/v2"}}


def test_create_minimal(make_client, fake_k8s):
    r = make_client("models").post("/api/models", json=CREATE)
    assert r.status_code == 201
    stored = fake_k8s.store[("gryviamodelregistries", NS, "bert-v2")]
    assert stored["kind"] == "GryviaModelRegistry"
    # modelName defaults to the object name; the stage to dev; nothing else is invented.
    assert stored["spec"] == {"modelName": "bert-v2", "version": "v2", "stage": "dev",
                              "artifacts": {"s3Path": "s3://bucket/bert/v2"}}
    body = r.json()
    assert body["metadata"]["name"] == "bert-v2"
    assert body["spec"]["stage"] == "dev" and body["spec"]["artifacts"] == ["s3://bucket/bert/v2"]


def test_create_full(make_client, fake_k8s):
    r = make_client("models").post("/api/models", json={
        "name": "llama", "modelName": "Llama-3.1", "version": "1.0.0+rc1", "stage": "production",
        "description": "chat model", "sourceJob": "train-7", "autoServe": True,
        "artifacts": {"pvcName": "models", "subPath": "llama/1", "format": "safetensors", "s3Path": "s3://b/l"},
        "servingConfig": {"backend": "triton", "replicas": 2, "gpuCount": 1, "gpuType": "A100-80G"},
    })
    assert r.status_code == 201
    spec = fake_k8s.store[("gryviamodelregistries", NS, "llama")]["spec"]
    assert spec == {
        "modelName": "Llama-3.1", "version": "1.0.0+rc1", "stage": "production", "description": "chat model",
        "source": {"jobRef": "train-7"}, "autoServe": True,
        "artifacts": {"s3Path": "s3://b/l", "pvcName": "models", "subPath": "llama/1", "format": "safetensors"},
        "servingConfig": {"backend": "triton", "replicas": 2, "gpuCount": 1, "gpuType": "A100-80G"},
    }
    assert r.json()["spec"]["sourceJob"] == "train-7"


def test_create_serving_config_omits_zero_gpu(make_client, fake_k8s):
    make_client("models").post("/api/models", json={**CREATE, "autoServe": True, "servingConfig": {}})
    sc = fake_k8s.store[("gryviamodelregistries", NS, "bert-v2")]["spec"]["servingConfig"]
    # gpuCount 0 is the CRD default: it is left out so the operator's --autoserve-default-gpu-count applies.
    assert sc == {"backend": "vllm", "replicas": 1}


def test_create_uses_the_tenant_namespace(make_client, fake_k8s):
    make_client("models").post("/api/models", json=CREATE)
    assert [k for k in fake_k8s.store] == [("gryviamodelregistries", NS, "bert-v2")]
    # And the new object is visible only through the same scoping as the other routes.
    c = make_client("models")
    assert c.get("/api/models/bert-v2").status_code == 200


def test_create_duplicate(make_client, fake_k8s):
    seed(fake_k8s, name="bert-v2")
    assert make_client("models").post("/api/models", json=CREATE).status_code == 409


@pytest.mark.parametrize("patch", [
    {"name": "Bad_Name"}, {"name": ""}, {"name": "a" * 64}, {"version": ""}, {"version": "v 1"},
    {"modelName": "bad name"}, {"stage": "archived"}, {"stage": "bogus"}, {"sourceJob": "Bad Job"},
    {"description": "x" * 1025}, {"extra": 1}, {"artifacts": {}}, {"artifacts": {"s3Path": "not a url"}},
    {"artifacts": {"pvcName": "Bad_PVC"}}, {"artifacts": {"pvcName": "p", "subPath": "../etc"}},
    {"artifacts": {"pvcName": "p", "subPath": "a/../b"}}, {"artifacts": {"s3Path": "s3://b/x", "extra": 1}},
    {"servingConfig": {"backend": "onnx"}}, {"servingConfig": {"replicas": 0}}, {"servingConfig": {"gpuCount": 65}},
    {"servingConfig": {"gpuType": "a b"}}, {"servingConfig": {"extra": 1}},
])
def test_create_validation(make_client, fake_k8s, patch):
    r = make_client("models").post("/api/models", json={**CREATE, **patch})
    assert r.status_code == 422, r.text
    assert not fake_k8s.store


def test_create_missing_fields(make_client, fake_k8s):
    c = make_client("models")
    assert c.post("/api/models", json={}).status_code == 422
    assert c.post("/api/models", json={"name": "m", "version": "v1"}).status_code == 422
    assert not fake_k8s.store


def test_rollback_sets_the_request_annotation(make_client, fake_k8s):
    seed(fake_k8s, stage="production", servingConfig={"serviceName": "chat"})
    fake_k8s.store[("gryviamodelregistries", NS, "bert")]["status"]["previousVersion"] = "bert-v0"
    r = make_client("models").post("/api/models/bert/rollback")
    assert r.status_code == 202
    assert r.json()["status"]["previousVersion"] == "bert-v0"
    ann = fake_k8s.store[("gryviamodelregistries", NS, "bert")]["metadata"]["annotations"]
    assert ann["gryvia.io/rollback-requested"].startswith("api at ")


@pytest.mark.parametrize("stage,serving,previous", [("staging", {"serviceName": "chat"}, "v0"),
                                                    ("production", {}, "v0"),
                                                    ("production", {"serviceName": "chat"}, "")])
def test_rollback_refused(make_client, fake_k8s, stage, serving, previous):
    seed(fake_k8s, stage=stage, servingConfig=serving)
    fake_k8s.store[("gryviamodelregistries", NS, "bert")]["status"]["previousVersion"] = previous
    assert make_client("models").post("/api/models/bert/rollback").status_code == 409
    assert "annotations" not in fake_k8s.store[("gryviamodelregistries", NS, "bert")]["metadata"]


def test_rollback_404(make_client):
    assert make_client("models").post("/api/models/nope/rollback").status_code == 404
