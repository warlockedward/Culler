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

// auditEventsDDL is the third step's statement: the append-only chain table,
// its event_type index, and the three structural triggers. Every statement's
// text must stay byte-identical to the audit_events block in schema.sql, so
// the sqlite_schema rows of a migrated spine hash like the reference spine
// built from the baseline.
const auditEventsDDL = `CREATE TABLE audit_events (
    aggregate_id        TEXT NOT NULL,
    seq                 INTEGER NOT NULL CHECK (seq >= 1),
    event_id            TEXT NOT NULL UNIQUE
                       CHECK (length(event_id) = 64 AND
                              length(CAST(event_id AS BLOB)) = 64 AND
                              event_id NOT GLOB '*[^0-9a-f]*'),
    aggregate_type      TEXT NOT NULL,
    event_type          TEXT NOT NULL,
    serializer_id       TEXT NOT NULL
                       CHECK (serializer_id IN ('shz-canon-v1', 'shz-canon-v0')),
    occurred_at         TEXT NOT NULL
                       CHECK (length(occurred_at) = 27 AND
                              occurred_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z'),
    recorded_at         TEXT NOT NULL
                       CHECK (length(recorded_at) = 27 AND
                              recorded_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z'),
    effective_at        TEXT
                       CHECK (effective_at IS NULL OR
                              (length(effective_at) = 27 AND
                               effective_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z')),
    supersedes_event_id TEXT
                       CHECK (supersedes_event_id IS NULL OR
                              (length(supersedes_event_id) = 64 AND
                               length(CAST(supersedes_event_id AS BLOB)) = 64 AND
                               supersedes_event_id NOT GLOB '*[^0-9a-f]*')),
    causation_id        TEXT,
    correlation_id      TEXT,
    prev_hash           TEXT NOT NULL
                       CHECK (length(prev_hash) = 64 AND
                              length(CAST(prev_hash AS BLOB)) = 64 AND
                              prev_hash NOT GLOB '*[^0-9a-f]*'),
    event_digest        TEXT NOT NULL
                       CHECK (length(event_digest) = 64 AND
                              length(CAST(event_digest AS BLOB)) = 64 AND
                              event_digest NOT GLOB '*[^0-9a-f]*'),
    chain_digest        TEXT NOT NULL
                       CHECK (length(chain_digest) = 64 AND
                              length(CAST(chain_digest AS BLOB)) = 64 AND
                              chain_digest NOT GLOB '*[^0-9a-f]*'),
    payload_family      TEXT NOT NULL,
    payload_json        TEXT NOT NULL CHECK (json_valid(payload_json)),
    PRIMARY KEY (aggregate_id, seq)
);
CREATE INDEX audit_events_event_type ON audit_events (event_type);
CREATE TRIGGER audit_events_no_update BEFORE UPDATE ON audit_events
BEGIN SELECT RAISE(ABORT, 'audit_events is append-only'); END;
CREATE TRIGGER audit_events_no_delete BEFORE DELETE ON audit_events
BEGIN SELECT RAISE(ABORT, 'audit_events is append-only'); END;
CREATE TRIGGER audit_events_seq_prefix BEFORE INSERT ON audit_events
WHEN NEW.seq > 1 AND NOT EXISTS (
    SELECT 1 FROM audit_events
    WHERE aggregate_id = NEW.aggregate_id AND seq = NEW.seq - 1
)
BEGIN SELECT RAISE(ABORT, 'audit_events seq prefix gap'); END;`

