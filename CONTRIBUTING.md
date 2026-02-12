# Contributing to KubeFabric

Thank you for your interest in contributing to KubeFabric!

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
git clone https://github.com/ssahani/kube-fabric.git
cd kube-fabric
make install-deps
make build
make test
```

## Code Style

- Go: Use gofmt and golint
- Rust: Use rustfmt and clippy  
- Python: Use black and pylint
- TypeScript: Use prettier and eslint

## Testing

- Write unit tests for new code
- Run `make test` before submitting PR
- Add integration tests for features

## Documentation

- Update docs for new features
- Add code comments
- Include examples

## Community

- GitHub Issues: Bug reports
- GitHub Discussions: Questions
- Documentation: https://github.com/ssahani/kube-fabric/docs

Thank you for contributing! 🚀
