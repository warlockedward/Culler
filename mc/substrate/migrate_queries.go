package substrate

// schemaHistoryDDL is the ledger table the first migration step creates on
// spines that predate it. New spines receive the table from the baseline
// schema itself (schema.sql carries the same statement).
const schemaHistoryDDL = `CREATE TABLE schema_history (
    version        INTEGER PRIMARY KEY,
    step_name      TEXT NOT NULL UNIQUE,
    step_checksum  TEXT NOT NULL
                   CHECK (length(step_checksum) = 64 AND step_checksum NOT GLOB '*[^0-9a-f]*'),
    from_schema_id TEXT NOT NULL
                   CHECK (length(from_schema_id) = 64 AND from_schema_id NOT GLOB '*[^0-9a-f]*'),
    to_schema_id   TEXT NOT NULL
                   CHECK (length(to_schema_id) = 64 AND to_schema_id NOT GLOB '*[^0-9a-f]*'),
    applied_at     TEXT NOT NULL
)`
