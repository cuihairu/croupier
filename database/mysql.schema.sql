-- MySQL 8.0+ schema for Croupier (aligned to current GORM models)
-- This script creates the database (utf8mb4) and all required tables.

-- 0) Create database with utf8mb4
CREATE DATABASE IF NOT EXISTS `croupier`
  DEFAULT CHARACTER SET utf8mb4
  DEFAULT COLLATE utf8mb4_unicode_ci;
USE `croupier`;

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

-- 2) Games & Game Envs
CREATE TABLE IF NOT EXISTS `games` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `name` VARCHAR(200) NOT NULL,
  `icon` VARCHAR(255) NULL,
  `description` TEXT NULL,
  `enabled` TINYINT(1) NOT NULL DEFAULT 1,
  `created_at` DATETIME(3) NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3) NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  KEY `idx_games_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `game_envs` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `game_id` BIGINT UNSIGNED NOT NULL,
  `env` VARCHAR(64) NOT NULL,
  `created_at` DATETIME(3) NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3) NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_game_envs_game_env` (`game_id`, `env`),
  KEY `idx_game_envs_game_id` (`game_id`),
  CONSTRAINT `fk_game_envs_game` FOREIGN KEY (`game_id`) REFERENCES `games` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 3) Internal messages
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
