-- file-ledger 台账数据库初始化
-- 适用版本: PostgreSQL 15+
-- 连接串: postgres://127.0.0.1:5432/file_ledger?sslmode=disable

-- 建库（需在 psql 或 pgAdmin 中手动执行，或者由应用初始化时自动运行）
-- CREATE DATABASE file_ledger;

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

-- registry 类型约束
CREATE TYPE entry_type AS ENUM ('local', 'mount', 'virtual');

CREATE TABLE registry (
    id          uuid        NOT NULL DEFAULT uuid_generate_v7(),
    alias       text        NOT NULL,
    type        entry_type  NOT NULL,
    root        text        NULL,       -- local/mount 必填
    rel_path    text        NULL,       -- virtual 必填
    jump_url    text        NULL,       -- virtual 可选
    note        text        NULL DEFAULT '',
    tags        jsonb       NULL DEFAULT '[]'::jsonb,
    read_only   boolean     NOT NULL DEFAULT false,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT registry_pk PRIMARY KEY (id),
    CONSTRAINT registry_alias_unique UNIQUE (alias),

    -- 类型约束检查
    CONSTRAINT chk_local_root CHECK (
        type = 'local' AND root IS NOT NULL
        OR type = 'mount' AND root IS NOT NULL
        OR type = 'virtual'
    ),
    CONSTRAINT chk_virtual_rel CHECK (
        type = 'virtual' AND rel_path IS NOT NULL
        OR type != 'virtual'
    )
);

-- 索引
CREATE INDEX idx_registry_alias ON registry (alias);
CREATE INDEX idx_registry_type ON registry (type);
CREATE INDEX idx_registry_tags ON registry USING gin (tags);

-- 自动更新 updated_at
CREATE OR REPLACE FUNCTION trigger_set_updated_at()
RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_registry_set_updated_at
    BEFORE UPDATE ON registry
    FOR EACH ROW
    EXECUTE FUNCTION trigger_set_updated_at();