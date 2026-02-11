apiVersion: kubefabric.ai/v1
kind: FabricGpuNode
metadata:
  name: ${node_name}
spec:
  nodeName: ${node_name}
  gpuType: ${gpu_type}
  gpuCount: ${gpu_count}
  rdma: ${rdma_enabled}
  sriov: true
  interconnect: NVLink
  %{ if rdma_enabled }
  bandwidth: "400Gbps"
  %{ endif }
  healthCheck:
    enabled: true
    intervalSeconds: 60
  labels:
    kubefabric.ai/gpu: "${gpu_type}"
    kubefabric.ai/gpu-count: "${gpu_count}"
    kubefabric.ai/rdma: "${rdma_enabled}"
    %{ if rdma_enabled }
    kubefabric.ai/rdma-device: "${rdma_device}"
    %{ endif }
