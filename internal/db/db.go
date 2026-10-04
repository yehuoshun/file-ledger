// Package db 数据库初始化：建库 + 建表（schema.sql 幂等执行）
package db

import (
	"context"
	"errors"
	"fmt"

	_ "embed"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"file-ledger/internal/logx"
)

var dlog = logx.New("db")

//go:embed schema.sql
var SchemaSQL string

// EnsureDatabase 确保目标库存在：连 admin 库检查，缺失则 CREATE DATABASE。
func EnsureDatabase(ctx context.Context, adminConnString, dbname string) error {
	conn, err := pgx.Connect(ctx, adminConnString)
	if err != nil {
		return fmt.Errorf("连接 admin 库失败: %w", err)
	}
	defer func() {
		if err := conn.Close(ctx); err != nil {
			dlog.Warn("conn.close", "err", err)
		}
	}()

	var exists bool
	if err := conn.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)", dbname,
	).Scan(&exists); err != nil {
		return fmt.Errorf("检查数据库是否存在失败: %w", err)
	}
	if exists {
		return nil
	}

	// CREATE DATABASE 不能在事务/预处理语句中执行，用 Exec 直跑
	_, err = conn.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s", pgx.Identifier{dbname}.Sanitize()))
	if err != nil {
		var pgErr *pgconn.PgError
		// 并发创建时可能撞 duplicate_database(42P04)，视为已存在
		if errors.As(err, &pgErr) && pgErr.Code == "42P04" {
			return nil
		}
		return fmt.Errorf("创建数据库 %s 失败: %w", dbname, err)
	}
	dlog.Info("database.created", "dbname", dbname)
	return nil
}

// EnsureSchema 执行 schema.sql（幂等），确保表/函数/触发器齐全。
func EnsureSchema(ctx context.Context, conn *pgx.Conn) error {
	if _, err := conn.Exec(ctx, SchemaSQL); err != nil {
		return fmt.Errorf("初始化表结构失败: %w", err)
	}
	return nil
}
