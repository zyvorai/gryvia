## External System Integrations

TensorReaper integrates seamlessly with popular ML platforms and tools.

## Experiment Tracking

### Weights & Biases

```yaml
apiVersion: tensorreaper.ai/v1
kind: FabricAIJob
metadata:
  name: training-with-wandb
spec:
  framework: pytorch

  # W&B integration
  integrations:
    wandb:
      enabled: true
      project: my-project
      entity: my-team
      apiKeySecret: wandb-api-key
      tags: [experiment-1, baseline]

  env:
    - name: WANDB_API_KEY
      valueFrom:
        secretKeyRef:
          name: wandb-api-key
          key: api-key

  command:
    - python
    - train.py
    - --wandb-project=my-project
```

**Auto-Integration**: TensorReaper automatically logs:
- GPU metrics (utilization, memory, temperature)
- Cost per epoch
- Resource allocation
- Job metadata

### Neptune.ai

```yaml
integrations:
  neptune:
    enabled: true
    project: team/project
    apiTokenSecret: neptune-token
    tags: [production, v2]
```

### MLflow

```yaml
integrations:
  mlflow:
    enabled: true
    trackingUri: http://mlflow:5000
    experimentName: llm-training
    runName: "run-{{ .job.name }}"
```

### TensorBoard

```yaml
integrations:
  tensorboard:
    enabled: true
    logDir: /tensorboard-logs
    serviceType: LoadBalancer
```

Access: `http://<external-ip>:6006`

## Model Registries

### HuggingFace Hub

```yaml
apiVersion: tensorreaper.ai/v1
kind: FabricJobHook
metadata:
  name: push-to-huggingface
spec:
  trigger: post-completion
  condition: "{{job.metrics.accuracy}} > 0.95"

  action:
    type: k8sJob
    k8sJob:
      image: python:3.11
      command:
        - python
        - -c
        - |
          from huggingface_hub import HfApi
          api = HfApi()
          api.upload_folder(
              folder_path="/checkpoints/{{ .job.name }}",
              repo_id="my-org/my-model",
              repo_type="model"
          )
      env:
        - name: HF_TOKEN
          valueFrom:
            secretKeyRef:
              name: hf-token
              key: token
```

### AWS SageMaker Model Registry

```yaml
spec:
  trigger: post-completion

  action:
    type: exec
    exec:
      command:
        - aws
        - sagemaker
        - create-model-package
        - --model-package-group-name=my-models
        - --model-data=s3://bucket/{{ .job.name }}/model.tar.gz
```

## Data Platforms

### DVC (Data Version Control)

```yaml
apiVersion: tensorreaper.ai/v1
kind: FabricDataset
metadata:
  name: my-dataset
spec:
  source:
    type: git-lfs
    git:
      repository: https://github.com/org/data-repo
      ref: main
      path: datasets/imagenet

  versioning:
    enabled: true
    strategy: git-lfs
```

### Pachyderm

```yaml
integrations:
  pachyderm:
    enabled: true
    project: ml-pipelines
    pipeline: training-pipeline
    input: data-repo@master
```

## Workflow Orchestration

### Airflow

```python
# Airflow DAG
from airflow import DAG
from airflow.providers.cncf.kubernetes.operators.kubernetes_pod import KubernetesPodOperator

with DAG('ml_training', schedule_interval='@daily') as dag:
    train = KubernetesPodOperator(
        task_id='train_model',
        namespace='default',
        name='training-job',
        image='tensorreaper/job-operator:1.0.0',
        cmds=['tensorreaper'],
        arguments=['submit', 'job.yaml'],
        env_vars={'KUBECONFIG': '/config/kubeconfig'}
    )
```

### Kubeflow Pipelines

```python
from kfp import dsl

@dsl.pipeline(name='TensorReaper Training')
def training_pipeline():
    train_op = dsl.ContainerOp(
        name='Submit Job',
        image='tensorreaper/cli:1.0.0',
        command=['tensorreaper', 'submit', 'job.yaml']
    )
```

