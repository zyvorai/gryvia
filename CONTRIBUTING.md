# Contributing to KubeFabric

Thank you for your interest in contributing to KubeFabric! This document provides guidelines and instructions for contributing.

## Code of Conduct

Be respectful, inclusive, and professional in all interactions.

## Getting Started

### Development Setup

1. **Fork and clone the repository**
   ```bash
   git clone https://github.com/YOUR_USERNAME/kube-fabric.git
   cd kube-fabric
   ```

2. **Set up development environment**
   ```bash
   make dev-setup
   ```

   This installs:
   - Go 1.22+ dependencies
   - Rust toolchain
   - Node.js dependencies
   - Python dependencies

3. **Create a development cluster**
   ```bash
   ./scripts/dev-environment.sh
   ```

   This creates a kind cluster with simulated GPU nodes.

## Development Workflow

### 1. Create a Feature Branch

```bash
git checkout -b feature/your-feature-name
```

### 2. Make Changes

Follow the coding standards for each language:

**Go (Operators)**
- Follow [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Run `gofmt` and `golangci-lint`
- Write tests for new functionality
- Update CRD types if needed

**Rust (CLI)**
- Follow [Rust API Guidelines](https://rust-lang.github.io/api-guidelines/)
- Run `cargo fmt` and `cargo clippy`
- Write unit tests
- Update CLI documentation

**TypeScript/React (Web UI)**
- Follow React best practices
- Use functional components and hooks
- Run `npm run lint`
- Write component tests

**Python (API Gateway)**
- Follow PEP 8 style guide
- Use type hints
- Run `black` and `flake8`
- Write pytest tests

### 3. Test Your Changes

```bash
# Run all tests
make test

# Test specific component
cd operators/gpu-operator && go test ./...
cd cli && cargo test
cd web-ui && npm test
cd services/api-gateway && pytest
```

### 4. Build and Test Locally

```bash
# Build all components
make build

# Build Docker images
make docker-build

# Deploy to dev cluster
make deploy
```

### 5. Format Code

```bash
make fmt
```

### 6. Commit Changes

Use conventional commit format:

```
<type>(<scope>): <subject>

<body>

<footer>
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation
- `style`: Formatting
- `refactor`: Code restructuring
- `test`: Tests
- `chore`: Maintenance

Examples:
```bash
git commit -m "feat(gpu-operator): add support for H200 GPUs"
git commit -m "fix(web-ui): correct quota calculation in dashboard"
git commit -m "docs: update installation guide"
```

### 7. Push and Create Pull Request

```bash
git push origin feature/your-feature-name
```

Then create a PR on GitHub with:
- Clear description of changes
- Link to related issues
- Screenshots for UI changes
- Test results

## Pull Request Process

1. **Automated Checks**: CI will run tests, linting, and builds
2. **Code Review**: Maintainers will review your code
3. **Updates**: Address review feedback
4. **Merge**: Approved PRs are merged by maintainers

## Component-Specific Guidelines

### Operators (Go)

**File Structure:**
```
operators/<name>-operator/
├── main.go
├── api/v1/<crd>_types.go
├── controllers/<crd>_controller.go
├── pkg/                  # Shared packages
├── config/               # K8s manifests
└── README.md
```

**Testing:**
- Unit tests for business logic
- Integration tests with envtest
- E2E tests optional

**Reconciliation:**
- Idempotent reconciliation logic
- Proper error handling and retry
- Status updates with conditions
- Metrics and events

### CLI (Rust)

**File Structure:**
```
cli/
├── src/
│   ├── main.rs
│   ├── client.rs         # K8s client
│   ├── types.rs          # CRD types
│   ├── display.rs        # Output formatting
│   └── commands/         # Command implementations
└── Cargo.toml
```

**User Experience:**
- Clear error messages
- Progress indicators for long operations
- Colored output with `--no-color` option
- JSON/YAML output formats

### Web UI (React/TypeScript)

**File Structure:**
```
web-ui/src/
├── components/           # Reusable components
├── pages/                # Page components
├── lib/                  # Utilities
└── types/                # Type definitions
```

**Best Practices:**
- Component composition over inheritance
- Hooks for state and effects
- React Query for data fetching
- TailwindCSS for styling
- Accessibility (a11y) compliance

### API Gateway (Python/FastAPI)

**File Structure:**
```
services/api-gateway/
├── main.py              # FastAPI app
├── requirements.txt
└── tests/
```

**Best Practices:**
- Type hints for all functions
- Pydantic models for validation
- Async endpoints where possible
- OpenAPI documentation
- Error handling with proper HTTP codes

## Adding New Features

### New Operator

1. Create operator directory structure
2. Define CRD in `crds/`
3. Implement controller logic
4. Add RBAC manifests
5. Write tests
6. Update documentation
7. Add example YAMLs

### New CLI Command

1. Add command in `cli/src/commands/`
2. Register in `main.rs`
3. Implement K8s API calls
4. Add output formatting
5. Write tests
6. Update CLI README

### New Web UI Page

1. Create page component in `web-ui/src/pages/`
2. Add route in `App.tsx`
3. Create API client functions
4. Implement data fetching
5. Style with TailwindCSS
6. Test responsiveness

## Testing

### Unit Tests

```bash
# Go
go test ./...

# Rust
cargo test

# Python
pytest
```

### Integration Tests

```bash
# Deploy to test cluster
make deploy

# Run integration tests
make test-integration
```

### E2E Tests

```bash
# Full E2E test suite
./scripts/e2e-test.sh
```

## Documentation

Update documentation for:
- New features
- API changes
- Configuration options
- Breaking changes

Documentation locations:
- `README.md` - Project overview
- Component READMEs - Specific docs
- `docs/` - Detailed guides
- Code comments - Implementation details

## Release Process

Maintainers handle releases:

1. Update version numbers
2. Update CHANGELOG
3. Tag release: `git tag -a v1.2.3 -m "Release v1.2.3"`
4. Push tag: `git push origin v1.2.3`
5. CI builds and publishes artifacts

## Getting Help

- **Questions**: GitHub Discussions
- **Bugs**: GitHub Issues
- **Chat**: (Add Slack/Discord link)
- **Email**: (Add email)

## Recognition

Contributors are recognized in:
- CONTRIBUTORS file
- Release notes
- Project README

## License

By contributing, you agree that your contributions will be licensed under the Apache License 2.0.

Thank you for contributing to KubeFabric! 🚀
