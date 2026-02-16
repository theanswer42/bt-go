
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
