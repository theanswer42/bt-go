# Backup Tool (bt) - Design Document

## Goals and Non-Goals

### Goals
- Provide a robust personal backup solution for files across multiple computers
- Enable backing up user-selected directories to remote storage
- Support file versioning with ability to view history and restore specific versions
- Use content-addressable storage for automatic deduplication across hosts
- Encrypt content and metadata before storing in the vault (per-directory opt-in for content, always-on for metadata)
- Maintain simplicity while ensuring reliability for real-world personal use

### Non-Goals
- This is not a shared filesystem or collaboration tool
- Not designed for team or organizational use (single user per host)
- Not implementing incremental or differential backups initially (full backups only)
- Not providing real-time synchronization across hosts
- Not a version control system (no branching, merging, needing to
  record each change etc.)

## External Dependencies

### Storage (Vault)
`bt` will use an external storage backend for content and metadata
backups. Each should implement the "Vault" abstraction, which
provides:
- **Content Storage**: Ability to store and retrieve content-addressed objects (identified by content checksum)
  - May be cold storage (e.g., S3 Glacier)
  - Must support idempotent writes (duplicate checksums are safely ignored)
- **Metadata Storage**: Ability to store and retrieve metadata files (SQLite databases, JSON files, etc.)
  - Must be identified by host ID
  - Should support versioning or ability to keep multiple versions

**Initial Implementations:**
- Local filesystem (for testing)
- AWS S3 or S3-compatible storage (primary use case)

### Encryption
- `filippo.io/age` — X25519 key pairs, ChaCha20-Poly1305 symmetric encryption, scrypt KDF for passphrase-protected private keys. Chosen for its simplicity, auditability, and native Go streaming support.

### Host System Requirements
The backup tool requires the following from the host system:
- File system access with ability to read file metadata (permissions, timestamps, etc.)
- Ability to calculate file checksums (SHA-256 or similar)
- SQLite support for local metadata management
- File watcher capabilities (for future daemon implementation)

## Risks and Open Questions

### Known Risks
1. **Large File Handling**: Strategy for handling very large files (e.g., 50GB+ files) is not yet defined
   - Should these be chunked or stored as single objects?
   - Vault implementation detail that needs consideration
   - Impact on memory usage and upload reliability

2. **Metadata Vault Synchronization**: Current design uploads entire SQLite DB to vault after each backup
   - Works for personal use but may become inefficient over time
   - Consider incremental sync strategies in future iterations

3. **Metadata Versioning**: Strategy for keeping multiple versions of metadata in vault is implementation-dependent
   - May rely on cloud provider features (S3 versioning)
   - Recovery scenarios if metadata corruption occurs need consideration

4. **Key Loss**: Loss of the encryption key pair means permanent data loss for all encrypted content. Keys are backed up to the vault as metadata, but if the vault itself is lost, encrypted content is irrecoverable.

5. **Forgotten Passphrase**: The private key is protected by a passphrase (age scrypt). A forgotten passphrase makes all encrypted backups irrecoverable — there is no recovery mechanism by design.

### Open Questions
1. **File Watcher Implementation**: How to efficiently detect and stage changed files using file watchers
   - Integration with daemon process
   - Handling of rapid file changes
   - Resource usage implications

2. **Database Growth**: Long-term strategy for metadata database size management
   - Retention policies for old snapshots
   - Archival strategies for ancient history
   - Acceptable for personal use but should monitor

3. **Concurrent Vault Writes**: If multiple hosts write same content simultaneously
   - Content is idempotent (safe)
   - Need protection mechanisms or just accept last-write-wins for rare edge cases?

## Interface

The backup tool provides a command-line interface (CLI) called `bt`.

### Configuration Management

#### Initialize Configuration
```bash
bt config init
```
Creates configuration file in `~/.config/bt.toml`.
Also prompts for a passphrase and generates an X25519 key pair:
- Public key stored in plaintext at `$BT_BASE_DIR/keys/bt.pub`
- Private key encrypted with passphrase (age scrypt) at `$BT_BASE_DIR/keys/bt.key`

#### View Configuration
```bash
bt config list
```
Displays current configuration settings.

### Vault Management

#### Initialize Vault
```bash
bt config vault init
```
Performs any necessary setup on the vault (e.g., creating bucket structure, verifying access).

### Directory Tracking

#### Track a Directory
```bash
bt dir init [--encrypted]
```
- Must be run within the directory to track.
- Marks this directory for backup (does not perform any actual backup
  operations)
