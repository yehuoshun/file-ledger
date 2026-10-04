-- file-ledger 台账 schema（应用启动时自动执行，幂等可重跑）
-- 适用版本: PostgreSQL 15+

-- UUID v7 生成函数（纯 PL/pgSQL，PG 15+ 可用）
-- 格式：tttttttt-tttt-7xxx-8xxx-xxxxxxxxxxxx（RFC 9562：8-4-4-4-12）
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE OR REPLACE FUNCTION uuid_generate_v7()
RETURNS uuid
LANGUAGE plpgsql AS $$
DECLARE
    ms      bigint;
    hex     text;   -- 20 hex 字符（10 随机字节）
    hex_ms  text;
BEGIN
    ms := (EXTRACT(EPOCH FROM clock_timestamp()) * 1000)::bigint;
    hex := encode(gen_random_bytes(10), 'hex');
    hex_ms := lpad(to_hex(ms), 12, '0');

    RETURN (
        substr(hex_ms, 1, 8) || '-' ||   -- 时间戳高 32 位
        substr(hex_ms, 9, 4) || '-7' ||  -- 时间戳低 16 位 + 版本号
        substr(hex, 1, 3) || '-8' ||     -- 12 位随机 + 变体
        substr(hex, 4, 3) || '-' ||
        substr(hex, 7, 12)               -- 48 位随机
    )::uuid;
END;
$$;

-- registry 台账表（字段与前端 web/src/views/Cloud.vue 对齐）
-- paths: jsonb 数组 [{type: local|mount|virtual, path: string}]，多备份路径
-- tags:  jsonb 字符串数组
CREATE TABLE IF NOT EXISTS registry (
    id          uuid        NOT NULL DEFAULT uuid_generate_v7(),
    alias       text        NOT NULL,
    parent_id   uuid        NULL REFERENCES registry(id) ON DELETE CASCADE,
    is_dir      boolean     NOT NULL DEFAULT true,
    size        text        NOT NULL DEFAULT '',
    note        text        NOT NULL DEFAULT '',
    tags        jsonb       NOT NULL DEFAULT '[]'::jsonb,
    paths       jsonb       NOT NULL DEFAULT '[]'::jsonb,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT registry_pk PRIMARY KEY (id),
    CONSTRAINT registry_alias_unique UNIQUE (alias)
);

-- 幂等迁移：老库补 parent_id（Windows 式文件夹树，根级为 NULL）
ALTER TABLE registry ADD COLUMN IF NOT EXISTS parent_id uuid NULL REFERENCES registry(id) ON DELETE CASCADE;

CREATE INDEX IF NOT EXISTS idx_registry_alias ON registry (alias);
CREATE INDEX IF NOT EXISTS idx_registry_tags ON registry USING gin (tags);
CREATE INDEX IF NOT EXISTS idx_registry_parent ON registry (parent_id);

-- 字段注释（COMMENT ON 天然幂等，可重复执行）
COMMENT ON TABLE registry IS '台账条目表（Windows 式文件夹树：parent_id 自引用）';
COMMENT ON COLUMN registry.id IS '主键（UUID v7，服务端生成）';
COMMENT ON COLUMN registry.alias IS '别名/名字，唯一';
COMMENT ON COLUMN registry.parent_id IS '父条目 id（虚拟文件夹层级，NULL=顶层；级联删除）';
COMMENT ON COLUMN registry.is_dir IS '是否文件夹（false=文件）';
COMMENT ON COLUMN registry.size IS '大小（人类可读文本，如 1.2 GB）';
COMMENT ON COLUMN registry.note IS '描述/备注';
COMMENT ON COLUMN registry.tags IS '标签数组（jsonb）';
COMMENT ON COLUMN registry.paths IS '路径数组（jsonb）：[{type: local|mount|virtual, path}]，支持多备份';
COMMENT ON COLUMN registry.created_at IS '创建时间（自动）';
COMMENT ON COLUMN registry.updated_at IS '更新时间（自动维护）';

-- 自动更新 updated_at
CREATE OR REPLACE FUNCTION trigger_set_updated_at()
RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$;

-- PG 无 CREATE TRIGGER IF NOT EXISTS，用 DO 块判断
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_trigger WHERE tgname = 'trg_registry_set_updated_at'
    ) THEN
        CREATE TRIGGER trg_registry_set_updated_at
            BEFORE UPDATE ON registry
            FOR EACH ROW
            EXECUTE FUNCTION trigger_set_updated_at();
    END IF;
END $$;
