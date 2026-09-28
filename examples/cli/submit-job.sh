#!/usr/bin/env bash
# Example: Submit a training job using Gryvia CLI

set -e

echo "Submitting LLM training job..."

gryvia submit -f - <<EOF
apiVersion: gryvia.io/v1
kind: FabricAIJob
metadata:
  name: llm-training
  namespace: ml-training
spec:
  framework: pytorch
  distributed:
    enabled: true
    strategy: ddp
    nodes: 4
    gpusPerNode: 8

  resources:
    gpuType: H100
    gpuCount: 8
    memory: 1Ti
    cpu: 96

  image: nvcr.io/nvidia/pytorch:24.01-py3

  command:
    - torchrun
    - --nproc_per_node=8
    - --nnodes=4
    - --master_port=29500
    - train_llm.py
    - --model=llama-70b
    - --batch-size=4

  env:
    - name: NCCL_IB_HCA
      value: "mlx5_0,mlx5_1"
    - name: NCCL_NET_GDR_LEVEL
      value: "5"
EOF

echo ""
echo "✓ Job submitted!"
echo ""
echo "Monitor with:"
echo "  gryvia status llm-training"
echo "  gryvia logs llm-training --follow"
