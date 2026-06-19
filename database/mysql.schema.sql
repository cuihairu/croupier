-- MySQL 8.0+ schema for Croupier (database-per-game architecture)
-- This script creates the databases and all required tables.
-- Each game has its own independent database.

-- =============================================
-- croupier_meta (元数据库)
-- =============================================
-- Contains metadata for all games: users, roles, permissions, games, environments

-- 0) Create metadata database with utf8mb4
CREATE DATABASE IF NOT EXISTS `croupier_meta`
  DEFAULT CHARACTER SET utf8mb4
  DEFAULT COLLATE utf8mb4_unicode_ci;
USE `croupier_meta`;

-- 1) Users, Roles, Role-Perms (gorm.Model)
CREATE TABLE IF NOT EXISTS `user_accounts` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `username` VARCHAR(64) NOT NULL,
  `display_name` VARCHAR(128) NULL,
  `email` VARCHAR(256) NULL,
  `phone` VARCHAR(32) NULL,
  `password_hash` VARCHAR(255) NULL,
  `active` TINYINT(1) NOT NULL DEFAULT 1,
  `otp_secret` VARCHAR(64) NULL,
  `created_at` DATETIME(3) NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3) NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_accounts_username` (`username`),
  KEY `idx_user_accounts_email` (`email`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `role_records` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `name` VARCHAR(64) NOT NULL,
  `description` VARCHAR(256) NULL,
  `created_at` DATETIME(3) NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3) NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_role_records_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `user_role_records` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id` BIGINT UNSIGNED NOT NULL,
  `role_id` BIGINT UNSIGNED NOT NULL,
  `created_at` DATETIME(3) NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3) NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_role_records_user_role` (`user_id`,`role_id`),
  KEY `idx_user_role_records_user_id` (`user_id`),
  KEY `idx_user_role_records_role_id` (`role_id`),
  CONSTRAINT `fk_user_role_user` FOREIGN KEY (`user_id`) REFERENCES `user_accounts` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_user_role_role` FOREIGN KEY (`role_id`) REFERENCES `role_records` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `role_perm_records` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `role_id` BIGINT UNSIGNED NOT NULL,
  `perm` VARCHAR(128) NOT NULL,
  `created_at` DATETIME(3) NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3) NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  KEY `idx_role_perm_role` (`role_id`),
  KEY `idx_role_perm_perm` (`perm`),
  CONSTRAINT `fk_role_perm_role` FOREIGN KEY (`role_id`) REFERENCES `role_records` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 2) Games Registry (metadata only)
CREATE TABLE IF NOT EXISTS `games` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `game_id` VARCHAR(64) NOT NULL,
  `name` VARCHAR(200) NOT NULL,
  `icon` VARCHAR(255) NULL,
  `description` TEXT NULL,
  `enabled` TINYINT(1) NOT NULL DEFAULT 1,
  `created_at` DATETIME(3) NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3) NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_games_game_id` (`game_id`),
  KEY `idx_games_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 3) Game Environments (metadata only)
CREATE TABLE IF NOT EXISTS `game_envs` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `game_id` VARCHAR(64) NOT NULL,
  `env` VARCHAR(64) NOT NULL,
  `database_name` VARCHAR(128) NOT NULL,
  `created_at` DATETIME(3) NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3) NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_game_envs_game_env` (`game_id`, `env`),
  KEY `idx_game_envs_game_id` (`game_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 4) Internal messages