- `--encrypted`: enables encryption for this directory. Files backed
  up from this directory will be encrypted before being stored in the
  vault.

### Backup Operations

#### Stage Files for Backup
```bash
bt add [FILENAME]
```
- Must be called within a tracked directory
- If FILENAME omitted, defaults to `.` (current directory)
- stages files for backup

#### Execute Backup
```bash
bt backup
```
- Host-level command (not scoped to current directory)
- Processes all staged operations
- Can be run manually or by daemon process

### Status and Inspection

#### View Directory Status
```bash
bt dir status
```
Shows current state of files in the current tracked directory:
- Which files have been backed up
- Which files are staged but not yet backed up
- Which files are not tracked
- Can optionally show deleted files

#### View File History
```bash
bt log FILENAME
```
Shows version history for a specific file, including:
- Timestamps of each version
- Content checksums
- Metadata changes
- Which versions are available for restore

### Restore Operations

#### Restore a File
```bash
bt restore FILENAME [OPTIONS]
```
- Must be called within a tracked directory
- Restores file with different name (e.g., `filename.txt.<checksum>`)
- Options allow selecting specific version to restore
- By default restores full metadata (permissions, ownership, timestamps)
- Option to restore content only without metadata

## Data Model

The system uses the following core entities. In the Go implementation,
these types are generated by sqlc and live in `internal/database/sqlc/`.

**Note on Identifiers:** All entity IDs are UUIDs except for
Content.id, which is the checksum (SHA-256 or similar) of the content
itself, enabling content-addressable storage.

### Content
Content:
- id: checksum (SHA-256 or similar) - not a UUID
- created_at: timestamp (for bookkeeping)
- encrypted_content_id: checksum (nullable, FK to another Content record)

The actual content is stored in the configured vault. A content object
should only be created *after* that content has been successfully
stored in the vault.

The ID is the content's own checksum, enabling:
- Content-addressable storage
- Automatic deduplication across all hosts
- Integrity verification

**Encryption indirection:**
When `encrypted_content_id` is set, this Content record is "virtual"
— the vault does not store data under this ID. Instead, the actual
bytes live at the Content record pointed to by `encrypted_content_id`.
When `encrypted_content_id` is null, this Content is "real" — the
vault stores data directly under this ID.

Example: a file with plaintext checksum ABC is encrypted to produce
ciphertext with checksum DEF.
- Content(ID=ABC, encrypted_content_id=DEF) — virtual, for dedup
- Content(ID=DEF, encrypted_content_id=null) — real, stored in vault
- FileSnapshot.ContentID = ABC (always points to plaintext checksum)

Restore follows the chain: snapshot → virtual content (ABC) →
encrypted content (DEF) → vault → decrypt → plaintext.

Schema migration:
```sql
ALTER TABLE content ADD COLUMN encrypted_content_id TEXT REFERENCES content(id);
```

### Directory
Directory:
- id: UUID
- path: absolute path on host
- created_at: timestamp
- encrypted: boolean (default false)

Represents a directory tracked for backup. Created when `bt dir init`
is run. When `encrypted` is true, files in this directory are
encrypted before being stored in the vault.

Schema migration:
```sql
ALTER TABLE directories ADD COLUMN encrypted INTEGER NOT NULL DEFAULT 0;
```

### File

File:
- id: UUID
- name: relative path within directory
- directory_id: UUID (foreign key to Directory)
- current_snapshot_id: UUID (foreign key to current FileSnapshot)
- deleted: boolean flag

Represents a file within a tracked directory.
The `name` is the relative path within the directory.

### FileSnapshot

FileSnapshot:
- id: UUID
- file_id: UUID (foreign key to File)
- content_id: checksum (foreign key to Content)
- created_at: timestamp (when snapshot was created)
- size: file size in bytes
- permissions: file mode/permissions
- uid: user ID
- gid: group ID
- accessed_at: file access time (atime from stat)
- modified_at: file modification time (mtime from stat)
- changed_at: metadata change time (ctime from stat)
- born_at: file creation time (birthtime from stat, if available)

Represents the state of a file at a specific point in time. Each file
has one current snapshot and may have several older snapshots.

### BackupOperation

BackupOperation:
- id: auto-increment integer
- started_at: timestamp
- finished_at: timestamp (nullable)
- operation: string (e.g. "AddDirectory", "BackupAll")
- parameters: string
- status: string ("running", "success", "error")

