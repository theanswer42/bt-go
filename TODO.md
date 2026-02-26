# TODO Items, in order of importance

## Encryption
Enable encryption at the directory level. If enabled, everything in
that directory will be encrypted in the vault.

Before we do this, we'll need a general investigation into how to
implement encryption in a "safe" way (not just safe as in secure, but
also safe as in, I should not lose my entire backup because I forgot
something).

## Full test with FSVault
Build a suite of sorts of manual integration tests.

## S3Vault

## Full test with S3Vault

## bt config init


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