CREATE TABLE IF NOT EXISTS `message_records` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `to_user_id` BIGINT UNSIGNED NOT NULL,
  `from_user_id` BIGINT UNSIGNED NULL,
  `title` VARCHAR(200) NULL,
  `content` TEXT NULL,
  `type` VARCHAR(32) NULL,
  `read_at` DATETIME(3) NULL,
  `created_at` DATETIME(3) NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3) NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  KEY `idx_message_records_to_user` (`to_user_id`),
  KEY `idx_message_records_read_at` (`read_at`),
  CONSTRAINT `fk_message_to_user` FOREIGN KEY (`to_user_id`) REFERENCES `user_accounts` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_message_from_user` FOREIGN KEY (`from_user_id`) REFERENCES `user_accounts` (`id`) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `broadcast_message_records` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `title` VARCHAR(200) NULL,
  `content` TEXT NULL,
  `type` VARCHAR(32) NULL,
  `audience` VARCHAR(16) NOT NULL,
  `created_at` DATETIME(3) NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3) NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `broadcast_role_records` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `broadcast_id` BIGINT UNSIGNED NOT NULL,
  `role_name` VARCHAR(64) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_broadcast_role_records_broadcast` (`broadcast_id`),
  KEY `idx_broadcast_role_records_role_name` (`role_name`),
  CONSTRAINT `fk_broadcast_role_broadcast` FOREIGN KEY (`broadcast_id`) REFERENCES `broadcast_message_records` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `broadcast_ack_records` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `broadcast_id` BIGINT UNSIGNED NOT NULL,
  `user_id` BIGINT UNSIGNED NOT NULL,
  `read_at` DATETIME(3) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_broadcast_ack_user` (`broadcast_id`,`user_id`),
  KEY `idx_broadcast_ack_user` (`user_id`),
  CONSTRAINT `fk_broadcast_ack_broadcast` FOREIGN KEY (`broadcast_id`) REFERENCES `broadcast_message_records` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_broadcast_ack_user` FOREIGN KEY (`user_id`) REFERENCES `user_accounts` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- =============================================
-- game_demo_prod (游戏数据库示例)
-- =============================================
-- Each game has its own independent database.
-- The game_id and env are implicit from the database name.
-- Example: game_demo_prod, game_demo_staging, game_rpg_prod

-- 5) Create game database
CREATE DATABASE IF NOT EXISTS `game_demo_prod`
  DEFAULT CHARACTER SET utf8mb4
  DEFAULT COLLATE utf8mb4_unicode_ci;
USE `game_demo_prod`;

-- 6) Game Events (game_id and env implicit from database name)
CREATE TABLE IF NOT EXISTS `events` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `event_time` DATETIME(3) NULL DEFAULT CURRENT_TIMESTAMP(3),
  `user_id` VARCHAR(64) NULL,
  `session_id` VARCHAR(64) NULL,
  `event` VARCHAR(128) NOT NULL,
  `channel` VARCHAR(64) NULL,
  `platform` VARCHAR(32) NULL,
  `country` CHAR(2) NULL,
  `app_version` VARCHAR(32) NULL,
  `event_id` CHAR(36) NULL,
  `server_id` VARCHAR(64) NULL,
  `props_json` TEXT NULL,
  `created_at` DATETIME(3) NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `idx_events_event_time` (`event_time`),
  KEY `idx_events_user_id` (`user_id`),
  KEY `idx_events_event` (`event`),
  KEY `idx_events_server_id` (`server_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 7) Game Payments (game_id and env implicit from database name)
CREATE TABLE IF NOT EXISTS `payments` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `time` DATETIME(3) NULL DEFAULT CURRENT_TIMESTAMP(3),
  `user_id` VARCHAR(64) NOT NULL,
  `order_id` VARCHAR(128) NOT NULL,
  `amount_cents` BIGINT UNSIGNED NOT NULL,
  `currency` CHAR(3) NULL,
  `status` VARCHAR(32) NULL,
  `channel` VARCHAR(64) NULL,
  `platform` VARCHAR(32) NULL,
  `country` CHAR(2) NULL,
  `region` VARCHAR(64) NULL,
  `city` VARCHAR(128) NULL,
  `product_id` VARCHAR(128) NULL,
  `reason` TEXT NULL,
  `server_id` VARCHAR(64) NULL,
  `created_at` DATETIME(3) NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_payments_order_id` (`order_id`),
  KEY `idx_payments_time` (`time`),
  KEY `idx_payments_user_id` (`user_id`),
  KEY `idx_payments_status` (`status`),
  KEY `idx_payments_server_id` (`server_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 8) Game Metrics (game_id and env implicit from database name)
CREATE TABLE IF NOT EXISTS `game_metrics` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `metric_date` DATE NOT NULL,
  `server_id` VARCHAR(64) NULL,
  `dau` BIGINT UNSIGNED NULL,
  `new_users` BIGINT UNSIGNED NULL,
  `revenue_cents` BIGINT UNSIGNED NULL,
  `peak_online` INTEGER UNSIGNED NULL,
  `version` BIGINT UNSIGNED NULL,
  `created_at` DATETIME(3) NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_game_metrics_date_server` (`metric_date`, `server_id`),
  KEY `idx_game_metrics_metric_date` (`metric_date`),
  KEY `idx_game_metrics_server_id` (`server_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