Tracks each backup operation performed by the tool. The ID also serves
as a metadata version number — when the database is uploaded to the
vault, the latest backup operation ID is stored alongside it.

## System Architecture
This section describes various types, abstractions and interfaces, and
how they all interact.
We will use python-like pseudo-code to describe types and algorithms.

A general note on return values: There's no strict consistency in
return values in the following - how and what is to be returned will
depend on the idiomatic way to do it in the implementation language.

### Config
This is all the config needed for `bt`

```python
@dataclass
class Config:
  host_id: UUID     # generated during `bt config init`

  base_dir: Path    # defaults to `$HOME/data/bt`
  log_dir: Path     # defaults to `$BT_BASE_DIR/log`

  vaults: List[VaultConfig] # Configured vaults

  encryption_config: EncryptionConfig

  dbconfig: DatabaseConfig
  stage_config: StagingConfig
  fsmgr_config: FsManagerConfig


@dataclass
class EncryptionConfig:
  public_key_path: Path   # defaults to $BT_BASE_DIR/keys/bt.pub
  private_key_path: Path  # defaults to $BT_BASE_DIR/keys/bt.key (passphrase-encrypted)

@dataclass
class DatabaseConfig:
  host_id: UUID
  data_dir: Path    # defaults to `$BT_BASE_DIR/data`

@dataclass
class StagingConfig:
  host_id: UUID
  staging_dir: Path # defaults to `$BT_BASE_DIR/staging`

@dataclass
class FsManagerConfig:
  host_id: UUID
  ignore_list: List[str] # global list of file patterns to ignore

```

### ConfigManager
Handles reading from and writing to the config file,
`$HOME/.config/bt.toml`
```python
class ConfigManager:
  def init(self, config_path="$HOME/.config/bt.toml"): ...
  def write_config(self, Config): ...
  def read_config(self) -> Config: ...
```

### VaultConfig
```python
@dataclass
class VaultConfig:
  host_id: UUID
  name: str

```

**VaultConfig Implementations:**

```python
@dataclass
class S3VaultConfig(VaultConfig):
  host_id: UUID
  name: str
  content_bucket: str
  content_bucket_prefix: str
  content_bucket_key: str
  metadata_bucket: str
  metadata_bucket_prefix: str
  metadata_bucket_key: str
```

```python
@dataclass
class FileSystemVaultConfig(VaultConfig):
  host_id: UUID
  vault_root: Path
```

### Vault

```python
class Vault:
    """
    Abstract interface for backup storage.
    All methods return Result indicating success/failure.
    File operations use paths to avoid loading large files into memory.
    """

    def __init__(self, config: VaultConfig):...
    def put_content(self, checksum: str, source_path: Path) -> bool:...
    def get_content(self, checksum: str, output_path: Path) -> bool:...
    def put_metadata(self, name: str, source_path: Path, version: int) -> bool:...
    def get_metadata(self, name: str, output_path: Path) -> bool:...
    def get_metadata_version(self, name: str) -> int:...
    def validate_setup(self) -> bool:...
```

**Vault Implementations:**

```python
class FileSystemVault(Vault):
    def __init__(self, config: FileSystemVaultConfig):...

class S3Vault(Vault):
    def __init__(self, config: S3VaultConfig):...

```

**Metadata naming convention:**
The `name` parameter to metadata methods identifies the metadata item.
Known names:
- `"db"` — the SQLite metadata database (always encrypted before upload)
- `"public_key"` — the age public key (plaintext)
- `"private_key"` — the age private key (passphrase-encrypted)

Key files are stored with a fixed version (immutable after initial setup).

**FileSystemVault metadata layout:**
```
<vault_root>/metadata/<hostID>/<name>          # metadata file
<vault_root>/metadata/<hostID>/<name>.version  # version number
```

### Metadata Store
This wraps around an SQLite3 database.

