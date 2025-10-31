-- Croupier Database Schema
-- 支持用户账号、权限管理、游戏管理和系统持久化

-- ================================
-- 1. 用户账号管理
-- ================================

-- 用户表
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    email VARCHAR(100) UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    salt VARCHAR(32) NOT NULL,
    display_name VARCHAR(100),
    avatar_url TEXT,
    status VARCHAR(20) DEFAULT 'active' CHECK (status IN ('active', 'inactive', 'banned')),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    last_login_at TIMESTAMP WITH TIME ZONE
);

-- 角色表
CREATE TABLE roles (
    id SERIAL PRIMARY KEY,
    name VARCHAR(50) UNIQUE NOT NULL,
    display_name VARCHAR(100),
    description TEXT,
    is_system BOOLEAN DEFAULT false, -- 系统内置角色不可删除
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 用户角色关联表
CREATE TABLE user_roles (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    role_id INTEGER REFERENCES roles(id) ON DELETE CASCADE,
    granted_by INTEGER REFERENCES users(id),
    granted_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP WITH TIME ZONE,
    UNIQUE(user_id, role_id)
);

-- 权限表
CREATE TABLE permissions (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) UNIQUE NOT NULL, -- 如: game:create, function:invoke, user:manage
    display_name VARCHAR(100),
    description TEXT,
    resource_type VARCHAR(50), -- game, function, user, system
    action VARCHAR(50), -- create, read, update, delete, invoke, manage
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 角色权限关联表
CREATE TABLE role_permissions (
    id SERIAL PRIMARY KEY,
    role_id INTEGER REFERENCES roles(id) ON DELETE CASCADE,
    permission_id INTEGER REFERENCES permissions(id) ON DELETE CASCADE,
    resource_filter JSONB, -- 资源过滤条件，如 {"game_id": ["game1", "game2"]}
    UNIQUE(role_id, permission_id)
);

-- ================================
-- 2. 游戏管理
-- ================================

-- 游戏表
CREATE TABLE games (
    id SERIAL PRIMARY KEY,
    game_id VARCHAR(100) UNIQUE NOT NULL, -- 如: mmorpg, fps_shooter
    name VARCHAR(200) NOT NULL,
    display_name VARCHAR(200),
    description TEXT,
    icon_url TEXT,
    homepage_url TEXT,
    version VARCHAR(50),
    status VARCHAR(20) DEFAULT 'active' CHECK (status IN ('active', 'inactive', 'maintenance')),
    tags JSONB, -- 游戏标签数组
    metadata JSONB, -- 额外元数据
    created_by INTEGER REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 游戏环境表
CREATE TABLE game_environments (
    id SERIAL PRIMARY KEY,
    game_id INTEGER REFERENCES games(id) ON DELETE CASCADE,
    env_name VARCHAR(50) NOT NULL, -- dev, staging, production
    display_name VARCHAR(100),
    description TEXT,
    config JSONB, -- 环境配置
    status VARCHAR(20) DEFAULT 'active' CHECK (status IN ('active', 'inactive')),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(game_id, env_name)
);

-- 游戏管理员表（游戏级权限）
CREATE TABLE game_admins (
    id SERIAL PRIMARY KEY,
    game_id INTEGER REFERENCES games(id) ON DELETE CASCADE,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(50) DEFAULT 'admin' CHECK (role IN ('owner', 'admin', 'developer', 'viewer')),
    granted_by INTEGER REFERENCES users(id),
    granted_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(game_id, user_id)
);

-- ================================
-- 3. Agent 注册和函数管理
-- ================================

-- Agent 注册表
CREATE TABLE agents (
    id SERIAL PRIMARY KEY,
    agent_id VARCHAR(100) UNIQUE NOT NULL,
    game_id INTEGER REFERENCES games(id) ON DELETE CASCADE,
    env_name VARCHAR(50) NOT NULL,
    rpc_addr VARCHAR(100) NOT NULL,
    version VARCHAR(50),
    metadata JSONB,
    status VARCHAR(20) DEFAULT 'active' CHECK (status IN ('active', 'inactive', 'disconnected')),
    last_heartbeat TIMESTAMP WITH TIME ZONE,
    registered_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP WITH TIME ZONE,
    FOREIGN KEY (game_id, env_name) REFERENCES game_environments(game_id, env_name)
);

-- 函数注册表
CREATE TABLE functions (
    id SERIAL PRIMARY KEY,
    function_id VARCHAR(100) NOT NULL,
    agent_id VARCHAR(100) REFERENCES agents(agent_id) ON DELETE CASCADE,
    game_id INTEGER REFERENCES games(id) ON DELETE CASCADE,
    env_name VARCHAR(50) NOT NULL,
    display_name VARCHAR(200),
    description TEXT,
    input_schema JSONB,
    output_schema JSONB,
    metadata JSONB,
    status VARCHAR(20) DEFAULT 'active' CHECK (status IN ('active', 'inactive')),
    registered_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(function_id, agent_id, game_id, env_name)
);

-- ================================
-- 4. Job 执行和审计
-- ================================

-- Job 执行记录
CREATE TABLE jobs (
    id SERIAL PRIMARY KEY,
    job_id VARCHAR(100) UNIQUE NOT NULL,
    function_id VARCHAR(100) NOT NULL,
    agent_id VARCHAR(100) REFERENCES agents(agent_id),
    game_id INTEGER REFERENCES games(id),
    env_name VARCHAR(50),
    user_id INTEGER REFERENCES users(id),
    idempotency_key VARCHAR(100),
    request_payload JSONB,
    response_payload JSONB,
    status VARCHAR(20) DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'success', 'failed', 'cancelled')),
    error_message TEXT,
    started_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP WITH TIME ZONE,
    duration_ms INTEGER,
    metadata JSONB
);

