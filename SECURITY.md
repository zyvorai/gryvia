# Security policy

## Reporting a vulnerability

Please do not open a public issue for security problems. Use GitHub's private
vulnerability reporting on this repository (Security > Report a vulnerability),
and include steps to reproduce and the affected version.

We aim to acknowledge reports within a few business days. Gryvia is early-stage
software; fixes ship in the next release and are noted in the changelog.

## Supported versions

Only the latest release on `main` receives security fixes.

## Threat model and known limits

Gryvia is alpha software. Read this before exposing it beyond a lab.

- **One shared key.** The API gateway and dashboard authenticate with a single shared bearer key
  (`GRYVIA_API_KEY`, sign in as `admin` with the key). There is no per-user identity, roles or audit
  trail for key-based sessions. OIDC login is supported by the gateway for SSO; per-user RBAC is not
  finished. Anyone who holds the key has full control of the platform's resources.
- **Well-known lab default.** Installs default to the key `Admin@321` so a quick start works. The
  dashboard shows a warning while it is in use. **Change it** for anything reachable from an untrusted
  network: `--set auth.apiKey=<secret>`, an existing Secret, or `auth.apiKey=""` to generate a random one.
- **Login hardening.** `POST /api/auth/login` compares credentials in constant time, slows failed
  attempts and is rate limited per client address (10 per minute).
- **TLS.** The dashboard and gateway serve HTTPS. The default certificate is self-signed; use
  `tls.mode=certManager` or `existingSecret` for a trusted one. Traffic between the dashboard and the
  gateway inside the cluster uses the same certificate without verification, so enable the chart's
  `networkPolicy.enabled` on shared clusters.
- **Cluster access.** The operators and the gateway run with broad Kubernetes RBAC on the `gryvia.io`
  resources, nodes and pods (see `helm/gryvia/templates/rbac.yaml`). Install Gryvia only on clusters where
  that is acceptable.
- **Containers.** Every workload runs as non-root with a read-only root filesystem, no privilege escalation and
  all capabilities dropped; the TLS Secret is mounted with `fsGroup`, so no init container needs root.
- **Images.** Release images and the Helm chart are signed with cosign (keyless) and carry SBOM and
  provenance attestations.

See [Authentication and TLS](https://zyvorai.github.io/gryvia/docs/guides/AUTH_AND_TLS) for configuration.
