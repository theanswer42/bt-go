-- Add encryption support: encrypted content indirection and per-directory encryption flag.

-- Nullable FK to another content record. When set, this content is "virtual" —
-- the vault stores data under the referenced content ID (the encrypted checksum).
ALTER TABLE contents ADD COLUMN encrypted_content_id TEXT REFERENCES contents(id);

-- Per-directory encryption opt-in. When true, files in this directory are
-- encrypted before being stored in the vault.
ALTER TABLE directories ADD COLUMN encrypted INTEGER NOT NULL DEFAULT 0;
