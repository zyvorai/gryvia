# KubeFabric Documentation

Complete documentation for KubeFabric - Enterprise GPU Compute Platform.

## Documentation Structure

### Getting Started
- [Quick Start Guide](getting-started/quickstart.md) - Get up and running in 10 minutes
- [Installation Guide](getting-started/installation.md) - Detailed installation instructions
- [Architecture Overview](getting-started/architecture.md) - System architecture and components
- [Concepts](getting-started/concepts.md) - Core concepts and terminology

### User Guides
- [Submitting Jobs](user-guide/jobs.md) - How to submit and manage AI workloads
- [GPU Management](user-guide/gpus.md) - Working with GPU resources
- [Storage](user-guide/storage.md) - Using parallel filesystems
- [Networking](user-guide/networking.md) - RDMA and high-speed networking
- [Quotas](user-guide/quotas.md) - Understanding and managing quotas
- [Cost Tracking](user-guide/costs.md) - Monitoring and optimizing costs

### Administrator Guides
- [Cluster Setup](admin-guide/cluster-setup.md) - Setting up a KubeFabric cluster
- [Node Configuration](admin-guide/nodes.md) - Configuring GPU nodes
- [Security](admin-guide/security.md) - Security best practices
- [Monitoring](admin-guide/monitoring.md) - Observability and alerting
- [Backup & Restore](admin-guide/backup-restore.md) - Disaster recovery
- [Upgrades](admin-guide/upgrades.md) - Upgrading KubeFabric
- [Troubleshooting](admin-guide/troubleshooting.md) - Common issues and solutions

### Developer Guides
- [API Reference](developer-guide/api-reference.md) - REST API documentation
- [CLI Reference](developer-guide/cli-reference.md) - Command-line interface
- [CRD Reference](developer-guide/crd-reference.md) - Custom Resource Definitions
- [Operator Development](developer-guide/operators.md) - Extending KubeFabric
- [Contributing](developer-guide/contributing.md) - How to contribute

### Examples
- [Job Examples](../examples/jobs/) - Sample job configurations
- [Workflow Examples](../examples/workflows/) - Argo Workflow templates
- [Integration Examples](../examples/integrations/) - JupyterHub, VSCode, Ray
- [Templates](../examples/templates/) - Pre-configured job templates

### Reference
- [Configuration](reference/configuration.md) - All configuration options
- [Metrics](reference/metrics.md) - Prometheus metrics
- [Events](reference/events.md) - Kubernetes events
- [RBAC](reference/rbac.md) - Role-based access control
- [GPU Pricing](reference/gpu-pricing.md) - GPU cost calculation

## Quick Links

### For End Users
- **New to KubeFabric?** Start with the [Quick Start Guide](getting-started/quickstart.md)
- **Submit your first job:** See [Job Submission Guide](user-guide/jobs.md)
- **Track costs:** Check [Cost Tracking Guide](user-guide/costs.md)

### For Administrators
- **Install KubeFabric:** Follow [Installation Guide](getting-started/installation.md)
- **Configure cluster:** See [Cluster Setup](admin-guide/cluster-setup.md)
- **Setup monitoring:** Check [Monitoring Guide](admin-guide/monitoring.md)

### For Developers
- **REST API:** Browse [API Reference](developer-guide/api-reference.md)
- **CLI commands:** See [CLI Reference](developer-guide/cli-reference.md)
- **Extend platform:** Read [Operator Development](developer-guide/operators.md)

## Support

- **Issues:** https://github.com/ssahani/kube-fabric/issues
- **Discussions:** https://github.com/ssahani/kube-fabric/discussions
- **Slack:** Join #kubefabric channel

## Contributing

We welcome contributions! See [Contributing Guide](developer-guide/contributing.md) for details.

## License

Apache License 2.0 - see [LICENSE](../LICENSE) for details.