// usageObservationsDDL is the fourth step's statements: the raw usage event
// blobs, the derived observation rows, and their append-only discipline.
// Every statement's text must stay byte-identical to the usage blocks in
// schema.sql, so the sqlite_schema rows of a migrated spine hash like the
// reference spine built from the baseline.
const usageObservationsDDL = `CREATE TABLE usage_events (
    request_identity    TEXT NOT NULL,
    identity_origin     TEXT NOT NULL CHECK (identity_origin IN ('provider','local')),
    local_request_id    TEXT NOT NULL,
    provider_request_id TEXT,
    native_event_id     TEXT,
    event_seq           INTEGER NOT NULL CHECK (event_seq >= 1),
    usage_family        TEXT NOT NULL,
    raw_json            TEXT NOT NULL CHECK (json_valid(raw_json)),
    occurred_at         TEXT NOT NULL
                       CHECK (length(occurred_at) = 27 AND
                              occurred_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z'),
    recorded_at         TEXT NOT NULL
                       CHECK (length(recorded_at) = 27 AND
                              recorded_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z'),
    PRIMARY KEY (request_identity, event_seq)
);
CREATE UNIQUE INDEX usage_events_native_event
    ON usage_events (request_identity, native_event_id);
CREATE INDEX usage_events_local ON usage_events (local_request_id);
CREATE TRIGGER usage_events_no_update BEFORE UPDATE ON usage_events
BEGIN SELECT RAISE(ABORT, 'usage_events is append-only'); END;
CREATE TRIGGER usage_events_no_delete BEFORE DELETE ON usage_events
BEGIN SELECT RAISE(ABORT, 'usage_events is append-only'); END;
CREATE TABLE usage_observations (
    observation_id            TEXT NOT NULL UNIQUE
                             CHECK (length(observation_id) = 64 AND
                                    length(CAST(observation_id AS BLOB)) = 64 AND
                                    observation_id NOT GLOB '*[^0-9a-f]*'),
    request_identity          TEXT NOT NULL,
    local_request_id          TEXT NOT NULL,
    provider_request_id       TEXT,
    event_seq                 INTEGER NOT NULL CHECK (event_seq >= 1),
    source_version            INTEGER NOT NULL CHECK (source_version >= 1),
    origin                    TEXT NOT NULL CHECK (origin IN ('provider','local')),
    usage_family              TEXT NOT NULL,
    model_id                  TEXT NOT NULL,
    output_tokens             TEXT NOT NULL,
    input_tokens              TEXT NOT NULL,
    report_kind               TEXT NOT NULL CHECK (report_kind IN ('cumulative','incremental')),
    finality                  TEXT NOT NULL CHECK (finality IN ('interim','final')),
    identity_provability      TEXT NOT NULL CHECK (identity_provability IN ('native','cursor','none')),
    state                     TEXT NOT NULL CHECK (state IN ('reported','unknown','reconciliation')),
    payload_digest            TEXT NOT NULL
                             CHECK (length(payload_digest) = 64 AND
                                    length(CAST(payload_digest AS BLOB)) = 64 AND
                                    payload_digest NOT GLOB '*[^0-9a-f]*'),
    supersedes_observation_id TEXT
                             CHECK (supersedes_observation_id IS NULL OR
                                    (length(supersedes_observation_id) = 64 AND
                                     length(CAST(supersedes_observation_id AS BLOB)) = 64 AND
                                     supersedes_observation_id NOT GLOB '*[^0-9a-f]*')),
    occurred_at               TEXT NOT NULL
                             CHECK (length(occurred_at) = 27 AND
                                    occurred_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z'),
    recorded_at               TEXT NOT NULL
                             CHECK (length(recorded_at) = 27 AND
                                    recorded_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z'),
    PRIMARY KEY (request_identity, event_seq, source_version),
    FOREIGN KEY (request_identity, event_seq) REFERENCES usage_events (request_identity, event_seq)
);
CREATE INDEX usage_observations_local ON usage_observations (local_request_id);
CREATE TRIGGER usage_observations_no_update BEFORE UPDATE ON usage_observations
BEGIN SELECT RAISE(ABORT, 'usage_observations is append-only'); END;
CREATE TRIGGER usage_observations_no_delete BEFORE DELETE ON usage_observations
BEGIN SELECT RAISE(ABORT, 'usage_observations is append-only'); END;`
