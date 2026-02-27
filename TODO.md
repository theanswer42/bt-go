# TODO Items, in order of importance

## Code review - file access time usage for checking if file changed
- **Issue: Snapshot Sensitivity to `atime`**. `snapshotsEqual` in `internal/database/sqlite.go` compares `AccessedAt`, which changes on file reads. This leads to redundant snapshots and backups.
  - **Suggested Fix**: Update `snapshotsEqual` to exclude `AccessedAt` from the comparison. Focus on `ModifiedAt`, `Size`, and `ContentID`.
- **Issue: Staging Queue Scalability**. `filesystemStore` uses a single `queue.json` file for the staging queue, which will become a bottleneck and performance risk as the number of staged items grows.
  - **Suggested Fix**: Transition the staging queue to a more scalable format, such as an SQLite table or a line-delimited JSON (JSONL) file.

## Code review Argon2id vs scrypt
- **Issue: KDF Discrepancy**. `DESIGN.md` specifies `Argon2id` for private key protection, but `internal/encryption/age.go` uses `scrypt` (the `age` default).
  - **Suggested Fix**: Either update `DESIGN.md` to reflect the use of `scrypt` or implement an `Argon2id` wrapper for the private key before passing it to `age`.

## S3Vault
Spec and build the S3Vault

# Manual testing work

## Full test with FSVault
Build a suite of sorts of manual integration tests.

## Full test with S3Vault

## bt config init


# Future

## Daemon design
Let's create a design for how to implement the bt-daemon. Include:
- proper signal handling
- log file rotation
- debian user level service setup

## CD - ie, build installable binary

## Save operation logs in the operation
It might be handy to save operation logs in a text field in the
operations table.

## backing up the same file multiple times
When staging a file, what do we do if this file is already staged?
What about if it was very recently backed up?
This probably needs separate consideration for if running manually or
if being done by a daemon. if the previous "stage" was less the
X minutes, remove the old operation and add new. This should be
configurable as `daemon.file_change_threshold`
Note: this should only be enforced for files being staged by the
daemon. manual call with with `bt add x` should not enforce this.

## SQLite configuration
- enable WAL mode

## Protect the SQLite database
- use process co-ordination with a lock file

## Code review
- FindOrCreateFile (sqlite.go) has a check-then-insert without a
  transaction. Safe under single-user, UNIQUE constraint catches
  races, but could wrap in a transaction for correctness.
- SearchDirectoryForPath (sqlite.go) loads all directories into memory
  and filters in Go. Fine at personal scale; could push filtering into
  SQL if the directory count grows.
- time.Now() is called directly in backupFile, CreateDirectory, and
  CreateContent instead of using an injected clock. Limits testability
  for timestamp assertions.



