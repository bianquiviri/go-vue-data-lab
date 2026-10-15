-- Audit table for the synthetic data generator (Issue #7).
-- Each row records a completed dataset identified by its fingerprint
-- (seed + counts). The generator uses it to skip already-generated datasets
-- and to make re-runs idempotent.
CREATE TABLE IF NOT EXISTS generator_seeds (
  dataset_key TEXT PRIMARY KEY,
  seed BIGINT NOT NULL,
  customers INTEGER NOT NULL,
  products INTEGER NOT NULL,
  orders INTEGER NOT NULL,
  min_items INTEGER NOT NULL,
  max_items INTEGER NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
