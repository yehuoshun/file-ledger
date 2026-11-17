package tag

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"file-ledger/internal/db"
	"file-ledger/internal/logx"
)

var slog = logx.New("tag")

// Store 标签字典存储：并发安全的连接包装（pgx.Conn 本身不支持并发，见 internal/db/conn.go）
type Store struct {
	conn *db.Conn
}

// NewStore 创建标签存储
func NewStore(conn *db.Conn) *Store {
	return &Store{conn: conn}
}

const tagCols = "id, name, bg, fg, created_at"

// sqlRenameTagInRegistry 把 registry.tags 数组里的 $1 替换为 $2（纯 jsonb 数组操作，不把整行读进内存）
const sqlRenameTagInRegistry = `
UPDATE registry r
SET tags = (
    SELECT COALESCE(
               jsonb_agg(CASE WHEN elem = to_jsonb($1::text) THEN to_jsonb($2::text) ELSE elem END
                         ORDER BY ord),
               '[]'::jsonb)
    FROM jsonb_array_elements(r.tags) WITH ORDINALITY AS t(elem, ord)
)
WHERE r.tags @> jsonb_build_array($1::text)`

// sqlRemoveTagFromRegistry 把 $1 从 registry.tags 数组里剔除（纯 jsonb 数组操作）
const sqlRemoveTagFromRegistry = `
UPDATE registry r
SET tags = (
    SELECT COALESCE(jsonb_agg(elem ORDER BY ord), '[]'::jsonb)
    FROM jsonb_array_elements(r.tags) WITH ORDINALITY AS t(elem, ord)
    WHERE elem <> to_jsonb($1::text)
)
WHERE r.tags @> jsonb_build_array($1::text)`

// scanTag 把查询行扫成 Tag
func scanTag(row pgx.Row) (*Tag, error) {
	var (
		t         Tag
		createdAt time.Time
	)
	if err := row.Scan(&t.ID, &t.Name, &t.BG, &t.FG, &createdAt); err != nil {
		return nil, err
	}
	// timestamptz 二进制协议必须扫进 time.Time，再转字符串输出
	t.CreatedAt = createdAt.Format(time.RFC3339)
	return &t, nil
}

