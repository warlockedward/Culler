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

// usageV2DDL is the fifth step's statements: the v2 settlement-consumption
// table (its object definitions byte-identical to the usage block in
// schema.sql) and the blob backfill that promotes each slot's current
// observation. The backfill INSERT is migration-only: a fresh spine has
// nothing to promote, and the step's same-transaction proof (count,
// multiset digest, both anti-join directions) must pass before the step
// commits. The v1 tables are never dropped: the originals survive the
// migration by construction.
const usageV2DDL = `CREATE TABLE usage (
    source_report_id      TEXT NOT NULL UNIQUE
                         CHECK (length(source_report_id) = 64 AND
                                length(CAST(source_report_id AS BLOB)) = 64 AND
                                source_report_id NOT GLOB '*[^0-9a-f]*'),
    request_identity      TEXT NOT NULL,
    local_request_id      TEXT NOT NULL,
    provider_request_id   TEXT,
    event_seq             INTEGER NOT NULL CHECK (event_seq >= 1),
    source_version        INTEGER NOT NULL CHECK (source_version >= 1),
    origin                TEXT NOT NULL CHECK (origin IN ('provider','local')),
    usage_family          TEXT NOT NULL,
    model_id              TEXT NOT NULL,
    output_tokens         TEXT NOT NULL,
    input_tokens          TEXT NOT NULL,
    report_kind           TEXT NOT NULL CHECK (report_kind IN ('cumulative','incremental')),
    finality              TEXT NOT NULL CHECK (finality IN ('interim','final')),
    identity_provability  TEXT NOT NULL CHECK (identity_provability IN ('native','cursor','none')),
    state                 TEXT NOT NULL CHECK (state IN ('reported','unknown','reconciliation')),
    payload_digest        TEXT NOT NULL
                         CHECK (length(payload_digest) = 64 AND
                                length(CAST(payload_digest AS BLOB)) = 64 AND
                                payload_digest NOT GLOB '*[^0-9a-f]*'),
    supersedes_report_id  TEXT
                         CHECK (supersedes_report_id IS NULL OR
                                (length(supersedes_report_id) = 64 AND
                                 length(CAST(supersedes_report_id AS BLOB)) = 64 AND
                                 supersedes_report_id NOT GLOB '*[^0-9a-f]*')),
    occurred_at           TEXT NOT NULL
                         CHECK (length(occurred_at) = 27 AND
                                occurred_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9].[0-9][0-9][0-9][0-9][0-9][0-9]Z'),
    PRIMARY KEY (source_report_id)
);
CREATE INDEX usage_request ON usage (request_identity, event_seq);
CREATE INDEX usage_local ON usage (local_request_id);
INSERT INTO usage (
    source_report_id, request_identity, local_request_id, provider_request_id,
    event_seq, source_version, origin, usage_family, model_id,
    output_tokens, input_tokens, report_kind, finality, identity_provability,
    state, payload_digest, supersedes_report_id, occurred_at
)
SELECT o.observation_id, o.request_identity, o.local_request_id, o.provider_request_id,
    o.event_seq, o.source_version, o.origin, o.usage_family, o.model_id,
    o.output_tokens, o.input_tokens, o.report_kind, o.finality, o.identity_provability,
    o.state, o.payload_digest, o.supersedes_observation_id, o.occurred_at
FROM usage_observations o
WHERE o.source_version = (
    SELECT MAX(o2.source_version) FROM usage_observations o2
    WHERE o2.request_identity = o.request_identity AND o2.event_seq = o.event_seq);`

