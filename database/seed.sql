-- Croupier 初始数据
-- 包含系统角色、权限、默认管理员和示例游戏

-- ================================
-- 1. 系统角色和权限
-- ================================

-- 插入系统角色
INSERT INTO roles (name, display_name, description, is_system) VALUES
    ('super_admin', '超级管理员', '系统最高权限，可管理所有资源', true),
    ('platform_admin', '平台管理员', '可管理用户、游戏和系统配置', true),
    ('game_admin', '游戏管理员', '可管理特定游戏的所有资源', true),
    ('developer', '开发者', '可调用函数和查看日志', true),
    ('viewer', '观察者', '只读权限，可查看资源状态', true);

-- 插入权限定义
INSERT INTO permissions (name, display_name, description, resource_type, action) VALUES
    -- 系统管理权限
    ('system:manage', '系统管理', '管理系统配置和全局设置', 'system', 'manage'),
    ('system:audit', '审计查看', '查看系统审计日志', 'system', 'read'),

    -- 用户管理权限
    ('user:create', '创建用户', '创建新用户账号', 'user', 'create'),
    ('user:read', '查看用户', '查看用户信息', 'user', 'read'),
    ('user:update', '更新用户', '修改用户信息', 'user', 'update'),
    ('user:delete', '删除用户', '删除用户账号', 'user', 'delete'),
    ('user:manage_roles', '角色管理', '分配和撤销用户角色', 'user', 'manage'),

    -- 游戏管理权限
    ('game:create', '创建游戏', '注册新游戏', 'game', 'create'),
    ('game:read', '查看游戏', '查看游戏信息', 'game', 'read'),
    ('game:update', '更新游戏', '修改游戏配置', 'game', 'update'),
    ('game:delete', '删除游戏', '删除游戏', 'game', 'delete'),
    ('game:manage_envs', '环境管理', '管理游戏环境', 'game', 'manage'),
    ('game:manage_admins', '管理员管理', '分配游戏管理员', 'game', 'manage'),

    -- 函数调用权限
    ('function:invoke', '调用函数', '同步调用函数', 'function', 'invoke'),
    ('function:job_start', '启动任务', '启动异步任务', 'function', 'create'),
    ('function:job_cancel', '取消任务', '取消运行中的任务', 'function', 'delete'),
    ('function:read', '查看函数', '查看函数列表和状态', 'function', 'read'),

    -- Agent 管理权限
    ('agent:read', '查看代理', '查看 Agent 状态', 'agent', 'read'),
    ('agent:manage', '管理代理', '管理 Agent 注册和配置', 'agent', 'manage'),

    -- API 密钥权限
    ('apikey:create', '创建密钥', '创建 API 密钥', 'apikey', 'create'),
    ('apikey:read', '查看密钥', '查看 API 密钥', 'apikey', 'read'),
    ('apikey:revoke', '撤销密钥', '撤销 API 密钥', 'apikey', 'delete');

-- 配置角色权限
-- 超级管理员：所有权限
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'super_admin';

-- 平台管理员：除系统管理外的所有权限
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'platform_admin' AND p.name != 'system:manage';

-- 游戏管理员：游戏相关权限
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'game_admin' AND p.name IN (
    'game:read', 'game:update', 'game:manage_envs', 'game:manage_admins',
    'function:invoke', 'function:job_start', 'function:job_cancel', 'function:read',
    'agent:read', 'agent:manage'
);

-- 开发者：函数调用和查看权限
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'developer' AND p.name IN (
    'game:read', 'function:invoke', 'function:job_start', 'function:job_cancel',
    'function:read', 'agent:read'
);

-- 观察者：只读权限
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'viewer' AND p.name IN (
    'game:read', 'function:read', 'agent:read', 'user:read'
);

-- ================================
-- 2. 默认管理员账号
-- ================================

-- 创建默认管理员 (密码: admin123)
-- 注意：实际部署时应该修改默认密码
INSERT INTO users (username, email, password_hash, salt, display_name, status) VALUES (
    'admin',
    'admin@croupier.local',
    '$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi', -- admin123
    'default_salt_should_be_random',
    '系统管理员',
    'active'
);

-- 分配超级管理员角色
INSERT INTO user_roles (user_id, role_id, granted_by)
SELECT u.id, r.id, u.id
FROM users u, roles r
WHERE u.username = 'admin' AND r.name = 'super_admin';

-- ================================
-- 3. 示例游戏数据
-- ================================

-- 插入示例游戏
INSERT INTO games (game_id, name, display_name, description, icon_url, status, tags, created_by) VALUES
    ('mmorpg', 'MMORPG Game', '大型多人在线角色扮演游戏', '一个完整的MMORPG游戏系统，支持角色、装备、副本等功能', 'https://example.com/icons/mmorpg.png', 'active', '["mmorpg", "rpg", "multiplayer"]', 1),
    ('fps_shooter', 'FPS Shooter', '第一人称射击游戏', '快节奏的射击游戏，支持多人对战', 'https://example.com/icons/fps.png', 'active', '["fps", "shooter", "pvp"]', 1),
    ('puzzle_game', 'Puzzle Game', '益智解谜游戏', '经典的三消益智游戏', 'https://example.com/icons/puzzle.png', 'active', '["puzzle", "casual"]', 1);

