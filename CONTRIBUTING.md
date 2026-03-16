# Contributing to GoPackX

## Development

### Prerequisites

- Go 1.22 or later

### Running tests

```bash
go test ./...
```

### Running benchmarks

```bash
go test ./pkg/solver/ -bench=. -benchmem
```

### Checking coverage

```bash
go test $(go list ./... | grep -v examples) -coverprofile=cover.out
go tool cover -func=cover.out | tail -1
```

## Style Guide

This project follows the [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md). Key conventions:

- Use `strconv` over `fmt` for conversions
- Use `map[T]struct{}` for sets
- Omit zero-value fields in struct literals
- Don't shadow builtins
- Verify interface compliance with `var _ Interface = (*Type)(nil)`
- Functional options use the `func(*T)` pattern

## Releasing a New Version

GoPackX uses semantic versioning. Tags go directly on `main` — this is the standard approach for Go libraries since `go get` resolves tags from the default branch.

### Steps

1. Ensure all tests pass and coverage is acceptable:

```bash
go test ./...
```

2. Push any pending commits to main:

```bash
git push origin main
```

3. Create and push the tag:

```bash
git tag v0.X.0
git push origin v0.X.0
```

4. Create the GitHub Release with changelog:

```bash
gh release create v0.X.0 --title "v0.X.0" --notes "$(cat <<'EOF'
## What's New

- Feature 1
- Feature 2

## Breaking Changes

- (if any)
EOF
)"
```

The Go module proxy indexes the new version automatically after the tag is pushed.

### Version Numbering

| Change | Version bump | Example |
|---|---|---|
| New features, backward compatible | Minor | v0.1.0 → v0.2.0 |
| Bug fixes only | Patch | v0.2.0 → v0.2.1 |
| Breaking API changes | Major | v0.x → v1.0.0 |

While in v0.x, minor versions may include breaking changes. After v1.0.0, the Go module compatibility guarantee applies.
