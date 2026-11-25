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

// renameHealthAlertBodySQL is the second step's statement. Its CREATE TRIGGER
// text must stay byte-identical to the trigger definition in schema.sql, so
// the sqlite_schema row of a migrated spine hashes like the reference spine
// built from the baseline.
const renameHealthAlertBodySQL = `DROP TRIGGER activity_health_alert_outbox;
CREATE TRIGGER activity_health_alert_outbox
AFTER INSERT ON activity
WHEN NEW.kind = 'dispatch.health'
BEGIN
    SELECT CASE WHEN NOT EXISTS (
        SELECT 1 FROM alert_routes WHERE class = 'health'
    ) THEN RAISE(ABORT, 'health alert routing is not configured') END;
    INSERT INTO outbox (kind, surface, channel_ref, payload)
    SELECT 'health', surface, channel_ref,
           json_object(
               'activity_id', NEW.id,
               'subject', NEW.subject,
               'detail', NEW.detail,
               'body', 'S-Horizon health: ' || NEW.detail
           )
    FROM alert_routes
    WHERE class = 'health'
    ORDER BY ordinal;
END;`
