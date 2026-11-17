package db

import (
	"context"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Conn 给单条 pgx 连接加互斥：pgx.Conn 不是并发安全的，两个请求同时查询会直接报
// "conn busy / failed to deallocate cached statement(s)"。
//
// 个人单机工具用一条连接足够，这里把访问排队，等价于「连接池大小 1」。
// 之所以不用官方的 pgxpool：它依赖 github.com/jackc/puddle（本机拉不到，go proxy 不通），
// 单连接串行化对本地单用户场景够用，且零新增依赖。
type Conn struct {
	mu sync.Mutex
	c  *pgx.Conn
}

// Wrap 把已连接的 pgx.Conn 包成并发安全的 Conn
func Wrap(c *pgx.Conn) *Conn { return &Conn{c: c} }

// Raw 返回底层连接：只给启动期（建库建表、关闭）用，别在请求路径上用，否则绕过串行化
func (c *Conn) Raw() *pgx.Conn { return c.c }

// Query 查询多行；锁在 rows 迭代结束或 Close 时释放
func (c *Conn) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	c.mu.Lock()
	rows, err := c.c.Query(ctx, sql, args...)
	if err != nil {
		c.mu.Unlock()
		return nil, err
	}
	return &lockedRows{Rows: rows, unlock: c.mu.Unlock}, nil
}

// QueryRow 查询单行；锁在 Scan 后释放（调用方必须 Scan，否则锁不会释放）
func (c *Conn) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	c.mu.Lock()
	return &lockedRow{Row: c.c.QueryRow(ctx, sql, args...), unlock: c.mu.Unlock}
}

// Exec 执行语句
func (c *Conn) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.c.Exec(ctx, sql, args...)
}

// Begin 开事务；锁在 Commit/Rollback 时释放
func (c *Conn) Begin(ctx context.Context) (pgx.Tx, error) {
	c.mu.Lock()
	tx, err := c.c.Begin(ctx)
	if err != nil {
		c.mu.Unlock()
		return nil, err
	}
	return &lockedTx{Tx: tx, unlock: c.mu.Unlock}, nil
}

// lockedRows 迭代结束（Next 返回 false）或 Close 时放锁；重复放锁由 once 兜住
type lockedRows struct {
	pgx.Rows
	once   sync.Once
	unlock func()
}

func (r *lockedRows) Next() bool {
	if r.Rows.Next() {
		return true
	}
	r.release()
	return false
}

func (r *lockedRows) Close() {
	r.Rows.Close()
	r.release()
}

func (r *lockedRows) release() {
	r.once.Do(r.unlock)
}

// lockedRow Scan 后放锁
type lockedRow struct {
	pgx.Row
	once   sync.Once
	unlock func()
}

func (r *lockedRow) Scan(dest ...any) error {
	defer r.release()
	return r.Row.Scan(dest...)
}

func (r *lockedRow) release() {
	r.once.Do(r.unlock)
}

// lockedTx 提交或回滚后放锁（store 里习惯 defer Rollback，Commit 之后再 Rollback 是空操作，不会重复放锁）
type lockedTx struct {
	pgx.Tx
	once   sync.Once
	unlock func()
}

func (t *lockedTx) Commit(ctx context.Context) error {
	defer t.release()
	return t.Tx.Commit(ctx)
}

func (t *lockedTx) Rollback(ctx context.Context) error {
	defer t.release()
	return t.Tx.Rollback(ctx)
}

func (t *lockedTx) release() {
	t.once.Do(t.unlock)
}
