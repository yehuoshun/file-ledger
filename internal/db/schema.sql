-- file-ledger 台账 schema（应用启动时自动执行，幂等可重跑）
-- 适用版本: PostgreSQL 15+

-- UUID v7 生成函数（纯 PL/pgSQL，PG 15+ 可用）
-- 格式：tttttttt-tttt-Vxxx-xxxx-xxxxxxxxxxxx  t=时间戳(ms) V=7 x=随机
-- 参考：https://www.rfc-editor.org/rfc/rfc9562
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE OR REPLACE FUNCTION uuid_generate_v7()
RETURNS uuid
LANGUAGE plpgsql AS $$
DECLARE
    ms      bigint;
    rand    bytea;
    hex_ms  text;
BEGIN
    ms := (EXTRACT(EPOCH FROM clock_timestamp()) * 1000)::bigint;
    rand := gen_random_bytes(10);
    hex_ms := lpad(to_hex(ms), 12, '0');

    RETURN (
        substr(hex_ms, 1, 8) || '-' ||
        substr(hex_ms, 9, 4) || '-7' ||
        encode(substr(rand, 1, 2), 'hex') || '-' ||
        '8' || encode(substr(rand, 3, 1), 'hex') || '-' ||
        encode(substr(rand, 4, 6), 'hex')
    )::uuid;
END;
$$;

-- registry 台账表（字段与前端 web/src/views/Cloud.vue 对齐）
-- paths: jsonb 数组 [{type: local|mount|virtual, path: string}]，多备份路径
-- tags:  jsonb 字符串数组
CREATE TABLE IF NOT EXISTS registry (
    id          uuid        NOT NULL DEFAULT uuid_generate_v7(),
    alias       text        NOT NULL,
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

CREATE INDEX IF NOT EXISTS idx_registry_alias ON registry (alias);
CREATE INDEX IF NOT EXISTS idx_registry_tags ON registry USING gin (tags);

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
