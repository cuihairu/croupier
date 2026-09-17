-- migrate-categories-to-menus.sql
-- 菜单管理系统 T-M5：把 page_specs 的 category_* 数据迁移到 menu_items，
-- 并把页面挂载到对应菜单（docs/design/menu-management.md §6）。
--
-- 用法（PostgreSQL，multiGame 模式需对每个游戏库各执行一次）：
--   psql -d <database> -f scripts/migrate-categories-to-menus.sql
-- 单库模式（multiGame: false）对唯一业务库执行一次即可。
--
-- 特性：
--   * 幂等：可重复执行，重跑不产生重复菜单、不覆盖已有 menu_id。
--   * 同 (game_id, env, category_key) 下 labels 不一致时取最近更新页面的
--     labels 作为菜单名（本次迁移要消灭的正是 labels 漂移）。
--   * 语句仅使用 CURRENT_TIMESTAMP/TRUE/标量子查询等可移植写法，
--     PostgreSQL / MySQL / SQLite 均可执行。
--   * 本脚本不删除旧列（category_key/category_labels_json），
--     旧列清理属于 T-M8 的删除任务。
--
-- 执行结束会输出两组校验结果：
--   * unmapped_categorized_pages 必须为 0（所有分类页面均已挂载菜单）
--   * 每个菜单下的页面数（人工核对与原分类一致）

-- 1. 从现有页面提取唯一分类，创建菜单
INSERT INTO menu_items (game_id, env, menu_key, labels, sort_order, is_visible, created_at, updated_at)
SELECT
    s.game_id,
    s.env,
    s.category_key,
    (
        SELECT p2.category_labels_json
        FROM page_specs p2
        WHERE p2.game_id = s.game_id
          AND p2.env = s.env
          AND p2.category_key = s.category_key
          AND p2.deleted_at IS NULL
        ORDER BY p2.updated_at DESC
        LIMIT 1
    ) AS labels,
    MIN(s.category_order) AS sort_order,
    TRUE AS is_visible,
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP
FROM page_specs s
WHERE s.deleted_at IS NULL
  AND COALESCE(s.category_key, '') <> ''
  AND NOT EXISTS (
      SELECT 1 FROM menu_items m
      WHERE m.game_id = s.game_id
        AND m.env = s.env
        AND m.menu_key = s.category_key
        AND m.deleted_at IS NULL
  )
GROUP BY s.game_id, s.env, s.category_key;

-- 2. 关联页面到菜单
UPDATE page_specs
SET menu_id = (
    SELECT m.id
    FROM menu_items m
    WHERE m.game_id = page_specs.game_id
      AND m.env = page_specs.env
      AND m.menu_key = page_specs.category_key
      AND m.deleted_at IS NULL
    LIMIT 1
)
WHERE deleted_at IS NULL
  AND COALESCE(category_key, '') <> ''
  AND menu_id IS NULL;

-- 3. 校验一：仍有分类但未挂载菜单的页面数（必须为 0）
SELECT COUNT(*) AS unmapped_categorized_pages
FROM page_specs
WHERE deleted_at IS NULL
  AND COALESCE(category_key, '') <> ''
  AND menu_id IS NULL;

-- 4. 校验二：每个菜单下的页面数（与迁移前分类计数应一致）
SELECT m.game_id, m.env, m.menu_key, COUNT(p.id) AS page_count
FROM menu_items m
LEFT JOIN page_specs p
    ON p.menu_id = m.id
   AND p.deleted_at IS NULL
WHERE m.deleted_at IS NULL
GROUP BY m.game_id, m.env, m.menu_key
ORDER BY m.game_id, m.env, m.menu_key;
