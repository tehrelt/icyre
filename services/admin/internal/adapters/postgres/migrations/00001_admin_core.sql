-- +goose Up
CREATE TABLE admin.moderation_operations (
    id                  uuid        PRIMARY KEY,
    actor_id            uuid        NOT NULL,
    idempotency_key     text        NOT NULL,
    request_hash        text        NOT NULL,
    action              text        NOT NULL CHECK (action IN ('track.block', 'track.unblock', 'user.block', 'user.unblock')),
    target_type         text        NOT NULL CHECK (target_type IN ('track', 'user')),
    target_id           uuid        NOT NULL,
    desired_blocked     boolean     NOT NULL,
    reason              text        NOT NULL CHECK (char_length(reason) BETWEEN 1 AND 512),
    status              text        NOT NULL CHECK (status IN ('PENDING', 'APPLIED', 'NOOP', 'REJECTED')),
    result_code          text        NOT NULL DEFAULT '',
    before_state        jsonb       NOT NULL DEFAULT '{}',
    after_state         jsonb       NOT NULL DEFAULT '{}',
    revocations_pending boolean     NOT NULL DEFAULT false,
    request_id          text        NOT NULL DEFAULT '',
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    completed_at        timestamptz,
    UNIQUE (actor_id, idempotency_key)
);

CREATE INDEX moderation_operations_pending_idx
    ON admin.moderation_operations (status, created_at ASC);

CREATE TABLE admin.audit_logs (
    id           uuid        PRIMARY KEY,
    operation_id uuid        NOT NULL,
    actor_id     uuid        NOT NULL,
    action       text        NOT NULL CHECK (action IN ('track.block', 'track.unblock', 'user.block', 'user.unblock')),
    target_type  text        NOT NULL CHECK (target_type IN ('track', 'user')),
    target_id    uuid        NOT NULL,
    phase        text        NOT NULL CHECK (phase IN ('requested', 'applied', 'noop', 'rejected')),
    reason       text        NOT NULL CHECK (char_length(reason) BETWEEN 1 AND 512),
    metadata     jsonb       NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(metadata) = 'object'),
    request_id   text        NOT NULL DEFAULT '',
    created_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (operation_id, phase)
);

CREATE INDEX audit_logs_created_idx
    ON admin.audit_logs (created_at DESC, id DESC);

CREATE INDEX audit_logs_target_idx
    ON admin.audit_logs (target_type, target_id, created_at DESC, id DESC);

CREATE INDEX audit_logs_actor_idx
    ON admin.audit_logs (actor_id, created_at DESC, id DESC);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION admin.prevent_audit_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'admin.audit_logs is immutable';
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER audit_logs_immutable
    BEFORE UPDATE OR DELETE ON admin.audit_logs
    FOR EACH ROW
    EXECUTE FUNCTION admin.prevent_audit_mutation();

CREATE TRIGGER audit_logs_no_truncate
    BEFORE TRUNCATE ON admin.audit_logs
    FOR EACH STATEMENT
    EXECUTE FUNCTION admin.prevent_audit_mutation();

CREATE TABLE admin.transcode_jobs (
    upload_id      uuid        PRIMARY KEY,
    track_id       uuid        NOT NULL,
    status         text        NOT NULL CHECK (status IN ('QUEUED', 'SUCCEEDED', 'FAILED')),
    failure_reason text        NOT NULL DEFAULT '',
    event_at       timestamptz NOT NULL,
    projected_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX transcode_jobs_event_idx
    ON admin.transcode_jobs (event_at DESC, upload_id DESC);

CREATE INDEX transcode_jobs_track_idx
    ON admin.transcode_jobs (track_id, event_at DESC, upload_id DESC);

-- +goose Down
DROP TABLE admin.transcode_jobs;
DROP TRIGGER audit_logs_no_truncate ON admin.audit_logs;
DROP TRIGGER audit_logs_immutable ON admin.audit_logs;
DROP FUNCTION admin.prevent_audit_mutation();
DROP TABLE admin.audit_logs;
DROP TABLE admin.moderation_operations;