## Monitoring & Observability

### Datadog

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: datadog-config
data:
  datadog.yaml: |
    logs_enabled: true
    apm_enabled: true

    # Custom checks for TensorReaper
    instances:
      - prometheus_url: http://prometheus:9090
        namespace: tensorreaper
        metrics:
          - tensorreaper_gpu_utilization
          - tensorreaper_job_duration
          - tensorreaper_cost_total
```

### New Relic

```yaml
integrations:
  newrelic:
    enabled: true
    licenseKey: <secret>
    appName: tensorreaper-cluster
```

### Grafana Cloud

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: grafana-cloud
data:
  username: <base64-encoded>
  api-key: <base64-encoded>
  prometheus-endpoint: <base64-encoded>
```

## CI/CD Integration

### GitHub Actions

```yaml
name: Train Model
on:
  push:
    branches: [main]

jobs:
  train:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2

      - name: Submit Training Job
        run: |
          tensorreaper submit job.yaml \
            --gpu-type A100-80G \
            --gpu-count 8 \
            --wait
        env:
          KUBECONFIG: ${{ secrets.KUBECONFIG }}

      - name: Get Job Status
        run: |
          tensorreaper job status training-job-${{ github.run_id }}
```

### GitLab CI

```yaml
train-model:
  stage: train
  image: tensorreaper/cli:1.0.0
  script:
    - tensorreaper submit job.yaml --wait
    - tensorreaper job logs training-job
  only:
    - main
```

### Jenkins

```groovy
pipeline {
    agent any
    stages {
        stage('Train') {
            steps {
                sh 'tensorreaper submit job.yaml'
                sh 'tensorreaper job wait training-job'
            }
        }
    }
}
```

## Slack Integration

### Job Notifications

```yaml
apiVersion: tensorreaper.ai/v1
kind: FabricJobHook
metadata:
  name: slack-notifications
spec:
  trigger: post-completion

  action:
    type: webhook
    webhook:
      url: https://hooks.slack.com/services/YOUR/WEBHOOK/URL
      method: POST
      body: |
        {
          "text": "Job {{ .job.name }} completed",
          "blocks": [
            {
              "type": "section",
              "text": {
                "type": "mrkdwn",
                "text": "*Status:* {{ .job.status }}\n*Duration:* {{ .job.duration }}\n*Cost:* ${{ .job.cost }}"
              }
            }
          ]
        }
```

### Slack Bot Commands

```bash
# In Slack channel #ml-jobs
/tensorreaper submit job.yaml
/tensorreaper status training-job-42
/tensorreaper logs training-job-42 --tail 50
/tensorreaper cost --team ml-research --month current
```

## Cost Management

### CloudHealth

```yaml
integrations:
  cloudhealth:
    enabled: true
    apiKey: <secret>
    reportingTags:
      - team
      - project
      - environment
```

### Kubecost Integration

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kubecost-config
data:
  custom-metrics: |
    - name: gpu_cost_hourly
      query: tensorreaper_job_cost_total / tensorreaper_job_duration_hours
```

## Authentication

### OAuth2 / OIDC

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: tensorreaper-auth
data:
  oauth2-config.yaml: |
    issuer: https://accounts.google.com
    clientID: your-client-id
    clientSecret: your-client-secret
    redirectURL: https://tensorreaper.company.com/callback
```

### LDAP

```yaml
auth:
  ldap:
    enabled: true
    server: ldap://ldap.company.com
    baseDN: dc=company,dc=com
    userFilter: (uid=%s)
    groupFilter: (memberUid=%s)
```

## Backup & DR

### Velero

```yaml
apiVersion: velero.io/v1
kind: Schedule
metadata:
  name: tensorreaper-backup
spec:
  schedule: "0 2 * * *"  # Daily at 2 AM
  template:
    includedNamespaces:
      - tensorreaper
    includedResources:
      - fabricaijobs
      - fabricqueues
      - fabricusers
      - persistentvolumeclaims
```

