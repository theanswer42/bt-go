# CLAUDE.md

## Project Overview

`bt` is a personal backup CLI tool. Users select directories across multiple machines; files are staged and backed up to a remote vault with full version history and automatic content deduplication via SHA-256 content-addressable storage.

Full design specification (CLI commands, method pseudocode, risks, future plans): see `DESIGN.md` in the project root. Read it before implementing any specific command or service method.

## Architecture

### Component Map

```
CLI (cmd/bt/)
  └── BtService              # orchestration layer — the only component that coordinates across services
        ├── Database          # SQLite wrapper; owns all metadata (directories, files, snapshots)
        ├── Vault             # pluggable remote storage; implementations: FileSystemVault, S3Vault
        ├── StagingArea       # holds files queued between `bt add` and `bt backup`
        └── FilesystemManager # abstracts local fs ops: path resolution, stat, file discovery
```

BtService is the only component that knows about the others. Everything below it is self-contained and testable in isolation. Per the interface rule below, the interfaces for Database, Vault, StagingArea, and FilesystemManager are defined *in the `bt` package* (where BtService consumes them), not in the packages that implement them.

### Data Model

- **Content** — id is the SHA-256 checksum of the file data (not a UUID). This is what makes it content-addressable. Deduplication is automatic: writing the same content twice is idempotent.
- **Directory** — a tracked directory on the local host. UUID id, absolute path.
- **File** — a file within a tracked directory. Tracks its current snapshot.
- **FileSnapshot** — the state of a file at a point in time: content checksum, size, and full stat metadata (permissions, uid, gid, atime, mtime, ctime, birthtime).

These types are shared across packages and will live in `internal/model/`.

### Design Principles

These constrain every implementation decision. When in doubt, re-read these before deciding.

1. **Vault is an interface.** Business logic never reaches for S3 or the filesystem directly. FileSystemVault exists so we can test without a real remote backend.
2. **Content is addressed by its checksum.** `Content.id = SHA-256(data)`. Writing duplicate content is a no-op at the vault level. This is the entire deduplication mechanism.
3. **Paths, not buffers.** Vault and filesystem methods take file paths, not `[]byte`. Large files stay off the heap.
4. **One SQLite DB per host.** The whole DB gets uploaded to the vault after each backup. Simple, correct for personal use. Hosts are fully independent — no cross-host metadata.
5. **Single-user, single-host scope.** No locking, no conflict resolution beyond idempotent content writes.

## Go Tooling & Setup

- **Go version:** 1.22+ (latest stable)
- **Formatter:** `gofmt` — all code must be formatted before committing
- **Linting:** `go vet ./...` at minimum; `golangci-lint` if installed
- **Testing:** `go test -race ./...` — always run with the race detector
- **Build check:** `go build ./...` after every implementation cycle

## Project Structure

```
.
├── cmd/
│   └── bt/
│       └── main.go              # Entry point only. Wire deps, call run. No logic here.
├── internal/
│   ├── model/                   # Shared types: Directory, File, FileSnapshot, Content
│   ├── bt/                      # BtService + interfaces it consumes (Vault, Database, StagingArea, FilesystemManager)
│   ├── config/                  # Config types and ConfigManager (reads/writes ~/.config/bt.toml)
│   ├── vault/                   # Vault implementations: FileSystemVault, S3Vault
│   ├── database/                # SQLite-backed Database implementation
│   ├── staging/                 # StagingArea implementation
│   └── fs/                      # FilesystemManager implementation
├── DESIGN.md                    # Full design spec — read before implementing any command or service
├── go.mod
├── go.sum
└── CLAUDE.md
```

Key rules:
- `cmd/` contains only entry points. All real logic lives in `internal/`.
- Packages are organized by domain capability, not by layer (not `repository/`, `handler/`, `service/` at the top level).
- Test files are colocated with source files, never in a separate `test/` directory.

## Idiomatic Go Conventions

**Naming**
- Short, scoped variable names are idiomatic: `i`, `n`, `err`, `ok`. Use longer names only when the scope is large and ambiguity is a real risk.
- Exported names get a doc comment that starts with the name: `// Server handles HTTP requests.`
- Avoid stuttering: `config.Config` is bad. `config.Options` or just `config.Config` only if the package name doesn't already convey the meaning.
- Interface names: single-method interfaces get an `-er` suffix (`Reader`, `Sender`, `Notifier`). Multi-method interfaces get a descriptive noun.

**Control Flow**
- Guard clauses first — handle errors and early returns at the top, not the bottom.
- `if err != nil` immediately after the call that produced the error. Never batch error checks.
- Prefer `switch` over long `if/else if` chains.

