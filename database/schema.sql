-- Croupier SQL schema (code-first). This file mirrors current GORM models.
-- Database-per-game architecture: Each game has its own independent database.
-- Note: Tables are created automatically by GORM at runtime. This schema
-- provides a reference or for manual bootstrap in non-GORM environments.

-- =============================================
-- croupier_meta (元数据库)
-- =============================================
-- Contains metadata for all games: users, roles, permissions, games, environments

-- Users, roles and permissions (string-based perms)
CREATE TABLE IF NOT EXISTS user_accounts (
  id SERIAL PRIMARY KEY,
  username VARCHAR(64) UNIQUE NOT NULL,
  display_name VARCHAR(128),
  email VARCHAR(256),
  phone VARCHAR(32),
  password_hash VARCHAR(255),
  active BOOLEAN DEFAULT TRUE,
  otp_secret VARCHAR(64),
  created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE TABLE IF NOT EXISTS role_records (
  id SERIAL PRIMARY KEY,
  name VARCHAR(64) UNIQUE NOT NULL,
  description VARCHAR(256),
  created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE TABLE IF NOT EXISTS user_role_records (
  id SERIAL PRIMARY KEY,
  user_id INTEGER REFERENCES user_records(id) ON DELETE CASCADE,
  role_id INTEGER REFERENCES role_records(id) ON DELETE CASCADE,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  deleted_at TIMESTAMP WITH TIME ZONE,
  UNIQUE(user_id, role_id)
);

CREATE TABLE IF NOT EXISTS role_perm_records (
  id SERIAL PRIMARY KEY,
  role_id INTEGER REFERENCES role_records(id) ON DELETE CASCADE,
  perm VARCHAR(128) NOT NULL,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_user_accounts_username ON user_accounts(username);
CREATE INDEX IF NOT EXISTS idx_user_accounts_email ON user_accounts(email);
CREATE INDEX IF NOT EXISTS idx_user_role_records_user_id ON user_role_records(user_id);
CREATE INDEX IF NOT EXISTS idx_user_role_records_role_id ON user_role_records(role_id);
CREATE INDEX IF NOT EXISTS idx_role_perm_role ON role_perm_records(role_id);
CREATE INDEX IF NOT EXISTS idx_role_perm_perm ON role_perm_records(perm);

-- Games registry (metadata only)
CREATE TABLE IF NOT EXISTS games (
  id SERIAL PRIMARY KEY,
  game_id VARCHAR(64) UNIQUE NOT NULL,
  name VARCHAR(200) NOT NULL,
  icon VARCHAR(255),
  description TEXT,
  enabled BOOLEAN DEFAULT TRUE,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_games_game_id ON games(game_id);
CREATE INDEX IF NOT EXISTS idx_games_name ON games(name);

-- Game environments (metadata only)
CREATE TABLE IF NOT EXISTS game_envs (
  id SERIAL PRIMARY KEY,
  game_id VARCHAR(64) NOT NULL,
  env VARCHAR(64) NOT NULL,
  database_name VARCHAR(128) NOT NULL,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  deleted_at TIMESTAMP WITH TIME ZONE,
  UNIQUE(game_id, env)
);

CREATE INDEX IF NOT EXISTS idx_game_envs_game_id ON game_envs(game_id);

-- Internal messaging
CREATE TABLE IF NOT EXISTS message_records (
  id SERIAL PRIMARY KEY,
  to_user_id INTEGER REFERENCES user_accounts(id) ON DELETE CASCADE,
  from_user_id INTEGER REFERENCES user_accounts(id) ON DELETE SET NULL,
  title VARCHAR(200),
  content TEXT,
  type VARCHAR(32),
  read_at TIMESTAMP WITH TIME ZONE,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_message_records_to_user ON message_records(to_user_id);
CREATE INDEX IF NOT EXISTS idx_message_records_read_at ON message_records(read_at);

CREATE TABLE IF NOT EXISTS broadcast_message_records (
  id SERIAL PRIMARY KEY,
  title VARCHAR(200),
  content TEXT,
  type VARCHAR(32),
  audience VARCHAR(16) NOT NULL CHECK (audience IN ('all','roles')),
  created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE TABLE IF NOT EXISTS broadcast_role_records (
  id SERIAL PRIMARY KEY,
  broadcast_id INTEGER REFERENCES broadcast_message_records(id) ON DELETE CASCADE,
  role_name VARCHAR(64) NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_broadcast_role_records_role_name ON broadcast_role_records(role_name);

CREATE TABLE IF NOT EXISTS broadcast_ack_records (
  id SERIAL PRIMARY KEY,
  broadcast_id INTEGER REFERENCES broadcast_message_records(id) ON DELETE CASCADE,
  user_id INTEGER REFERENCES user_accounts(id) ON DELETE CASCADE,
  read_at TIMESTAMP WITH TIME ZONE NOT NULL,
  UNIQUE (broadcast_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_broadcast_ack_records_user ON broadcast_ack_records(user_id);

-- =============================================
-- game_demo_prod (游戏数据库示例)
-- =============================================
-- Each game has its own independent database.
-- The game_id and env are implicit from the database name.
-- Example: game_demo_prod, game_demo_staging, game_rpg_prod

CREATE DATABASE IF NOT EXISTS game_demo_prod;

-- Connect to game database and create game-specific tables
\c game_demo_prod

-- Game events (game_id and env implicit from database name)
CREATE TABLE IF NOT EXISTS events (
  id SERIAL PRIMARY KEY,
  event_time TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  user_id VARCHAR(64),
  session_id VARCHAR(64),
  event VARCHAR(128) NOT NULL,
  channel VARCHAR(64),
  platform VARCHAR(32),
  country FixedString(2),
  app_version VARCHAR(32),
  event_id UUID,
  server_id VARCHAR(64), -- MMORPG multi-server support
  props_json TEXT,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_events_event_time ON events(event_time);
CREATE INDEX IF NOT EXISTS idx_events_user_id ON events(user_id);
CREATE INDEX IF NOT EXISTS idx_events_event ON events(event);
CREATE INDEX IF NOT EXISTS idx_events_server_id ON events(server_id);

-- Game payments (game_id and env implicit from database name)
CREATE TABLE IF NOT EXISTS payments (
  id SERIAL PRIMARY KEY,
  time TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  user_id VARCHAR(64) NOT NULL,
  order_id VARCHAR(128) NOT NULL,
  amount_cents BIGINT NOT NULL,
  currency FixedString(3),
  status VARCHAR(32),
  channel VARCHAR(64),
  platform VARCHAR(32),
  country FixedString(2),
  region VARCHAR(64),
  city VARCHAR(128),
  product_id VARCHAR(128),
  reason TEXT,
  server_id VARCHAR(64), -- MMORPG multi-server support
  created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(order_id)
);

CREATE INDEX IF NOT EXISTS idx_payments_time ON payments(time);
CREATE INDEX IF NOT EXISTS idx_payments_user_id ON payments(user_id);
CREATE INDEX IF NOT EXISTS idx_payments_order_id ON payments(order_id);
CREATE INDEX IF NOT EXISTS idx_payments_status ON payments(status);
CREATE INDEX IF NOT EXISTS idx_payments_server_id ON payments(server_id);

-- Game-specific metrics (game_id and env implicit from database name)
CREATE TABLE IF NOT EXISTS game_metrics (
  id SERIAL PRIMARY KEY,
  metric_date DATE NOT NULL,
  server_id VARCHAR(64),
  dau BIGINT,
  new_users BIGINT,
  revenue_cents BIGINT,
  peak_online INTEGER,
  version BIGINT,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(metric_date, server_id)
);

CREATE INDEX IF NOT EXISTS idx_game_metrics_metric_date ON game_metrics(metric_date);
CREATE INDEX IF NOT EXISTS idx_game_metrics_server_id ON game_metrics(server_id);
