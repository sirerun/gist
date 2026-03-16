# Contributing to Gist

Thank you for your interest in contributing to Gist. This guide covers development setup, coding standards, and the pull request process.

## Development Setup

```bash
git clone https://github.com/sirerun/gist.git
cd gist
go build ./...
go test ./... -race
```

### Prerequisites

- Go 1.23+
- golangci-lint v2+ (`go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest`)
- PostgreSQL 16+ (optional, for integration tests)

### Quality Gates

Every change must pass all gates before merge:

```bash
go build ./...                          # Build
go vet ./...                            # Vet
go test ./... -race -timeout 120s       # Test (with race detector)
golangci-lint run ./...                 # Lint (zero findings required)
```

CI runs these automatically on every pull request.

### Running Integration Tests

Integration tests require a PostgreSQL instance with the `pg_trgm` extension:

```bash
# Start PostgreSQL (e.g., via Docker)
docker run -d --name gist-pg -p 5432:5432 \
  -e POSTGRES_PASSWORD=testpass -e POSTGRES_DB=gist_test postgres:16

# Enable pg_trgm
PGPASSWORD=testpass psql -h localhost -U postgres -d gist_test \
  -c "CREATE EXTENSION IF NOT EXISTS pg_trgm;"

# Run all tests including integration
GIST_TEST_DSN="postgres://postgres:testpass@localhost:5432/gist_test?sslmode=disable" \
  go test ./... -race -tags=integration -timeout 120s
```

Without `GIST_TEST_DSN`, only unit tests run.

## Making Changes

1. Fork and clone the repository.
2. Create a feature branch from `main`.
3. Write tests first, then implement.
4. Ensure all quality gates pass locally.
5. Submit a pull request against `main`.

### Branch Naming

Use descriptive branch names:

```
feat/batch-index-progress
fix/trigram-search-edge-case
docs/update-mcp-examples
```

## Code Style

### Go Conventions

- Prefer the Go standard library over third-party packages.
- Use interface segregation — define narrow interfaces where they're consumed.
- Keep exported APIs minimal. Only export what other packages need.
- Use `context.Context` as the first parameter for functions that do I/O.

### Testing

- Write table-driven tests using the `testing` package.
- Do **not** add testify or other test assertion libraries.
- Run tests with `-race` to catch data races.

```go
func TestMyFunction(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {name: "valid input", input: "hello", want: "HELLO"},
        {name: "empty input", input: "", wantErr: true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := MyFunction(tt.input)
            if (err != nil) != tt.wantErr {
                t.Fatalf("error = %v, wantErr = %v", err, tt.wantErr)
            }
            if got != tt.want {
                t.Errorf("got %q, want %q", got, tt.want)
            }
        })
    }
}
```

### Error Handling

- Use sentinel errors (`var ErrNotFound = errors.New(...)`) for expected conditions.
- Wrap errors with context: `fmt.Errorf("indexing source %s: %w", label, err)`.
- Check error return values from all function calls.

## Commit Messages

Use [Conventional Commits](https://www.conventionalcommits.org/). Releases are automated via [release-please](https://github.com/googleapis/release-please) — commit message prefixes determine the version bump:

| Prefix | Version Bump | Example |
|--------|-------------|---------|
| `feat:` | Minor (0.x.0) | `feat: add YAML chunking support` |
| `fix:` | Patch (0.0.x) | `fix: handle empty documents in batch index` |
| `feat!:` or `BREAKING CHANGE:` | Major (x.0.0) | `feat!: remove deprecated WithStore option` |
| `docs:`, `test:`, `chore:` | No release | `docs: update search examples` |

Scope is optional but encouraged. Use the package or feature area: `search`, `chunk`, `mcp`, `cli`, `store`, `ci`.

## Pull Requests

### PR Requirements

- All quality gates pass (CI enforces this).
- New code has tests.
- No secrets, credentials, or `.env` files committed.
- PR description explains **what** changed and **why**.

### Review Process

- PRs are rebased and merged (no squash, no merge commits).
- At least one approval required before merge.
- Address review feedback with new commits (don't force-push during review).

## Getting Help

- Open an issue at [github.com/sirerun/gist/issues](https://github.com/sirerun/gist/issues)

## License

By contributing, you agree that your contributions will be licensed under the Apache License 2.0.
