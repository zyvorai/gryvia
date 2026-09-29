# Gryvia Flight Recorder (node-local preview)

The collector now keeps a bounded timeline of eBPF observations attributed to a
`GryviaAIJob`. The collector uses the host process cgroup's pod UID and the
Kubernetes pod list for its node. It never assigns an event to a job when the
pod cannot be resolved. The pod cache expires after one minute without a
successful API refresh.

Install the `network-intelligence` chart with `ebpf.enabled=true`. The
collector needs its existing pod-list RBAC, host PID access and `/host/proc`
mount. Inspect each node's attachment status first:

```sh
kubectl -n gryvia-network get pods -l app.kubernetes.io/component=collector -o wide
kubectl -n gryvia-network port-forward pod/<collector-pod> 9090:9090
curl -s localhost:9090/api/v1/ebpf/status
curl -s 'localhost:9090/api/v1/flight/diagnose?namespace=ml&job=train'
```

The response identifies its node and says `node-local; observed events only`.
Query each node hosting a job's pods; there is no cluster-wide aggregator yet.
Events are retained in memory (at most 2,048 per job and 128 jobs per node),
and the endpoint returns the latest 200. It returns 404 when no events were
attributed on this node. It is not a recording of actual GPU occupancy, RDMA
wire bytes, or complete training steps. Confirm GPU activity with DCGM/CUPTI;
NCCL and RDMA probes still require hardware validation.

For TCP connections observed from open to close, `tcp_retransmit` records the
number of retransmissions at close. The kernel can attribute retransmits that
occur in softirq context to the owning process captured at connection open.
Connections already open when the program attaches are not included.

## Identity caveats

- Attribution trusts the `gryvia.io/job` pod label set by the AI operator, so
  anyone who can create pods in a namespace can label a pod into that
  namespace's job timeline (never into another namespace's).
- The PID is resolved when the event is processed, not when it was emitted. A
  process that exits and whose PID is reused within that short gap could be
  mis-resolved; an exited process is simply dropped.
- Resolution needs `hostPID` and the `/host/proc` mount. When the collector runs
  in a cluster without that mount it resolves nothing rather than falling back
  to its own PID namespace.
- `counts` and `findings` cover the events returned (the latest 200), not the
  whole retained timeline.

## Restricting access

The collector's HTTP listener (`:9090`) serves metrics and every diagnostic
endpoint, including this one, without request authentication. It carries job
names and traffic timing, so do not expose it outside the cluster operators.

- Prefer `kubectl port-forward` (as above); it needs RBAC on the pod.
- Do not create a Service, Ingress or LoadBalancer for the collector.
- Restrict pod ingress on port 9090 to the monitoring namespace, for example:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: collector-restrict
  namespace: gryvia-network
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/component: collector
  policyTypes: [Ingress]
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: monitoring
      ports:
        - port: 9090
```

- The chart defaults to `collector.hostNetwork: true`, where the port is bound on the
  node's addresses and a pod NetworkPolicy does not apply. Use node firewall
  rules, or set `collector.hostNetwork=false` before relying on the policy above.

A cluster-wide, authenticated aggregation API and a CLI command need to precede
general availability.