// identityFoundationDDL is the seventh step's object definitions: the P4
// identity plane (users, sessions, teams, membership, grants), the
// trusted_ctx lifecycle table, and the object-scope guards. Every
// statement's text must stay byte-identical to the identity block in
// schema.sql, so the sqlite_schema rows of a migrated spine hash like the
// reference spine built from the baseline.
const identityFoundationDDL = `CREATE TABLE users (
    id            TEXT NOT NULL UNIQUE
               CHECK (length(id) = 32 AND
                      length(CAST(id AS BLOB)) = 32 AND
                      id NOT GLOB '*[^0-9a-f]*'),
    username      TEXT NOT NULL
               CHECK (length(username) BETWEEN 3 AND 64 AND
                      username NOT GLOB '*[^a-zA-Z0-9._-]*'),
    password_hash TEXT NOT NULL
               CHECK (length(password_hash) = 60 AND
                      password_hash GLOB '$2[ab]$*'),
    created_at    TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (id)
);
CREATE UNIQUE INDEX users_username ON users (username);
CREATE TABLE user_sessions (
    session_id          TEXT NOT NULL UNIQUE
                     CHECK (length(session_id) = 32 AND
                            length(CAST(session_id AS BLOB)) = 32 AND
                            session_id NOT GLOB '*[^0-9a-f]*'),
    token_hash          TEXT NOT NULL UNIQUE
                     CHECK (length(token_hash) = 64 AND
                            length(CAST(token_hash AS BLOB)) = 64 AND
                            token_hash NOT GLOB '*[^0-9a-f]*'),
    user_id             TEXT NOT NULL REFERENCES users (id),
    state               TEXT NOT NULL CHECK (state IN ('active','revoked','expired')),
    created_at          TEXT NOT NULL DEFAULT (datetime('now')),
    absolute_expires_at TEXT NOT NULL,
    idle_expires_at     TEXT NOT NULL,
    last_seen_at        TEXT NOT NULL,
    revoked_by          TEXT
                     CHECK (revoked_by IS NULL OR
                            (length(revoked_by) = 32 AND
                             length(CAST(revoked_by AS BLOB)) = 32 AND
                             revoked_by NOT GLOB '*[^0-9a-f]*')),
    PRIMARY KEY (session_id)
);
CREATE INDEX user_sessions_user ON user_sessions (user_id);
CREATE TABLE teams (
    id         TEXT NOT NULL UNIQUE
            CHECK (length(id) = 32 AND
                   length(CAST(id AS BLOB)) = 32 AND
                   id NOT GLOB '*[^0-9a-f]*'),
    name       TEXT NOT NULL
            CHECK (length(name) BETWEEN 2 AND 64 AND
                   name NOT GLOB '*[^a-zA-Z0-9._-]*'),
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (id)
);
CREATE UNIQUE INDEX teams_name ON teams (name);
CREATE TABLE team_members (
    team_id   TEXT NOT NULL REFERENCES teams (id),
    user_id   TEXT NOT NULL REFERENCES users (id),
    joined_at TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (team_id, user_id)
);
CREATE TABLE grants (
    grant_id           TEXT NOT NULL UNIQUE
                    CHECK (length(grant_id) = 32 AND
                           length(CAST(grant_id AS BLOB)) = 32 AND
                           grant_id NOT GLOB '*[^0-9a-f]*'),
    grantee_user_id    TEXT NOT NULL REFERENCES users (id),
    scope              TEXT NOT NULL
                    CHECK (length(scope) BETWEEN 1 AND 128),
    resource           TEXT
                    CHECK (resource IS NULL OR length(resource) <= 256),
    granted_by_user_id TEXT NOT NULL REFERENCES users (id),
    created_at         TEXT NOT NULL DEFAULT (datetime('now')),
    effective_at       TEXT NOT NULL,
    expires_at         TEXT NOT NULL,
    revoked_at         TEXT,
    PRIMARY KEY (grant_id)
);
CREATE INDEX grants_grantee ON grants (grantee_user_id, scope);
CREATE TABLE trusted_ctx (
    actor TEXT PRIMARY KEY
        CHECK (length(actor) = 32 AND
               length(CAST(actor AS BLOB)) = 32 AND
               actor NOT GLOB '*[^0-9a-f]*'),
    admin INTEGER NOT NULL DEFAULT 0
);
CREATE TRIGGER grants_guarded_insert BEFORE INSERT ON grants
BEGIN
    SELECT RAISE(ABORT, 'grant insert requires a guard-bound trusted context')
    WHERE NOT EXISTS (SELECT 1 FROM trusted_ctx);
    SELECT RAISE(ABORT, 'grant attribution disagrees with the trusted context')
    WHERE NEW.granted_by_user_id NOT IN (SELECT actor FROM trusted_ctx);
END;
CREATE TRIGGER user_sessions_guarded_insert BEFORE INSERT ON user_sessions
BEGIN
    SELECT RAISE(ABORT, 'session insert requires a guard-bound trusted context')
    WHERE NOT EXISTS (SELECT 1 FROM trusted_ctx);
    SELECT RAISE(ABORT, 'session ownership disagrees with the trusted context')
    WHERE NEW.user_id NOT IN (SELECT actor FROM trusted_ctx);
END;
CREATE TRIGGER user_sessions_guarded_revoke BEFORE UPDATE OF state ON user_sessions
WHEN NEW.state IN ('revoked','expired') AND OLD.state = 'active'
BEGIN
    SELECT RAISE(ABORT, 'session state change requires a guard-bound trusted context')
    WHERE NOT EXISTS (SELECT 1 FROM trusted_ctx);
    SELECT RAISE(ABORT, 'session state actor disagrees with the trusted context')
    WHERE NEW.user_id NOT IN (SELECT actor FROM trusted_ctx)
      AND NOT EXISTS (SELECT 1 FROM trusted_ctx WHERE admin = 1);
END;
CREATE TRIGGER user_sessions_state_machine BEFORE UPDATE OF state ON user_sessions
WHEN NEW.state = 'active' AND OLD.state IN ('revoked','expired')
BEGIN
    SELECT RAISE(ABORT, 'a session that left active cannot re-enter it');
END;`

