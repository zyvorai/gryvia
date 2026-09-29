# Authentication and TLS

## Signing in

The dashboard and API use one shared bearer key. Sign in as **`admin`** with the key as the password.
The dashboard sends the credentials to `POST /api/auth/login`; the gateway checks them and returns a signed,
short-lived session token (8 hours by default) that the browser uses for later requests. The web UI contains
no credential and the long-lived API key is never stored in the browser. Sessions end at expiry (the dashboard
returns you to the sign-in page) and also when the API key is rotated. Tune them on the gateway with
`GRYVIA_SESSION_TTL_SECONDS`, and set `GRYVIA_SESSION_SECRET` to sign sessions with a key of their own.

Scripts and `curl` can skip the login and send the key directly:

```bash
curl -k -H "Authorization: Bearer $GRYVIA_API_KEY" https://<host>/api/cluster/stats
```

### The default key

Installs default to the well-known lab key `Admin@321` so a quick start needs no setup. While it is in
use the dashboard shows a dismissible warning. Before exposing Gryvia to anyone else pick one of:

```bash
# 1. Your own key
helm upgrade --install gryvia oci://ghcr.io/zyvorai/charts/gryvia -n gryvia-system \
  --set auth.apiKey='a-long-random-secret'

# 2. Let the chart generate a random key once and keep it across upgrades
helm upgrade --install gryvia oci://ghcr.io/zyvorai/charts/gryvia -n gryvia-system --set auth.apiKey=
kubectl -n gryvia-system get secret gryvia-api-key -o jsonpath='{.data.GRYVIA_API_KEY}' | base64 -d

# 3. A Secret you manage (key: GRYVIA_API_KEY)
helm upgrade --install gryvia oci://ghcr.io/zyvorai/charts/gryvia -n gryvia-system \
  --set auth.existingSecret=my-gryvia-key
```

`./scripts/install.sh` and `./scripts/deploy-remote.sh` read `GRYVIA_API_KEY` from the environment.

### Rate limiting

Sign-in is limited to 10 attempts per minute per client address, and a failed attempt is delayed by half
a second. Behind the dashboard's proxy the gateway uses the forwarded client address, and it trusts that
header only when the connection comes from a private or loopback address.

### Single sign-on

The gateway can validate OIDC JWTs (`OIDC_ENABLED`, `OIDC_ISSUER_URL`, `OIDC_CLIENT_ID`). The dashboard
then offers an SSO button next to the key login. Roles and per-user permissions are not finished; treat
every authenticated user as an administrator.

## TLS

The dashboard (port 443) and the gateway (port 8080) serve HTTPS. `tls.mode` in the chart selects the
certificate source:

| Mode | Behaviour |
|------|-----------|
| `selfSigned` (default) | The chart creates one self-signed certificate in the `gryvia-tls` Secret and reuses it on upgrades. Add names or IPs with `tls.extraSANs`. Browsers warn once. |
| `certManager` | Creates a cert-manager `Certificate` for the issuer in `tls.certManager.issuerName` (`issuerKind` defaults to `ClusterIssuer`). |
| `existingSecret` | Uses a `kubernetes.io/tls` Secret you created, named by `tls.secretName`. |

To rotate the self-signed certificate, delete the `gryvia-tls` Secret and run `helm upgrade`.

To publish the dashboard under a real hostname enable the Ingress and let it terminate TLS:

```bash
helm upgrade --install gryvia oci://ghcr.io/zyvorai/charts/gryvia -n gryvia-system \
  --set ui.ingress.enabled=true --set ui.ingress.className=nginx \
  --set ui.ingress.host=gryvia.example.com --set ui.ingress.tlsSecretName=gryvia-example-com \
  --set 'ui.ingress.annotations.nginx\.ingress\.kubernetes\.io/backend-protocol=HTTPS'
```

The dashboard proxy talks to the gateway over HTTPS inside the cluster using the shared certificate and does
not verify it. On shared clusters enable `--set networkPolicy.enabled=true` so only the dashboard pods can
reach the gateway.
