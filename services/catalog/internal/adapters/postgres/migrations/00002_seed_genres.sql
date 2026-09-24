-- +goose Up
-- Curated starting genres. IDs are fixed so fixtures and clients can rely on them.
INSERT INTO catalog.genres (id, slug, name) VALUES
    ('01926f3a-0000-7000-8000-000000000001', 'ambient',          'Ambient'),
    ('01926f3a-0000-7000-8000-000000000002', 'ambient-pop',      'Ambient pop'),
    ('01926f3a-0000-7000-8000-000000000003', 'ambient-folk',     'Ambient folk'),
    ('01926f3a-0000-7000-8000-000000000004', 'downtempo',        'Downtempo'),
    ('01926f3a-0000-7000-8000-000000000005', 'electronic',       'Electronic'),
    ('01926f3a-0000-7000-8000-000000000006', 'field-recordings', 'Field recordings'),
    ('01926f3a-0000-7000-8000-000000000007', 'jazz',             'Jazz'),
    ('01926f3a-0000-7000-8000-000000000008', 'classical',        'Classical'),
    ('01926f3a-0000-7000-8000-000000000009', 'hip-hop',          'Hip-hop'),
    ('01926f3a-0000-7000-8000-00000000000a', 'rock',             'Rock')
ON CONFLICT (id) DO NOTHING;

-- +goose Down
DELETE FROM catalog.genres WHERE id::text LIKE '01926f3a-0000-7000-8000-%';
