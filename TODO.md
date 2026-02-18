## Tests

## Features
### threshold since last back up
backing up the same file twice - if the previous "stage" was less the
X minutes, remove the old operation and add new. This should be
configurable as `daemon.file_change_threshold`
Note: this should only be enforced for files being staged by the
daemon. manual call with with `bt add x` should not enforce this.

bt log

bt restore
 - include implementation of .btignore + ignore in config

bt history

bt add -a -r

logs
 - logs for everything to local
 - operation logs in table

encryption?

## Infra
for sqlite
- enable WAL mode
- use process co-ordination with a lock file

pre-commit format and lint check

github actions to run tests, build

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



