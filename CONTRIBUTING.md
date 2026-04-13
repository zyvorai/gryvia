# Contributing to TensorReaper

Thank you for your interest in contributing to TensorReaper!

## Code of Conduct

We pledge to make participation in our project a harassment-free experience for everyone.

## How to Contribute

### Reporting Bugs

Create detailed bug reports with:
- Clear title and description
- Steps to reproduce
- Expected vs actual behavior
- Version information
- Relevant logs

### Pull Requests

1. Fork the repository
2. Create feature branch: `git checkout -b feature/my-feature`
3. Make changes and add tests
4. Commit: `git commit -m "feat: Add feature"`
5. Push: `git push origin feature/my-feature`
6. Create Pull Request

### Commit Message Format

```
<type>: <description>

[optional body]
```

Types: feat, fix, docs, style, refactor, test, chore

## Development Setup

```bash
git clone https://github.com/ssahani/TensorReaper.git
cd tensor-reaper
make build          # Build all operators + CLI + Web UI
make test           # Run all tests
make lint           # Run linters
```

### Prerequisites

- Go 1.22+
- Rust (stable)
- Node.js 20+
- Docker / Podman
- kubectl + helm

## Code Style

- **Go**: `gofmt`, `golangci-lint` (v1.57+), production logging (`Development: false`)
- **Rust**: `rustfmt`, `clippy` with `-D warnings`
- **Python**: `black`, `flake8`, use `config.ConfigException` (not bare `Exception`)
- **TypeScript**: `prettier`, `eslint`, proper types (no `any`)
- **CSS**: Tailwind dark theme (slate palette), no light-theme classes (`bg-white`, `text-gray-*`)
- **Dockerfiles**: Multi-arch support (`TARGETARCH` build arg), non-root user, distroless base
- **Helm**: Resource limits required, security contexts, no `privileged: true`
- **CI**: Pin actions to SHA, add `permissions` block, add `concurrency` group

## Security Guidelines

- Never commit credentials or secrets (use `data:` with base64 placeholders)
- Pin container images to specific versions, never use `:latest`
- Use `hmac.compare_digest()` for secret comparison in Python
- Validate all user input at system boundaries
- Set `allowPrivilegeEscalation: false` and drop ALL capabilities

## Testing

- Write unit tests for new code
- Run `make test` before submitting PR
- Add integration tests for features
- Ensure Makefile targets use subshells: `(cd dir && cmd)` not `cd dir && cmd && cd ..`

## Documentation

- Update docs for new features
- Keep README.md architecture section current
- Include examples in `examples/` directory

## Community

- GitHub Issues: Bug reports
- GitHub Discussions: Questions
- Documentation: https://github.com/ssahani/TensorReaper/docs

Thank you for contributing! 🚀
