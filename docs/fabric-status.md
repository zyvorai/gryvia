# Fabric status and quota pacing

Two opt-in features of the eBPF collector (`collector/`, chart `helm/network-intelligence`). Both are **off by default**;
with the defaults the chart renders no new flags and no new RBAC.

## 1. Fabric status in `GryviaFabricSignal`

Fabric signals are attributed to a job through the Flight Recorder's resolver: host PID, the pod UID in
`/host/proc/<pid>/cgroup`, the node's pod list, its `gryvia.io/job` label. Anything that does not resolve stays under
`_unattributed/<process>` (node counters under `_node/cnp` and `_node/pfc`); the resolver fails closed and never guesses.

With `-publish-fabric-status` (chart `ebpf.publishFabricStatus=true`) the collector, every 30 s and only inside a cluster:

- lists `GryviaFabricSignal` objects in the namespace of each attributed job,
- and for those whose `spec.jobRef` equals the job name sends a merge-patch to `.../gryviafabricsignals/<name>/status`.

It creates nothing, never touches `spec`, skips `_unattributed` and `_node/*`, handles at most 256 jobs per cycle, and
logs (never fails on) API errors. All status fields are sent every time, so a value that fell to zero is overwritten;
`gdsHitRatio` is cleared when GPUDirect Storage is not measured. RBAC added to the collector ClusterRole only when the
value is on: `list` on `gryviafabricsignals`, `patch` on `gryviafabricsignals/status`.

Limits: each node's collector writes its own view, so for a job on several nodes the last writer wins; the status is
advisory. Unit-tested against a fake API server; not run against a real cluster.

## 2. Quota-driven pacing (MUTATING)

`GryviaQuota.spec.network.maxEgressMbps` (optional, 1 to 32000) caps the outbound TCP rate of **each new connection**
opened by pods in `spec.namespaces` (`SO_MAX_PACING_RATE` via the `quota_pace` sockops program). It is not an aggregate
bandwidth limit, does not apply to UDP, RDMA or connections that already exist, and a new container can be unpaced for up
to one sync interval (30 s).

It runs only when all of these are set: `-quota-pace`, `-cgroup-path` (cgroup v2 root, for example
`/host/sys/fs/cgroup`), `-quota-pace-sync`, a cluster (`KUBERNETES_SERVICE_HOST`), `NODE_NAME` and a readable service
account namespace. Chart values: `ebpf.cgroupPath`, `ebpf.quotaPace.enabled`, `ebpf.quotaPace.sync`; the chart adds `list`
on `gryviaquotas` to the collector ClusterRole only when `sync` is on. Start with `ebpf.quotaPace.dryRun=true` (flag
`-quota-pace-dry-run`): it logs `would grant` / `would revoke` and never writes the map.

Every 30 s the collector lists quotas with a cap, lists the Running pods of each listed namespace on this node, resolves
each pod's cgroup id (inode of the pod's cgroup directory and its container directories under `-cgroup-path`; cgroupfs
and systemd layouts) and grants the `Pacer` a 2-minute lease. Safety rules, each covered by a unit test:

- an API error (quota list or any pod list) grants nothing new and revokes nothing; leases then expire on their own
  (fail open);
- only cgroups below `-cgroup-path` are granted (validated UID and QoS, symlinks and escapes refused, root never granted);
- `kube-system`, `gryvia-system`, `kube-public`, `kube-node-lease` and the collector's own namespace are never paced,
  and the collector refuses to sync when it cannot tell its own namespace;
- caps outside 1..32000 Mbit/s are skipped (never clamped up); of several quotas the strictest wins; the program also
  floors any rate at 1 Mbit/s;
- a removed quota, a cap of 0 or a vanished pod is revoked; expiry deletes the map entry; shutdown deletes every entry;
- every grant, renewal, revoke, expiry and shutdown deletion is logged;
- nothing is exposed over HTTP.

An already paced socket keeps its rate until it closes; revoking only unpaces new connections.

Verified on the Linux 7.0 test host with the real `pace_rate` map and real sockops attached to a throwaway cgroup (never
the root): the granted cgroup's new loopback connections were clamped to the cap, a sibling cgroup stayed unlimited, and
revoke, a failed list, expiry and shutdown behaved as described. The Kubernetes side was faked there. Unverified: a real
API server, real quotas on real workloads, and GPU or RDMA traffic.