-- Job 事件流（用于流式返回）
CREATE TABLE job_events (
    id SERIAL PRIMARY KEY,
    job_id VARCHAR(100) REFERENCES jobs(job_id) ON DELETE CASCADE,
    event_type VARCHAR(50) NOT NULL, -- progress, log, error, completed
    event_data JSONB,
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 审计日志
CREATE TABLE audit_events (
    id SERIAL PRIMARY KEY,
    event_id VARCHAR(100) UNIQUE NOT NULL,
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    kind VARCHAR(50) NOT NULL, -- invoke, job_start, job_cancel, user_login, etc.
    actor_type VARCHAR(20), -- user, system, agent
    actor_id VARCHAR(100),
    target_type VARCHAR(50), -- function, job, user, game
    target_id VARCHAR(100),
    action VARCHAR(50),
    resource VARCHAR(200),
    metadata JSONB,
    client_ip INET,
    user_agent TEXT,
    prev_hash VARCHAR(64), -- 链式哈希
    hash VARCHAR(64) NOT NULL -- 当前记录哈希
);

-- ================================
-- 5. 配置和系统管理
-- ================================

-- 系统配置表
CREATE TABLE system_configs (
    id SERIAL PRIMARY KEY,
    key VARCHAR(100) UNIQUE NOT NULL,
    value JSONB NOT NULL,
    description TEXT,
    category VARCHAR(50), -- security, feature, ui, etc.
    is_public BOOLEAN DEFAULT false, -- 是否可被前端读取
    updated_by INTEGER REFERENCES users(id),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- API 密钥管理
CREATE TABLE api_keys (
    id SERIAL PRIMARY KEY,
    key_id VARCHAR(100) UNIQUE NOT NULL,
    key_hash VARCHAR(255) NOT NULL,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    permissions JSONB, -- 密钥权限范围
    last_used_at TIMESTAMP WITH TIME ZONE,
    expires_at TIMESTAMP WITH TIME ZONE,
    status VARCHAR(20) DEFAULT 'active' CHECK (status IN ('active', 'inactive', 'revoked')),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- ================================
-- 6. 索引优化
-- ================================

-- 用户相关索引
CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_status ON users(status);
CREATE INDEX idx_user_roles_user_id ON user_roles(user_id);
CREATE INDEX idx_user_roles_role_id ON user_roles(role_id);

-- 游戏相关索引
CREATE INDEX idx_games_game_id ON games(game_id);
CREATE INDEX idx_games_status ON games(status);
CREATE INDEX idx_game_environments_game_id ON game_environments(game_id);
CREATE INDEX idx_game_admins_game_id ON game_admins(game_id);
CREATE INDEX idx_game_admins_user_id ON game_admins(user_id);

-- Agent 和函数索引
CREATE INDEX idx_agents_game_id_env ON agents(game_id, env_name);
CREATE INDEX idx_agents_status ON agents(status);
CREATE INDEX idx_agents_last_heartbeat ON agents(last_heartbeat);
CREATE INDEX idx_functions_game_id_env ON functions(game_id, env_name);
CREATE INDEX idx_functions_function_id ON functions(function_id);
CREATE INDEX idx_functions_agent_id ON functions(agent_id);

-- Job 相关索引
CREATE INDEX idx_jobs_job_id ON jobs(job_id);
CREATE INDEX idx_jobs_function_id ON jobs(function_id);
CREATE INDEX idx_jobs_game_id_env ON jobs(game_id, env_name);
CREATE INDEX idx_jobs_user_id ON jobs(user_id);
CREATE INDEX idx_jobs_status ON jobs(status);
CREATE INDEX idx_jobs_started_at ON jobs(started_at);
CREATE INDEX idx_jobs_idempotency_key ON jobs(idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX idx_job_events_job_id ON job_events(job_id);
CREATE INDEX idx_job_events_timestamp ON job_events(timestamp);

-- 审计索引
CREATE INDEX idx_audit_events_timestamp ON audit_events(timestamp);
CREATE INDEX idx_audit_events_actor ON audit_events(actor_type, actor_id);
CREATE INDEX idx_audit_events_target ON audit_events(target_type, target_id);
CREATE INDEX idx_audit_events_kind ON audit_events(kind);

-- 系统配置索引
CREATE INDEX idx_system_configs_category ON system_configs(category);
CREATE INDEX idx_api_keys_user_id ON api_keys(user_id);
CREATE INDEX idx_api_keys_key_id ON api_keys(key_id);
CREATE INDEX idx_api_keys_status ON api_keys(status);

-- ================================
-- 7. 触发器和函数
-- ================================

-- 更新 updated_at 触发器函数
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- 为需要的表添加 updated_at 触发器
CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_roles_updated_at BEFORE UPDATE ON roles
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_games_updated_at BEFORE UPDATE ON games
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_system_configs_updated_at BEFORE UPDATE ON system_configs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();