-- ============================================================================
-- Migration: Function OpenAPI Backfill
-- Date: 2026-02-28
-- Description:
--   1) Backfill `open_api_spec` from legacy `openapi_operation`
--   2) Normalize `spec_format` default values
--   3) Keep old columns for compatibility (do not drop in this step)
-- ============================================================================

-- Ensure target columns exist (GORM also manages these during AutoMigrate).
ALTER TABLE functions ADD COLUMN IF NOT EXISTS open_api_spec JSONB;
ALTER TABLE functions ADD COLUMN IF NOT EXISTS spec_format VARCHAR(32);

-- Backfill OpenAPI payload from older column name.
UPDATE functions
SET open_api_spec = COALESCE(open_api_spec, openapi_operation)
WHERE openapi_operation IS NOT NULL;

-- Normalize spec format values.
UPDATE functions
SET spec_format = 'openapi3.0.3'
WHERE (spec_format IS NULL OR spec_format = '')
  AND open_api_spec IS NOT NULL;

UPDATE functions
SET spec_format = 'legacy'
WHERE spec_format IS NULL OR spec_format = '';

-- Optional index for filtering by spec format.
CREATE INDEX IF NOT EXISTS idx_functions_spec_format ON functions(spec_format);