// List 标签全量，按 name 升序
func (s *Store) List(ctx context.Context) ([]Tag, error) {
	rows, err := s.conn.Query(ctx, "SELECT "+tagCols+" FROM tag ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("查询标签失败: %w", err)
	}
	defer rows.Close()

	out := make([]Tag, 0)
	for rows.Next() {
		t, err := scanTag(rows)
		if err != nil {
			return nil, fmt.Errorf("扫描标签失败: %w", err)
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

// likeEscaper 转义 LIKE/ILIKE 通配符（PG 默认转义符是反斜杠），让用户输入的百分号/下划线/反斜杠按字面匹配
var likeEscaper = strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`)

// escapeLike 把搜索词里的 LIKE 通配符转义成字面量
func escapeLike(q string) string { return likeEscaper.Replace(q) }

// ListPaged 按名字包含匹配（大小写不敏感）分页查询标签，返回当页数据与匹配总数。
// q 为空表示不过滤（LIKE 模式退化为 "%%"，命中全部）；排序按 name 升序，与 List 一致。
// limit/offset 的合法性由 api 层校验。
func (s *Store) ListPaged(ctx context.Context, q string, limit, offset int) ([]Tag, int, error) {
	pattern := escapeLike(q)
	var total int
	if err := s.conn.QueryRow(ctx,
		"SELECT count(*) FROM tag WHERE name ILIKE '%'||$1||'%'", pattern).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("统计标签数失败: %w", err)
	}

	rows, err := s.conn.Query(ctx,
		"SELECT "+tagCols+" FROM tag WHERE name ILIKE '%'||$1||'%' ORDER BY name LIMIT $2 OFFSET $3",
		pattern, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("查询标签失败: %w", err)
	}
	defer rows.Close()

	out := make([]Tag, 0)
	for rows.Next() {
		t, err := scanTag(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("扫描标签失败: %w", err)
		}
		out = append(out, *t)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// ListByNames 按名字批量精确取标签（名单由 api 层 trim 去空）；名字不存在直接忽略，
// 排序按 name 升序。返回条数即命中数。
func (s *Store) ListByNames(ctx context.Context, names []string) ([]Tag, error) {
	out := make([]Tag, 0, len(names))
	if len(names) == 0 {
		return out, nil
	}

	rows, err := s.conn.Query(ctx,
		"SELECT "+tagCols+" FROM tag WHERE name = ANY($1) ORDER BY name", names)
	if err != nil {
		return nil, fmt.Errorf("查询标签失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		t, err := scanTag(rows)
		if err != nil {
			return nil, fmt.Errorf("扫描标签失败: %w", err)
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

// Create 新增标签；name 必填且唯一；bg/fg 留空时按调色板分配（tag 表当前行数 % 4）
func (s *Store) Create(ctx context.Context, t *Tag) (*Tag, error) {
	name, err := NormalizeName(t.Name)
	if err != nil {
		return nil, err
	}
	t.Name = name
	t.BG = strings.TrimSpace(t.BG)
	t.FG = strings.TrimSpace(t.FG)
	if t.BG == "" || t.FG == "" {
		bg, fg, err := s.nextColors(ctx)
		if err != nil {
			return nil, err
		}
		if t.BG == "" {
			t.BG = bg
		}
		if t.FG == "" {
			t.FG = fg
		}
	}

	row := s.conn.QueryRow(ctx,
		`INSERT INTO tag (name, bg, fg) VALUES ($1, $2, $3) RETURNING `+tagCols,
		t.Name, t.BG, t.FG)
	created, err := scanTag(row)
	if err != nil {
		slog.Warn("create", "rid", logx.RIDFrom(ctx), "name", t.Name, "err", translateErr(err))
		return nil, translateErr(err)
	}
	slog.Info("create", "rid", logx.RIDFrom(ctx), "id", created.ID, "name", created.Name)
	return created, nil
}

// nextColors 按「tag 表当前行数 % 调色板长度」取一组配色
func (s *Store) nextColors(ctx context.Context) (string, string, error) {
	var n int
	if err := s.conn.QueryRow(ctx, "SELECT count(*) FROM tag").Scan(&n); err != nil {
		return "", "", fmt.Errorf("统计标签数失败: %w", err)
	}
	p := palette[n%len(palette)]
	return p[0], p[1], nil
}

// Update 改名/改色；改名时同一事务把 registry.tags 里的旧名替换为新名。
// bg/fg 留空表示沿用旧配色，避免只改名的请求把颜色清掉。
func (s *Store) Update(ctx context.Context, id string, t *Tag) (*Tag, error) {
	name, err := NormalizeName(t.Name)
	if err != nil {
		return nil, err
	}
	t.Name = name
	t.BG = strings.TrimSpace(t.BG)
	t.FG = strings.TrimSpace(t.FG)

	tx, err := s.conn.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("开启事务失败: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// 先取旧名与旧配色：改名要同步 registry.tags，空配色沿用旧值
	var oldName, oldBG, oldFG string
	if err := tx.QueryRow(ctx,
		"SELECT name, bg, fg FROM tag WHERE id = $1", id).Scan(&oldName, &oldBG, &oldFG); err != nil {
		return nil, translateErr(err)
	}
	if t.BG == "" {
		t.BG = oldBG
	}
	if t.FG == "" {
		t.FG = oldFG
	}

	row := tx.QueryRow(ctx,
		`UPDATE tag SET name = $1, bg = $2, fg = $3 WHERE id = $4 RETURNING `+tagCols,
		t.Name, t.BG, t.FG, id)
	updated, err := scanTag(row)
	if err != nil {
		slog.Warn("update", "rid", logx.RIDFrom(ctx), "id", id, "err", translateErr(err))
		return nil, translateErr(err)
	}

	// 改名：同一事务里把条目上的旧标签名换成新名
	if oldName != t.Name {
		if _, err := tx.Exec(ctx, sqlRenameTagInRegistry, oldName, t.Name); err != nil {
			return nil, fmt.Errorf("同步条目标签失败: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("提交事务失败: %w", err)
	}
	slog.Info("update", "rid", logx.RIDFrom(ctx), "id", updated.ID,
		"name", updated.Name, "old", oldName)
	return updated, nil
}

// Delete 删除标签；同一事务里把该名字从所有 registry.tags 中剔除；不存在返回 ErrNotFound
func (s *Store) Delete(ctx context.Context, id string) error {
	tx, err := s.conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var name string
	if err := tx.QueryRow(ctx, "SELECT name FROM tag WHERE id = $1", id).Scan(&name); err != nil {
		return translateErr(err)
	}
	res, err := tx.Exec(ctx, "DELETE FROM tag WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("删除标签失败: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}
	// 同一事务：把该名字从所有条目的 tags 里剔除
	if _, err := tx.Exec(ctx, sqlRemoveTagFromRegistry, name); err != nil {
		return fmt.Errorf("摘除条目标签失败: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}
	slog.Info("delete", "rid", logx.RIDFrom(ctx), "id", id, "name", name)
	return nil
}

// Ensure 批量登记缺失标签：trim、丢空、按首次出现去重、超长截断到 MaxNameRunes；
// 缺失的按调色板顺序（从 tag 表当前行数起）分配配色，ON CONFLICT DO NOTHING 保证幂等。
func (s *Store) Ensure(ctx context.Context, names []string) error {
	cleaned := normalizeNames(names)
	if len(cleaned) == 0 {
		return nil
	}

	// 已存在的名字一次查出，避免逐个 round trip
	rows, err := s.conn.Query(ctx, "SELECT name FROM tag WHERE name = ANY($1)", cleaned)
	if err != nil {
		return fmt.Errorf("查询已存在标签失败: %w", err)
	}
	existing := make(map[string]struct{}, len(cleaned))
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return fmt.Errorf("扫描标签名失败: %w", err)
		}
		existing[name] = struct{}{}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("读取标签名失败: %w", err)
	}

	missing := make([]string, 0, len(cleaned))
	for _, name := range cleaned {
		if _, ok := existing[name]; !ok {
			missing = append(missing, name)
		}
	}
	if len(missing) == 0 {
		return nil
	}

	var n int
	if err := s.conn.QueryRow(ctx, "SELECT count(*) FROM tag").Scan(&n); err != nil {
		return fmt.Errorf("统计标签数失败: %w", err)
	}
	bgs := make([]string, len(missing))
	fgs := make([]string, len(missing))
	for i := range missing {
		p := palette[(n+i)%len(palette)]
		bgs[i], fgs[i] = p[0], p[1]
	}

	res, err := s.conn.Exec(ctx,
		`INSERT INTO tag (name, bg, fg)
		 SELECT * FROM unnest($1::text[], $2::text[], $3::text[])
		 ON CONFLICT (name) DO NOTHING`,
		missing, bgs, fgs)
	if err != nil {
		return fmt.Errorf("登记标签失败: %w", err)
	}
	slog.Info("ensure", "rid", logx.RIDFrom(ctx),
		"requested", len(cleaned), "added", res.RowsAffected())
	return nil
}

// normalizeNames trim、丢空、按首次出现去重、超长截断（批量登记用，不因脏数据中断保存）
func normalizeNames(names []string) []string {
	out := make([]string, 0, len(names))
	seen := make(map[string]struct{}, len(names))
	for _, raw := range names {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		name = TruncateName(name)
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}

// translateErr 把 pgx 错误映射为业务错误：
// 唯一约束冲突(23505) → ErrNameTaken；无行 → ErrNotFound
func translateErr(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrNameTaken
	}
	return err
}
