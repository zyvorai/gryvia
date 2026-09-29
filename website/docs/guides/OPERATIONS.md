# Operations: upgrade, uninstall, backup, troubleshooting

## Upgrade

Helm installs CRDs on the first install only and never upgrades them, so apply the CRDs first:

```bash
kubectl apply --server-side --force-conflicts -f crds/        # from the release you are upgrading to
helm upgrade gryvia oci://ghcr.io/zyvorai/charts/gryvia -n gryvia-system --reuse-values --version <new>
```

Gryvia is alpha and `gryvia.io/v1alpha1` may change between releases; read the [changelog](https://github.com/zyvorai/gryvia/blob/main/CHANGELOG.md)
before upgrading. **Two changes are breaking: the `Fabric*` → `Gryvia*` kind rename and the move from `gryvia.io/v1` to `gryvia.io/v1alpha1`**: objects created under the old kinds
(`FabricAIJob`, ...) or the old `v1` version are not converted. Export them, change `kind:` and `apiVersion:` and re-apply, then delete the old CRDs (`scripts/rename-kinds.sh` and `scripts/rename-version.sh` document the mapping).

The self-signed certificate (`gryvia-tls`) and the API key Secret are kept across upgrades. Open dashboard tabs
reload themselves once when a new version is deployed.

## Uninstall

```bash
helm uninstall gryvia -n gryvia-system
```

CRDs, your custom resources and the `gryvia-tls` Secret are kept on purpose. To remove everything, including
data:

```bash
kubectl get crd -o name | grep '\.gryvia\.io$' | xargs kubectl delete    # deletes every Gryvia object
kubectl delete namespace gryvia-system
```

## Back up

Gryvia keeps its state in Kubernetes custom resources. Export them all:

```bash
kinds=$(kubectl get crd -o name | grep '\.gryvia\.io$' | sed 's#.*/##' | paste -sd, -)
kubectl get "$kinds" -A -o yaml > gryvia-backup.yaml
```

Restore with `kubectl apply -f gryvia-backup.yaml` (status fields are recomputed by the operators). Also back
up the `gryvia-api-key` Secret if you generated a random key.

## Troubleshooting

| Symptom | What to check |
|---------|---------------|
| Pods stuck in `ImagePullBackOff` | The images are pulled from `global.imageRegistry` (default `ghcr.io/zyvorai`). Check the tag exists, or set `imagePullSecrets` for a private mirror. |
| Dashboard shows "Wrong username or password." | Sign in as `admin` with the API key. Read a generated key with `kubectl -n gryvia-system get secret gryvia-api-key -o jsonpath='{.data.GRYVIA_API_KEY}' \| base64 -d`. |
| "Too many sign-in attempts" | Sign-in is limited to 10 per minute per client address. Wait a minute. |
| Browser certificate warning | Expected with the default self-signed certificate. See [Authentication and TLS](./AUTH_AND_TLS.md) to use your own. |
| A page says "Gryvia was updated" | A new version was deployed while the tab was open. Reload. |
| Nodes show phase `Failed` with "node not found" | The `GryviaGpuNode` names a Kubernetes node that does not exist (the kind demo data is fictional). Register real nodes. |
| GPU or network pages say no source is connected | They need the collectors and operators that feed them (DCGM exporter, network-intelligence, security operator). The page names what is missing. |
| A job stays `Pending` | `kubectl -n <ns> describe gryviaaijob <name>` shows the scheduling reason, for example no node with the requested GPU type or a quota limit. |
| Operators restart or lose leadership | `kubectl -n gryvia-system logs deploy/gryvia-gpu-operator` (and `-ai-`, `-quota-`). Leader election uses leases in `gryvia-system`. |

Collect logs for a bug report with `kubectl -n gryvia-system logs -l app=gryvia-api-gateway --tail=200` and the
operator deployments above; remove secrets before sharing.