```python
class Database:
    def __init__(self, config: DatabaseConfig):...
    def find_directory_by_path(self, path: Path) -> Directory: ...
    def search_directory_for_path(self, path: Path) -> Directory: ...
    def find_file_by_path(self, directory: Directory, path: Path) -> File:...
    def find_or_create_file(self, directory: Directory, path: Path) -> File:...

    def create_directory(self, path: Path) -> Directory:
        directories = self.find_directories_by_path_prefix(path)
        # now in a transaction, create a directory, modify files in
        # child directories and then delete child directories

    def find_directories_by_path_prefix(self, path_prefix: Path) -> List[Directory]:...
    def move_files(self, source_dir: Directory, dest_dir: Directory) -> bool:...
    def delete_directory(self, directory: Directory) -> bool:...

    def find_files_by_directory(self, directory: Directory) -> List[File]:...

    def find_file_snapshots_for_file(self, file: File) -> List[FileSnapshot]:...
    def find_file_snapshot_by_checksum(self, file: File, checksum: str) -> FileSnapshot:...

    def create_content(self, checksum: str) -> Content:...
    def find_content_by_checksum(self, checksum: str) -> Content:...

    # Atomically records a backup: finds or creates the file record,
    # creates content if needed, and creates a new snapshot + updates
    # the file's current snapshot pointer if anything changed.
    def create_file_snapshot_and_content(self, directory_id: str, relative_path: str, snapshot: FileSnapshot):...

    def create_backup_operation(self, operation: str, parameters: str) -> BackupOperation:...
    def finish_backup_operation(self, id: int, status: str):...
    def list_backup_operations(self, limit: int) -> List[BackupOperation]:...

```

### Staging Area
This stages files to be backed up.

```python
class StagingArea:
    def __init__(self, config: StagingConfig):...
    def stage_for_backup(self, directory: Directory, relative_path: str, source_path: Path) -> bool
    def get_next_staged_operation(self) -> (file: File, file_snapshot: FileSnapshot, staged_file_path: Path):...
    def is_staged(self, file: File) -> bool:...
    def get_staged_files_count(self) -> int:...
```


### Filesystem Manager
This abstracts all filesystem/path related operations

```python
class FilesystemManager:
    def __init__(self, config: FsManagerConfig):...

```

### Encryptor

Handles encryption of content files and unlocking for decryption.
Encryption uses the public key only — no user intervention required.
Decryption requires the user's passphrase to unlock the private key,
which produces a DecryptionContext for the session.

The Encryptor itself is stateless — it never holds an unlocked
private key. All file operations use paths to avoid loading large
files into memory, consistent with the Vault interface pattern. age
supports streaming encryption/decryption natively.

```python
class Encryptor:
    def __init__(self, config: EncryptionConfig): ...

    def setup(self, passphrase: str) -> bool:
        """
        One-time key generation. Called during `bt config init`.
        - Generates X25519 key pair
        - Stores public key in plaintext at config.public_key_path
        - Encrypts private key with passphrase (age scrypt
          passphrase encryption)
        - Stores encrypted private key at config.private_key_path
        - Argon2id parameters should be set to high memory/iteration
          counts. Both keys are stored in the vault, so the KDF is
          the only barrier to offline brute-force by anyone with
          vault access.
        """
        ...

    def encrypt(self, input_path: Path, output_path: Path) -> bool:
        """
        Encrypts file at input_path, writes ciphertext to output_path.
        Uses public key only — no passphrase required.
        Caller is responsible for computing checksum of output_path
        (for the encrypted Content record).
        """
        ...

    def unlock(self, passphrase: str) -> DecryptionContext:
        """
        Unlocks the private key and returns a DecryptionContext.
        Raises if passphrase is incorrect.
        The caller owns the lifecycle of the returned context.
        """
        ...

    def is_configured(self) -> bool:
        """
        Returns True if public and private key files exist at
        configured paths.
        """
        ...
```

### DecryptionContext

Holds an unlocked private key in memory for the duration of a
restore session. Created by `Encryptor.unlock()`. The unlocked key
is held in memory only and never written to disk. The caller
controls the lifecycle of this object.

```python
class DecryptionContext:
    def decrypt(self, input_path: Path, output_path: Path) -> bool:
        """
        Decrypts file at input_path, writes plaintext to output_path.
        """
        ...
```

**Go implementation notes:**
- `age.GenerateX25519Identity()` for key generation
- `age.Encrypt(dst, recipient)` for encryption
- `age.Decrypt(src, identity)` for decryption
- age passphrase-based encryption for private key storage

### BtService
This is the orchestration layer that uses various services to perform
high level operations needed by the CLI.