-- 为每个游戏创建环境
INSERT INTO game_environments (game_id, env_name, display_name, description, config) VALUES
    -- MMORPG 环境
    (1, 'dev', '开发环境', 'MMORPG开发测试环境', '{"debug": true, "log_level": "debug"}'),
    (1, 'staging', '预发布环境', 'MMORPG预发布测试环境', '{"debug": false, "log_level": "info"}'),
    (1, 'production', '生产环境', 'MMORPG生产环境', '{"debug": false, "log_level": "warn"}'),

    -- FPS 环境
    (2, 'dev', '开发环境', 'FPS开发测试环境', '{"debug": true, "max_players": 8}'),
    (2, 'production', '生产环境', 'FPS生产环境', '{"debug": false, "max_players": 32}'),

    -- Puzzle 环境
    (3, 'dev', '开发环境', 'Puzzle开发环境', '{"debug": true}'),
    (3, 'production', '生产环境', 'Puzzle生产环境', '{"debug": false}');

-- 设置默认管理员为所有游戏的拥有者
INSERT INTO game_admins (game_id, user_id, role, granted_by)
SELECT g.id, 1, 'owner', 1
FROM games g;

-- ================================
-- 4. 系统配置
-- ================================

INSERT INTO system_configs (key, value, description, category, is_public) VALUES
    ('auth.jwt_secret', '"your-secret-key-here"', 'JWT 签名密钥', 'security', false),
    ('auth.jwt_expiry', '86400', 'JWT 过期时间（秒）', 'security', false),
    ('auth.session_timeout', '7200', '会话超时时间（秒）', 'security', false),
    ('auth.max_login_attempts', '5', '最大登录尝试次数', 'security', false),
    ('auth.lockout_duration', '900', '账号锁定时间（秒）', 'security', false),

    ('agent.heartbeat_interval', '30', 'Agent 心跳间隔（秒）', 'feature', false),
    ('agent.registration_timeout', '300', 'Agent 注册超时（秒）', 'feature', false),
    ('agent.max_idle_time', '600', 'Agent 最大空闲时间（秒）', 'feature', false),

    ('job.default_timeout', '300', '任务默认超时时间（秒）', 'feature', false),
    ('job.max_concurrent', '100', '最大并发任务数', 'feature', false),
    ('job.result_retention', '2592000', '任务结果保留时间（秒，30天）', 'feature', false),

    ('ui.page_size', '20', '分页大小', 'ui', true),
    ('ui.max_page_size', '100', '最大分页大小', 'ui', true),
    ('ui.default_theme', '"light"', '默认主题', 'ui', true),
    ('ui.enable_dark_mode', 'true', '启用暗色模式', 'ui', true),

    ('audit.retention_days', '365', '审计日志保留天数', 'feature', false),
    ('audit.log_level', '"info"', '审计日志级别', 'feature', false),
    ('audit.hash_algorithm', '"sha256"', '审计链哈希算法', 'feature', false);

-- ================================
-- 5. 开发用测试数据
-- ================================

-- 创建测试用户
INSERT INTO users (username, email, password_hash, salt, display_name, status) VALUES
    ('developer1', 'dev1@example.com', '$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi', 'salt1', '开发者1', 'active'),
    ('developer2', 'dev2@example.com', '$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi', 'salt2', '开发者2', 'active'),
    ('gameadmin1', 'gadmin1@example.com', '$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi', 'salt3', '游戏管理员1', 'active');

-- 分配角色
INSERT INTO user_roles (user_id, role_id, granted_by) VALUES
    (2, (SELECT id FROM roles WHERE name = 'developer'), 1),
    (3, (SELECT id FROM roles WHERE name = 'developer'), 1),
    (4, (SELECT id FROM roles WHERE name = 'game_admin'), 1);

-- 设置游戏管理员权限
INSERT INTO game_admins (game_id, user_id, role, granted_by) VALUES
    (1, 4, 'admin', 1), -- gameadmin1 管理 MMORPG
    (2, 4, 'admin', 1); -- gameadmin1 管理 FPS

-- 创建示例 API 密钥
INSERT INTO api_keys (key_id, key_hash, user_id, name, description, permissions, expires_at) VALUES
    ('api_key_dev1', '$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi', 2, '开发测试密钥', '用于开发环境测试', '{"games": ["mmorpg"], "actions": ["function:invoke", "function:read"]}', CURRENT_TIMESTAMP + INTERVAL '1 year'),
    ('api_key_admin', '$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi', 1, '管理员密钥', '系统管理专用', '{"actions": ["*"]}', CURRENT_TIMESTAMP + INTERVAL '1 year');