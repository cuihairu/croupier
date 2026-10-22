-- capture-agent（CDC）前置：示例表 + 预建 publication（pgoutput）。
-- postgres 官方镜像仅在首次初始化空卷时执行 initdb 脚本；存量卷升级到带
-- capture-agent 的 compose 版本时，请手工执行同款 SQL（见
-- docs/operations/deploy-docker.md「capture agent」小节）。
CREATE TABLE IF NOT EXISTS public.capture_demo (
    id         int PRIMARY KEY,
    note       text,
    updated_at timestamptz NOT NULL DEFAULT now()
);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_publication WHERE pubname = 'croupier_capture') THEN
        CREATE PUBLICATION croupier_capture FOR TABLE public.capture_demo;
    END IF;
END $$;