```python
@dataclass
class FileStatus:
    path: Path
    is_backed_up: bool
    is_staged: bool
    is_modified_since: bool

class BtService:
    def __init__(
        self,
            fsmgr:FilesystemManager,
            staging_area:StagingArea,
            database = Database,
            vaults = [Vault],
            encryptor = Encryptor,
        ):...

    def add_directory(self, path: Path) -> bool:
        path = self.fsmgr.resolve_and_validate(path)
        directory = self.database.find_directory_by_path(path)
        if directory:
            return True
        directory = self.database.create_directory(path)
        return True

    def stage_file(self, path: Path, recursive: bool) -> int:
        path = self.fsmgr.resolve_and_validate(path)
        directory = self.database.find_directory_by_path(path)
        if path.is_directory:
            paths = self.fsmgr.find_files_in_directory(path, recursive)
        else:
            paths = [path]
        for path in paths:
            relative_path = path.relative_to(directory.path)
            self.staging_area.stage_for_backup(directory, relative_path, path)
        return len(paths)

    def get_status(self, path:Path, recursive: bool) -> List[FileStatus]:
        path = self.fsmgr.resolve_and_validate(path)
        directory = self.database.find_directory_by_path(path)
        if not directory:
            return None
        if path.is_directory:
            paths = self.fsmgr.find_files_in_directory(path, recursive)
        else:
            paths = [path]
        return [get_file_status(directory, path) for path in paths]

    def get_file_status(self, directory:Directory, path:Path) -> FileStatus:
        ...

    def get_file_history(self, path:Path) -> List[FileSnapshot]:...

    def restore(self, path:Path, checksum:str) -> List[str]:
        # If path matches a tracked directory, restores all files in
        # that directory (checksum must be empty in that case).
        # Otherwise, restores the single file at the given path.
        # Returns the list of output file paths written.
        ...

    def back_up_staged_file(self, file: File, file_snapshot: FileSnapshot, staged_file_path: Path) -> bool:
        """
        Backup flow for encrypted directories:
        1. Staging produces plaintext file with checksum ABC.
        2. Check if Content(ABC) already exists — if so, skip (dedup).
        3. Encrypt staged file → temp file, compute checksum DEF.
        4. Create Content(ID=DEF) — real, stored in vault.
        5. Create Content(ID=ABC, encrypted_content_id=DEF) — virtual.
        6. vault.put_content(DEF, encrypted_temp_file).
        7. FileSnapshot.ContentID = ABC.

        For unencrypted directories, the existing flow is unchanged:
        content is stored directly under its plaintext checksum.
        """
        ...

    def back_up_all_staged_files(self) -> int:...

    def restore_file(self, file: File, snapshot: FileSnapshot, output_path: Path, decryption_ctx: DecryptionContext = None) -> bool:
        """
        Restore flow for encrypted content:
        1. Look up Content(ABC) → encrypted_content_id = DEF.
        2. vault.get_content(DEF) → encrypted temp file.
        3. decryption_ctx.decrypt(temp, output_path).

        For unencrypted content (encrypted_content_id is null):
        1. vault.get_content(ABC) → output_path directly.

        The CLI must prompt for passphrase once before a restore
        session and pass the DecryptionContext to all restore calls.
        """
        ...

    def backup_metadata(self) -> bool:
        """
        Metadata backup flow:
        1. Encrypt DB file using encryptor.encrypt.
        2. vault.put_metadata("db", encrypted_db_path, version).
        3. vault.put_metadata("public_key", pubkey_path, fixed_version).
        4. vault.put_metadata("private_key", enckey_path, fixed_version).

        Keys are stored with a fixed version (immutable after setup).
        The DB is always encrypted before upload, regardless of
        whether any directories use encryption.
        """
        ...

    def restore_metadata(self, passphrase: str) -> bool:
        """
        Metadata restore flow (new host setup):
        1. vault.get_metadata("public_key", ...) and
           vault.get_metadata("private_key", ...).
        2. Prompt for passphrase, unlock private key via
           encryptor.unlock(passphrase).
        3. vault.get_metadata("db", ...) → encrypted DB file.
        4. Decrypt DB → usable metadata.
        """
        ...
```

### Multi-Host Coordination

**Content Sharing:**
- Multiple hosts can back up to same vault
- Content deduplication happens automatically (shared checksums)
- Each host has independent metadata store
- Database identified by unique host_id
- No cross-host metadata visibility through this tool

**Host Identification:**
- host_id auto-generated on first run (UUID)
- Stored in local configuration

### Future Considerations

**Key Rotation:**
- Mechanism to re-encrypt content with a new key pair
- Requires decrypting all encrypted content and re-encrypting
- Needs careful handling of the transition period

**File Watching:**
- Daemon could use inotify/FSEvents to detect changes
- Automatically run staging for changed files
- Requires careful handling of rapid changes

