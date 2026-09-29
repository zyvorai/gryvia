# Security Examples

Example YAML manifests for Gryvia security CRDs providing eBPF-based runtime protection.

## Examples

- **[security-policy.yaml](security-policy.yaml)** - `GryviaSecurityPolicy` with container escape, crypto mining, exfiltration, privilege escalation, and driver integrity detection enabled with auto-block

## Usage

```bash
kubectl apply -f security-policy.yaml
```
