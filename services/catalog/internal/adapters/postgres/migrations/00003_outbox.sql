-- +goose Up
-- Transactional outbox (libs/platform/outbox): catalog.events messages are
-- written here in the transaction of the change and relayed to Kafka.
CREATE TABLE catalog.outbox (
    id         bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    topic      text        NOT NULL,
    key        bytea,
    value      bytea       NOT NULL,
    headers    jsonb       NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE catalog.outbox;
