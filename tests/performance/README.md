# TensorReaper Performance Benchmarks

Performance benchmarking tools for TensorReaper platform.

## Benchmarks

### 1. Sequential Job Submission
Tests scheduler performance with jobs submitted one at a time.

### 2. Parallel Job Submission
Tests concurrent job submission with controlled parallelism.

### 3. Burst Submission
Tests system behavior under sudden load spikes.

### 4. Large-Scale Submission
Tests platform scalability with hundreds of jobs.

## Running Benchmarks

```bash
cd tests/performance
go run benchmark.go
```

## Results

Results are saved to `benchmark_results.json`:

```json
[
  {
    "test": "Sequential Submission",
    "total_jobs": 10,
    "successful_jobs": 10,
    "failed_jobs": 0,
    "avg_time_to_schedule_ms": 2500,
    "avg_time_to_complete_ms": 15000,
    "total_duration_ms": 180000,
    "jobs_per_second": 0.055
  }
]
```

## Metrics

- **Total Jobs**: Number of jobs submitted
- **Successful Jobs**: Jobs that completed successfully
- **Failed Jobs**: Jobs that failed
- **Avg Time to Schedule**: Average time from submission to running state
- **Avg Time to Complete**: Average total execution time
- **Total Duration**: Total benchmark duration
- **Jobs/Second**: Job submission throughput

## Performance Targets

Target metrics for a healthy cluster:

- **Time to Schedule**: < 5 seconds
- **Jobs/Second**: > 10 jobs/second
- **Success Rate**: > 95%
- **Large Scale (100 jobs)**: Complete in < 10 minutes

## Customization

Edit `benchmark.go` to customize:
- Number of jobs per test
- Parallelism levels
- Job specifications
- Timeout values

## CI/CD Integration

Benchmarks run nightly and on major releases to track performance trends.

## Interpreting Results

Good performance:
```
Avg Time to Schedule: < 5s
Jobs/Second: > 10
Success Rate: 100%
```

Needs investigation:
```
Avg Time to Schedule: > 30s
Jobs/Second: < 1
Success Rate: < 90%
```

Common issues:
- High schedule time: Quota limits, node availability
- Low throughput: API server limits, operator performance
- Failures: Resource constraints, configuration errors
