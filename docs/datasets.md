# Datasets (GryviaDataset)

A `GryviaDataset` names a data source and the storage-operator materializes it into a PVC, one directory per
version, so training jobs, RAG ingestion and evaluation read the same bytes. It is opt-in.

## What is verified and what is not

| Verified | How |
| --- | --- |
| PVC creation (size, storage class, owner), the download Job for each source type (image, env, volumes, non-root pod), the termination-message result read back into status, versions and `keepLast` retention, a changed source downloading again, failures and invalid specs reported in status | Unit tests with a fake client (`operators/storage-operator/controllers/gryviadataset_controller_test.go`) |
| The download script (http with sha256, the swap into `/data/<version>`, pruning, the file count, size and checksum) on a real cluster | The "Datasets" step of `.github/workflows/e2e-ml.yml` with a stand-in HTTP server; these steps also passed on a single-node k3s host (2026-10-01) |
| Gateway routes and tenant scoping | `services/api-gateway/tests/test_datasets.py` |

| Not verified | Why |
| --- | --- |
| s3 downloads with the AWS CLI image, and nfs copies | No S3 bucket or NFS server in CI; the Jobs are unit-tested only |
| Large datasets, resumable downloads | The Job downloads everything again on a source change; there is no partial resume |

## Turning it on

```yaml
storageOperator:
  enabled: true
  datasets:
    enabled: true          # --enable-datasets
    namespace: ""          # --dataset-namespace: default for spec.namespace (empty: the chart's namespace)
    image: busybox:1.36    # http and nfs Jobs: sh, wget, sha256sum, find, stat
    s3Image: amazon/aws-cli:2.17.0
    defaultSize: 10Gi      # PVC size without spec.cache.size
```

Without `--enable-datasets` and with `platformCompletion.reportUnsupportedAPIs`, datasets keep getting
`Ready=False, reason UnsupportedAPI`, as before.

## Spec

The kind is cluster-scoped. The fields the controller reads:

| Field | Meaning |
| --- | --- |
| `source.type` | `http`, `s3` or `nfs` (`gcs`, `azure-blob`, `git-lfs` and `vast` are rejected in status) |
| `source.http.url`, `source.http.checksumURL` | One file. With `checksumURL` the first field of its first line must equal the sha256 of the file, or the download fails |
| `source.s3.bucket`, `prefix`, `region`, `credentialsSecret` | `aws s3 sync s3://bucket/prefix`. The Secret, in the dataset namespace, holds `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` |
| `source.nfs.server`, `source.nfs.path` | The export is mounted read-only and copied |
| `version` | Directory name of this version (default `latest`; letters, digits, `.`, `_`, `-`) |
| `namespace` | Where the PVC and Jobs live (default `--dataset-namespace`); consumers run there |
| `cache.size`, `cache.storageClass` | The PVC (ReadWriteOnce) |
| `versioning.enabled`, `versioning.retentionPolicy.keepLast` | Without versioning only the current version is kept; with it the newest `keepLast` (all when 0) |

`access`, `statistics`, `tags`, `license` and `description` are stored and shown, not enforced.

## What the controller does

1. Creates PVC `dataset-<name>` in the namespace, owned by the dataset (deleting the dataset deletes the data).
2. Hashes the source and version. When they differ from `status.sourceHash`, it runs Job `dataset-<name>-<hash>`
   (non-root, all capabilities dropped, 2 retries, deleted 24h after it finishes). The Job downloads into
   `/data/.tmp-<version>`, swaps it into `/data/<version>`, removes version directories that retention no longer
   keeps, and writes `{"files","bytes","sha256"}` to its termination message (the sha256 is over the sorted
   per-file checksums).
3. On success: `status.state: ready`, `currentVersion`, `subPath` (the version directory), `fileCount`,
   `totalSizeBytes`, and an entry in `versions` with the size and checksum. On failure: `state: error` and the
   reason in `message` (a checksum mismatch, for example). Condition `Ready` mirrors the state.

A job reads the current version with:

```yaml
volumes: [{name: data, persistentVolumeClaim: {claimName: dataset-corpus}}]
volumeMounts: [{name: data, mountPath: /data, subPath: v1, readOnly: true}]
```

The PVC is ReadWriteOnce: on a multi-node cluster its readers have to run on the node that mounted it, or use a
storage class with ReadWriteMany.

## Surfaces

* Gateway: `GET /api/datasets`, `GET /api/datasets/{name}`, `POST /api/datasets`, `DELETE /api/datasets/{name}`.
  A tenant sees and deletes only datasets whose namespace is one of theirs; a tenant's new dataset goes to their
  first namespace (another namespace is 403).
* CLI: `gryvia datasets list|get|create -f|delete`.
* Dashboard: the "Datasets" page under Models.

Example: [examples/datasets/http-dataset.yaml](../examples/datasets/http-dataset.yaml).
