-- +goose Up
-- 0001_baseline marks the schema state produced by the legacy GORM
-- AutoMigrate bridge (see internal/db/migrate.EnsureUpToDate). It contains no
-- DDL: the baseline tables are created by the scoped AutoMigrate functions the
-- first time a database is bootstrapped, and this migration only records that
-- the baseline version has been reached.
--
-- All schema evolution after this point MUST be added as numbered migration
-- files in this directory (see docs/architecture/database-migration-strategy.md).
SELECT 1;

-- +goose Down
SELECT 1;
