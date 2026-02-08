-- ============================================================================
-- Migration: OpenAPI 3.0.3 Schema Support
-- Date: 2025-02-09
-- Description: Remove legacy descriptor fields, add OpenAPI 3.0.3 support
-- ============================================================================

-- 1. Delete legacy descriptor fields from functions table
-- (Skip if columns don't exist to avoid errors)
ALTER TABLE functions DROP COLUMN IF EXISTS params;
ALTER TABLE functions DROP COLUMN IF EXISTS descriptor;
ALTER TABLE functions DROP COLUMN IF EXISTS manifest;

-- 2. Add new OpenAPI 3.0.3 columns
ALTER TABLE functions ADD COLUMN IF NOT EXISTS openapi_operation JSONB;
ALTER TABLE functions ADD COLUMN IF NOT EXISTS request_schema TEXT;
ALTER TABLE functions ADD COLUMN IF NOT EXISTS response_schema TEXT;

-- 3. Create index on openapi_operation for faster queries
CREATE INDEX IF NOT EXISTS idx_functions_openapi_operation ON functions USING GIN (openapi_operation);

-- 4. Add comments
COMMENT ON COLUMN functions.openapi_operation IS 'OpenAPI 3.0.3 Operation Object in JSON format';
COMMENT ON COLUMN functions.request_schema IS 'JSON Schema for request body validation';
COMMENT ON COLUMN functions.response_schema IS 'JSON Schema for response body validation';

-- Migration completed successfully
