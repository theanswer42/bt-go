
tests + github actions to run tests

pre-commit format and lint check

for sqlite
- enable WAL mode
- use process co-ordination with a lock file

local execution lock
- use lock file to ensure only one process is able to run anything
  that writes
- can we do something where we define some functions in bt service as
  needing some kind of assurance? wonder what's a good way to do this.

vault management
- restore state for host
- "vault management execution block" for all bt service
  functions. This should ensure the vault metadata is up to date with
  local metadata

Log management
- back up logs to the vault?

code review items
- FindOrCreateFile (sqlite.go) has a check-then-insert without a
  transaction. Safe under single-user, UNIQUE constraint catches
  races, but could wrap in a transaction for correctness.
- SearchDirectoryForPath (sqlite.go) loads all directories into memory
  and filters in Go. Fine at personal scale; could push filtering into
  SQL if the directory count grows.
- time.Now() is called directly in backupFile, CreateDirectory, and
  CreateContent instead of using an injected clock. Limits testability
  for timestamp assertions.

Staging the same file twice - if the previous "stage" was less the X
minutes, remove the old operation and add new. This should be
configurable as `staging.file_change_threshold`
