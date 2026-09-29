# Changelog

All notable changes to Gryvia are recorded here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
and the project aims to follow [Semantic Versioning](https://semver.org/) once it leaves alpha.

## [Unreleased]

### Added
- Helm chart `helm/gryvia` installs the operators, API gateway and dashboard in one release, with optional
  Ingress, cert-manager TLS, PodDisruptionBudgets and a NetworkPolicy.
- Tag-driven release workflow: multi-arch images on ghcr.io (mirrored to Docker Hub), signed with cosign,
  OCI Helm chart and CLI binaries.
- `scripts/install.sh` and `scripts/kind-demo.sh` (a GPU-free demo with fictional data on kind).
- `POST /api/auth/login` on the gateway with rate limiting, a `usingDefaultKey` flag on `/api/auth/me`, and a
  dashboard warning while the default lab key is in use. `auth.apiKey=""` generates a random key.
- Dashboard: job logs, events and pods; searchable, sortable, paginated tables with URL state; create forms for
  traces and security policies; structured editors for tuner parameter spaces and workflow steps.
- Accessibility pass: skip link, focus management, modal focus trapping, table captions, text alternatives for
  the service graph, heat map and charts.
- Open tabs recover automatically after a redeploy.

### Changed
- **Breaking:** the API version is now `gryvia.io/v1alpha1` (it was `v1`), matching the project's alpha status. Objects created under `v1` are not converted: export them, change `apiVersion`, and re-apply. `scripts/rename-version.sh` documents the change; `deploy-remote.sh` recreates CRDs that still store `v1` when they hold no objects.
- **Breaking:** API kinds renamed from `Fabric*` to `Gryvia*` (for example `FabricAIJob` is now `GryviaAIJob`).
  Recreate existing objects under the new kinds; `scripts/rename-kinds.sh` documents the mapping.
- The dashboard follows netra's design system and defaults to the system light or dark setting.
- Status vocabulary unified across pages (`Succeeded` counts as completed, `Scheduling` as pending).

### Fixed
- Costs are labelled as spend to date; empty states name the operator or collector that feeds the data instead
  of showing reassuring zeros.
- Rate limits count per client address instead of per proxy pod.

### Security
- Dashboard sessions are signed, expiring tokens issued by `/api/auth/login`; the API key is no longer stored in the browser.
- The dashboard bundle no longer contains a login credential; the gateway validates sign-in.
- The chart runs no root containers: the TLS Secret is mounted with `fsGroup` instead of a root init container.
- Login attempts are constant-time compared, delayed on failure and rate limited per client address.
