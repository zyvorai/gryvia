# Gryvia E2E Tests

End-to-end tests for Gryvia platform.

## Prerequisites

- Running Kubernetes cluster
- Gryvia operators deployed
- `kubectl` configured
- Go 1.22+

## Running Tests

### Run all E2E tests

```bash
cd tests/e2e
go test -v -timeout 30m ./...
```

### Run specific test

```bash
go test -v -run TestE2E_AIJobLifecycle
go test -v -run TestE2E_QuotaEnforcement
go test -v -run TestE2E_NodeRegistration
go test -v -run TestE2E_StorageProvisioning
```

### Run with parallel execution

```bash
go test -v -parallel 4 ./...
```

## Test Scenarios

### 1. AI Job Lifecycle Test
Tests complete job lifecycle:
- Create GryviaAIJob
- Wait for job to be scheduled
- Verify job transitions through phases
- Clean up job

### 2. Quota Enforcement Test
Tests quota management:
- Create team quota
- Verify quota status updates
- Check quota limits
- Delete quota

### 3. Node Registration Test
Tests GPU node management:
- List all GPU nodes
- Verify node specifications
- Check GPU counts and types

### 4. Storage Provisioning Test
Tests storage backend:
- Create storage configuration
- Wait for CSI driver deployment
- Verify storage ready status
- Clean up storage

## Test Environment

The tests create a dedicated namespace `gryvia-e2e-test` for isolation.

All resources are cleaned up after tests complete.

## CI/CD Integration

These tests run automatically in GitHub Actions on:
- Push to main branch
- Pull requests
- Scheduled daily runs

## Troubleshooting

### Tests fail to connect to cluster
```bash
export KUBECONFIG=/path/to/kubeconfig
```

### Timeout errors
Increase timeout:
```bash
go test -v -timeout 60m ./...
```

### Check test logs
```bash
go test -v -timeout 30m ./... 2>&1 | tee test.log
```

## Writing New Tests

Example test structure:

```go
func TestE2E_NewFeature(t *testing.T) {
    setupTestCluster(t)
    defer teardownTestCluster(t)

    ctx := context.Background()

    t.Run("SubTest1", func(t *testing.T) {
        // Test logic
        require.NoError(t, err)
        assert.Equal(t, expected, actual)
    })
}
```

## Performance Tests

For performance testing, see `tests/performance/`.
