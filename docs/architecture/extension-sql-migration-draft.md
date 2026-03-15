# Extension SQL Migration Draft

更新时间：2026-03-15
状态：草案

本文件定义扩展系统第一阶段的 SQL migration 草案。

---

## 1. 目标

第一阶段先落以下表：

- `extension_catalogs`
- `extension_releases`
- `extension_installations`
- `extension_runtime_bindings`
- `extension_events`

目标：

- 支撑 catalog / installation / binding / event 基础流程
- 尽量与当前 MySQL / GORM 使用习惯一致

---

## 2. Migration 文件建议

建议新增：

- `003_extension_catalog.sql`
- `004_extension_release.sql`
- `005_extension_installation.sql`
- `006_extension_runtime_binding.sql`
- `007_extension_event.sql`

如果希望集中一次建表，也可以合并为：

- `003_extension_runtime_init.sql`

当前更建议拆开，便于回滚和审查。

---

## 3. SQL 草案

### 3.1 `003_extension_catalog.sql`

```sql
CREATE TABLE IF NOT EXISTS extension_catalogs (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  deleted_at DATETIME(3) NULL,
  extension_id VARCHAR(128) NOT NULL,
  name VARCHAR(128) NOT NULL,
  display_name VARCHAR(255) NOT NULL,
  vendor VARCHAR(128) NOT NULL,
  kind VARCHAR(32) NOT NULL,
  summary TEXT NULL,
  icon_url VARCHAR(512) NULL,
  homepage_url VARCHAR(512) NULL,
  status VARCHAR(32) NOT NULL,
  latest_version VARCHAR(64) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_extension_catalogs_extension_id (extension_id),
  KEY idx_extension_catalogs_deleted_at (deleted_at),
  KEY idx_extension_catalogs_kind (kind),
  KEY idx_extension_catalogs_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

### 3.2 `004_extension_release.sql`

```sql
CREATE TABLE IF NOT EXISTS extension_releases (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  deleted_at DATETIME(3) NULL,
  extension_id VARCHAR(128) NOT NULL,
  version VARCHAR(64) NOT NULL,
  release_channel VARCHAR(32) NOT NULL,
  manifest_json LONGTEXT NOT NULL,
  package_ref VARCHAR(512) NULL,
  checksum VARCHAR(128) NULL,
  min_core_version VARCHAR(64) NULL,
  changelog TEXT NULL,
  published_at_unix BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (id),
  UNIQUE KEY uk_extension_releases_extension_version (extension_id, version),
  KEY idx_extension_releases_deleted_at (deleted_at),
  KEY idx_extension_releases_release_channel (release_channel),
  KEY idx_extension_releases_published_at_unix (published_at_unix)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

### 3.3 `005_extension_installation.sql`

```sql
CREATE TABLE IF NOT EXISTS extension_installations (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  deleted_at DATETIME(3) NULL,
  installation_key VARCHAR(191) NOT NULL,
  extension_id VARCHAR(128) NOT NULL,
  release_version VARCHAR(64) NOT NULL,
  scope_type VARCHAR(32) NOT NULL,
  scope_id VARCHAR(128) NOT NULL,
  target_type VARCHAR(32) NOT NULL,
  target_id VARCHAR(128) NULL,
  status VARCHAR(32) NOT NULL,
  desired_state VARCHAR(32) NOT NULL,
  enabled TINYINT(1) NOT NULL DEFAULT 0,
  config_json LONGTEXT NULL,
  secret_refs_json LONGTEXT NULL,
  last_error TEXT NULL,
  installed_by VARCHAR(128) NULL,
  installed_at_unix BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (id),
  UNIQUE KEY uk_extension_installations_installation_key (installation_key),
  KEY idx_extension_installations_deleted_at (deleted_at),
  KEY idx_extension_installations_extension_id (extension_id),
  KEY idx_extension_installations_scope (scope_type, scope_id),
  KEY idx_extension_installations_target (target_type, target_id),
  KEY idx_extension_installations_status (status),
  KEY idx_extension_installations_desired_state (desired_state),
  KEY idx_extension_installations_enabled (enabled),
  KEY idx_extension_installations_installed_at_unix (installed_at_unix)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

### 3.4 `006_extension_runtime_binding.sql`

```sql
CREATE TABLE IF NOT EXISTS extension_runtime_bindings (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  deleted_at DATETIME(3) NULL,
  installation_id BIGINT UNSIGNED NOT NULL,
  binding_type VARCHAR(32) NOT NULL,
  binding_key VARCHAR(191) NOT NULL,
  target_ref VARCHAR(255) NULL,
  spec_json LONGTEXT NULL,
  status VARCHAR(32) NOT NULL,
  last_error TEXT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_extension_runtime_bindings_installation_key (installation_id, binding_key),
  KEY idx_extension_runtime_bindings_deleted_at (deleted_at),
  KEY idx_extension_runtime_bindings_installation_id (installation_id),
  KEY idx_extension_runtime_bindings_binding_type (binding_type),
  KEY idx_extension_runtime_bindings_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

### 3.5 `007_extension_event.sql`

```sql
CREATE TABLE IF NOT EXISTS extension_events (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  deleted_at DATETIME(3) NULL,
  installation_id BIGINT UNSIGNED NOT NULL,
  event_type VARCHAR(32) NOT NULL,
  level VARCHAR(16) NOT NULL,
  message TEXT NOT NULL,
  payload_json LONGTEXT NULL,
  created_by VARCHAR(128) NULL,
  PRIMARY KEY (id),
  KEY idx_extension_events_deleted_at (deleted_at),
  KEY idx_extension_events_installation_id (installation_id),
  KEY idx_extension_events_event_type (event_type),
  KEY idx_extension_events_level (level),
  KEY idx_extension_events_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

---

## 4. 第一阶段不建的表

第一阶段可暂不建：

- `extension_capabilities`
- `extension_health`
- `extension_secret_bindings`

理由：

- capability 可以先由 manifest 现算
- health 可以先落到 event 或状态字段
- secret binding 第一阶段可先存在 `secret_refs_json`

---

## 5. 迁移策略建议

### 5.1 第一阶段

- 先建新表
- 不删除任何旧表
- 不替换旧 YAML 配置链路

### 5.2 第二阶段

- `official.external-platform` 接入 installation 数据源
- 旧 YAML 只做兼容输入

### 5.3 第三阶段

- YAML 不再作为主数据源

---

## 6. 风险点

- 如果后续决定统一使用 JSON column，需要评估 MySQL 版本兼容性
- 如果 installation_key 规则不稳定，后续会影响幂等安装与升级
- 如果 target scope 模型频繁变动，会影响索引设计

---

## 7. 下一步

下一步需要补：

- 与 GORM model 对齐校验
- migration 执行顺序确认
- installation_key 生成规则
