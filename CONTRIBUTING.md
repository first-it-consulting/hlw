# Contributing to hlw

Thanks for your interest in contributing! This document provides guidelines and instructions for contributing.

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/<your-username>/hlw.git`
3. Create a branch: `git checkout -b feature/your-feature-name`
4. Make your changes
5. Run tests: `make test`
6. Commit your changes: `git commit -m "Add your feature"`
7. Push to the branch: `git push origin feature/your-feature-name`
8. Open a Pull Request

## Development Setup

### Prerequisites

- Go 1.27+ (see `go` directive in `go.mod`)
- Make

### Building

```bash
# Build the binary
make build

# Run tests
make test

# Install locally
make install
```

### Code Style

This project follows standard Go coding conventions:
- Use `gofmt` or `go fmt` to format code
- Follow [Effective Go](https://go.dev/doc/effective_go) guidelines
- Write tests for new functionality
- Keep functions small and focused
- Use meaningful variable names

## Pull Request Process

Work happens on a branch; opening a pull request against `main` is what runs CI.

1. Branch off `main` and push your work
2. Open a PR — [`ci.yml`](.github/workflows/ci.yml) runs gofmt, `go vet`, a
   `go mod tidy` check, `go test -race`, and validates `config.example.json`
3. On success it builds a **release candidate** and attaches it to the run as
   `hlw-<version>-rc.<run-number>`, so a reviewer can try the actual binary
4. Update documentation and add tests for new behaviour
5. If the change should ship, bump [`VERSION`](VERSION) in the same PR
6. Merging to `main` runs [`release.yml`](.github/workflows/release.yml): checks
   again, then builds, tags `v<VERSION>` and publishes the release

Merges that do not change `VERSION` are built and tested but publish nothing, so
you can land work without cutting a release.

Run the same checks locally before pushing:

```bash
make check
```

## Reporting Issues

When reporting issues, please include:
- Clear description of the problem
- Steps to reproduce
- Expected vs actual behavior
- Your environment (OS, Go version)
- Relevant logs or screenshots

## Code of Conduct

- Be respectful and inclusive
- Provide constructive feedback
- Focus on what is best for the community