**Errors**
- Always wrap with context: `fmt.Errorf("fetching user %d: %w", id, err)`
- Define sentinel errors at package level for expected conditions: `var ErrNotFound = errors.New("not found")`
- Use custom error types only when callers need to programmatically inspect error details.
- Never silently swallow an error. If you truly must ignore one, add a comment explaining why.

**Interfaces & Composition**
- Define interfaces in the package that *uses* them, not the package that implements them. This is the single most important rule for testability in Go.
- Keep interfaces small. A 1–2 method interface is almost always the right size.
- Compile-time interface checks for your concrete types:
  ```go
  var _ MyInterface = (*MyStruct)(nil)
  ```
- `context.Context` is always the first parameter when a function needs cancellation or deadlines.

**Concurrency**
- Prefer channels over mutexes for communication between goroutines.
- Use `sync.WaitGroup` for fan-out/fan-in patterns.
- Document which struct fields are safe for concurrent access and which are not.

## Testability Patterns

These are non-negotiable for this project:

1. **Interfaces at the boundary.** Any external dependency (database, HTTP client, filesystem, clock, random source) must be behind an interface. The concrete implementation is injected via a constructor.

2. **Constructor injection.** All dependencies are passed through `New<Type>(deps...)` constructors. No `init()` functions that set up global state. No package-level mutable variables.

3. **Table-driven tests.** All tests use the standard Go pattern:
   ```go
   tests := []struct {
       name     string
       input    SomeType
       want     SomeOutput
       wantErr  bool
   }{
       {name: "happy path", ...},
       {name: "missing required field", ...},
   }
   for _, tt := range tests {
       t.Run(tt.name, func(t *testing.T) {
           // ...
       })
   }
   ```

4. **Hand-written mocks.** Keep mocks simple and local to the test file (or a `_test.go` file in the same package). Only reach for `testify/mock` or code generation if mocks become complex. Mocks should be minimal — only implement the methods your test actually exercises.

5. **No time.Now() or rand in business logic.** Inject a clock interface or a source of randomness so tests are deterministic.

6. **Test coverage expectations:**
   - Every new exported function or method must have tests.
   - Tests must cover: happy path, each distinct error case, and boundary/edge conditions.
   - Infrastructure glue code (main.go, wiring) does not need unit tests.

## Testing Conventions

```go
// File: internal/user/service_test.go
package user

import (
    "testing"
    // ...
)

// mockRepository is a test double for Repository, defined right here in the test file.
type mockRepository struct {
    // fields to control behavior and record calls
}

func (m *mockRepository) FindByID(ctx context.Context, id string) (*User, error) {
    // return what the test needs
}

func TestService_CreateUser(t *testing.T) {
    tests := []struct {
        name    string
        input   CreateUserParams
        setup   func(*mockRepository)
        want    *User
        wantErr error
    }{
        // ... test cases
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            repo := &mockRepository{}
            if tt.setup != nil {
                tt.setup(repo)
            }
            svc := NewService(repo)
            got, err := svc.CreateUser(context.Background(), tt.input)
            // assertions
        })
    }
}
```

- Use `t.Helper()` in any shared test helper function.
- Use subtests (`t.Run`) — they give you targeted execution with `go test -run`.
- Avoid `testify/assert` until you decide you want it. `if got != want { t.Errorf(...) }` is perfectly fine and keeps dependencies minimal.
- Use `t.Parallel()` on subtests when tests are independent and have no shared mutable state.

## Dependency Policy

- **Prefer the standard library.** Only reach for a third-party package if the stdlib solution is genuinely inadequate. When you do add one, add a comment in go.mod explaining why.
- Always run `go mod tidy` after adding or removing dependencies.
- No vendoring unless explicitly needed for air-gapped builds.

## Claude Code Workflow

When I ask you to implement a feature, follow this order:

1. **Define the interface first** — what does this component need to do? Write the interface and its doc comments.
2. **Implement the concrete type** — satisfy the interface with a constructor that accepts dependencies.
3. **Write the tests** — table-driven, covering happy path and error cases. Tests should pass.
4. **Verify** — run `go build ./...` and `go test -race ./...`. Do not move on if either fails.

Other guidelines for working with me:
- Keep changes small and focused. One logical unit per implementation cycle.
- If you see a design decision with meaningful trade-offs, **stop and surface them** before committing to a path. Don't silently pick one.
- If a test fails, fix it before moving on. Never leave a red test.
- When you add a dependency or make an architectural choice, briefly explain why.