// activityChainMapDDL is the sixth step's object definition: the
// cross-reference between the activity ledger and its chain projection. The
// data movement itself (the backfill) runs as the step's same-transaction
// Go hook, because the chain digests cannot be computed in SQL.
const activityChainMapDDL = `CREATE TABLE activity_chain_map (
    activity_id INTEGER PRIMARY KEY REFERENCES activity (id),
    event_id    TEXT NOT NULL UNIQUE
               CHECK (length(event_id) = 64 AND
                      length(CAST(event_id AS BLOB)) = 64 AND
                      event_id NOT GLOB '*[^0-9a-f]*')
);`

// legacySealDDL is the eighth step's object definitions: the legacy actor
// attribution registry (Class A) and the execution-point seal of the legacy
// 'operator' actor. Every statement's text must stay byte-identical to the
// sealing block in schema.sql, so the sqlite_schema rows of a migrated spine
// hash like the reference spine built from the baseline.
const legacySealDDL = `CREATE TABLE legacy_actor_mapping (
    legacy_actor    TEXT PRIMARY KEY
                CHECK (length(legacy_actor) BETWEEN 1 AND 128),
    mapped_user_id  TEXT NOT NULL REFERENCES users (id),
    basis           TEXT NOT NULL
                CHECK (length(basis) BETWEEN 1 AND 256),
    nature          TEXT NOT NULL
                CHECK (length(nature) BETWEEN 1 AND 256),
    strength        TEXT NOT NULL
                CHECK (length(strength) BETWEEN 1 AND 256),
    evidence        TEXT
                CHECK (evidence IS NULL OR length(evidence) <= 4096),
    created_at      TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX legacy_actor_mapping_user ON legacy_actor_mapping (mapped_user_id);

CREATE TRIGGER legacy_actor_mapping_no_update BEFORE UPDATE ON legacy_actor_mapping
BEGIN SELECT RAISE(ABORT, 'legacy actor mappings are append only'); END;

CREATE TRIGGER legacy_actor_mapping_no_delete BEFORE DELETE ON legacy_actor_mapping
BEGIN SELECT RAISE(ABORT, 'legacy actor mappings are append only'); END;

CREATE TRIGGER activity_legacy_actor_sealed BEFORE INSERT ON activity
WHEN NEW.actor = 'operator'
BEGIN SELECT RAISE(ABORT, 'the legacy operator actor is sealed after the identity migration'); END;`

// degradedGrantsDDL is the ninth step's object definitions: the Degraded
// Operation Authorization registry and its object-scope guard. Every
// statement's text must stay byte-identical to the degraded grants block in
// schema.sql, so the sqlite_schema rows of a migrated spine hash like the
// reference spine built from the baseline.
const degradedGrantsDDL = `CREATE TABLE degraded_grants (
    grant_id              TEXT NOT NULL UNIQUE
                        CHECK (length(grant_id) = 32 AND
                               length(CAST(grant_id AS BLOB)) = 32 AND
                               grant_id NOT GLOB '*[^0-9a-f]*'),
    risk                  TEXT NOT NULL
                        CHECK (length(risk) BETWEEN 1 AND 128),
    scope                 TEXT NOT NULL
                        CHECK (length(scope) BETWEEN 1 AND 128),
    resource              TEXT
                        CHECK (resource IS NULL OR length(resource) <= 256),
    compensation          TEXT NOT NULL
                        CHECK (length(compensation) BETWEEN 1 AND 1024),
    revocation_conditions TEXT NOT NULL
                        CHECK (length(revocation_conditions) BETWEEN 1 AND 1024),
    issued_by_user_id     TEXT NOT NULL REFERENCES users (id),
    created_at            TEXT NOT NULL DEFAULT (datetime('now')),
    expires_at            TEXT NOT NULL,
    revoked_at            TEXT,
    PRIMARY KEY (grant_id)
);

CREATE INDEX degraded_grants_risk_scope ON degraded_grants (risk, scope);

CREATE TRIGGER degraded_grants_guarded_insert BEFORE INSERT ON degraded_grants
BEGIN
    SELECT RAISE(ABORT, 'degraded grant insert requires a guard-bound trusted context')
    WHERE NOT EXISTS (SELECT 1 FROM trusted_ctx);
    SELECT RAISE(ABORT, 'degraded grant attribution disagrees with the trusted context')
    WHERE NEW.issued_by_user_id NOT IN (SELECT actor FROM trusted_ctx);
END;`