### S3 Backup

```yaml
apiVersion: tensorreaper.ai/v1
kind: FabricJobHook
metadata:
  name: backup-checkpoints
spec:
  trigger: on-checkpoint

  action:
    type: exec
    exec:
      command:
        - aws
        - s3
        - sync
        - /checkpoints/{{ .job.name }}
        - s3://backups/checkpoints/{{ .job.name }}
      timeout: 10m
```

## Security Scanning

### Trivy

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: image-scanning
spec:
  schedule: "0 */6 * * *"  # Every 6 hours
  jobTemplate:
    spec:
      template:
        spec:
          containers:
            - name: trivy
              image: aquasec/trivy:0.58.0
              command:
                - trivy
                - image
                - --severity=HIGH,CRITICAL
                - nvcr.io/nvidia/pytorch:24.01-py3
```

## API Integration Examples

### Python SDK

```python
from tensorreaper import Client

client = Client()

# Submit job
job = client.jobs.create(
    name="my-training-job",
    framework="pytorch",
    gpu_type="A100-80G",
    gpu_count=8,
    image="nvcr.io/nvidia/pytorch:24.01-py3",
    command=["python", "train.py"]
)

# Wait for completion
job.wait()

# Get metrics
metrics = job.get_metrics()
print(f"Accuracy: {metrics['accuracy']}")
print(f"Cost: ${metrics['cost']}")
```

### REST API

```bash
# Submit job
curl -X POST https://tensorreaper-api/v1/jobs \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "training-job",
    "framework": "pytorch",
    "resources": {
      "gpuType": "A100-80G",
      "gpuCount": 8
    },
    "image": "nvcr.io/nvidia/pytorch:24.01-py3",
    "command": ["python", "train.py"]
  }'

# Get job status
curl https://tensorreaper-api/v1/jobs/training-job \
  -H "Authorization: Bearer $TOKEN"
```

### GraphQL API

```graphql
mutation SubmitJob {
  createJob(input: {
    name: "training-job"
    framework: PYTORCH
    resources: {
      gpuType: "A100-80G"
      gpuCount: 8
    }
    image: "nvcr.io/nvidia/pytorch:24.01-py3"
    command: ["python", "train.py"]
  }) {
    id
    status
    createdAt
  }
}

query GetJobMetrics {
  job(name: "training-job") {
    metrics {
      accuracy
      loss
      cost
      duration
    }
  }
}
```

## Best Practices

### 1. Secret Management

Use Kubernetes secrets for API keys:

```bash
kubectl create secret generic wandb-api-key \
  --from-literal=api-key=YOUR_API_KEY

kubectl create secret generic hf-token \
  --from-literal=token=YOUR_HF_TOKEN
```

### 2. Webhooks

Use retry logic for webhooks:

```yaml
webhook:
  url: https://api.example.com/callback
  retry:
    attempts: 3
    backoff: exponential
```

### 3. Rate Limiting

Respect API rate limits:

```yaml
integrations:
  wandb:
    rateLimit:
      maxRequestsPerSecond: 10
      burst: 20
```

### 4. Monitoring

Monitor integration health:

```promql
rate(tensorreaper_integration_errors_total[5m])
```

## Troubleshooting

### Integration Not Working

```bash
# Check integration logs
kubectl logs -n tensorreaper deploy/integrations-controller

# Test webhook
tensorreaper integrations test wandb

# View integration status
tensorreaper integrations status
```

### Authentication Failures

```bash
# Verify secret
kubectl get secret wandb-api-key -o yaml

# Test credentials
tensorreaper integrations auth-test wandb
```

## Support

- Integration Issues: https://github.com/ssahani/tensor-reaper/issues
- Integration Requests: https://github.com/ssahani/tensor-reaper/discussions

---

*See [ADVANCED_FEATURES.md](ADVANCED_FEATURES.md) for more information*
