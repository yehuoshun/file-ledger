package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"file-ledger/internal/db"
	"file-ledger/internal/logx"
)

var slog = logx.New("registry")

// Store 台账存储：并发安全的连接包装（pgx.Conn 本身不支持并发，见 internal/db/conn.go）
type Store struct {
	conn *db.Conn
}

func NewStore(conn *db.Conn) *Store {
	return &Store{conn: conn}
}

const entryCols = "id, alias, parent_id, is_dir, size, note, tags, paths, created_at, updated_at"

// scanEntry 把查询行扫成 Entry
func scanEntry(row pgx.Row) (*Entry, error) {
	var (
		e         Entry
		tagsRaw   []byte
		pathsRaw  []byte
		createdAt time.Time
		updatedAt time.Time
	)
	if err := row.Scan(&e.ID, &e.Alias, &e.ParentID, &e.IsDir, &e.Size, &e.Note,
		&tagsRaw, &pathsRaw, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	// timestamptz 二进制协议必须扫进 time.Time，再转字符串输出
	e.CreatedAt = createdAt.Format(time.RFC3339)
	e.UpdatedAt = updatedAt.Format(time.RFC3339)
	if err := json.Unmarshal(tagsRaw, &e.Tags); err != nil {
		return nil, fmt.Errorf("解析 tags 失败: %w", err)
	}
	if err := json.Unmarshal(pathsRaw, &e.Paths); err != nil {
		return nil, fmt.Errorf("解析 paths 失败: %w", err)
	}
	return &e, nil
}

// List 台账全量（前端组树：Windows 式层级），按别名排序
func (s *Store) List(ctx context.Context) ([]Entry, error) {
	rows, err := s.conn.Query(ctx,
		"SELECT "+entryCols+" FROM registry ORDER BY alias")
	if err != nil {
		return nil, fmt.Errorf("查询台账失败: %w", err)
	}
	defer rows.Close()

	out := make([]Entry, 0)
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, fmt.Errorf("扫描台账失败: %w", err)
		}
		out = append(out, *e)
	}
	return out, rows.Err()
}

// Create 新增条目；id 由 DB 端 uuid_generate_v7() 生成；父条目必须存在（虚拟文件夹层级）
func (s *Store) Create(ctx context.Context, e *Entry) (*Entry, error) {
	if err := s.checkParent(ctx, e.ParentID); err != nil {
		return nil, err
	}
	tags, err := marshalJSONB(e.Tags)
	if err != nil {
		return nil, err
	}
	paths, err := marshalJSONB(e.Paths)
	if err != nil {
		return nil, err
	}

	row := s.conn.QueryRow(ctx,
		`INSERT INTO registry (alias, parent_id, is_dir, size, note, tags, paths)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING `+entryCols,
		e.Alias, e.ParentID, e.IsDir, e.Size, e.Note, tags, paths)
	created, err := scanEntry(row)
	if err != nil {
		slog.Warn("create", "rid", logx.RIDFrom(ctx), "alias", e.Alias, "err", translateErr(err))
		return nil, translateErr(err)
	}
	slog.Info("create", "rid", logx.RIDFrom(ctx), "id", created.ID, "alias", created.Alias,
		"parent", nullable(created.ParentID), "paths", len(created.Paths))
	return created, nil
}

// Update 按 id 更新；返回更新后的条目；父条目必须存在
func (s *Store) Update(ctx context.Context, id string, e *Entry) (*Entry, error) {
	if err := s.checkParent(ctx, e.ParentID); err != nil {
		return nil, err
	}
	tags, err := marshalJSONB(e.Tags)
	if err != nil {
		return nil, err
	}
	paths, err := marshalJSONB(e.Paths)
	if err != nil {
		return nil, err
	}

	row := s.conn.QueryRow(ctx,
		`UPDATE registry SET alias=$1, parent_id=$2, is_dir=$3, size=$4, note=$5, tags=$6, paths=$7
		 WHERE id=$8
		 RETURNING `+entryCols,
		e.Alias, e.ParentID, e.IsDir, e.Size, e.Note, tags, paths, id)
	updated, err := scanEntry(row)
	if err != nil {
		slog.Warn("update", "rid", logx.RIDFrom(ctx), "id", id, "err", translateErr(err))
		return nil, translateErr(err)
	}
	slog.Info("update", "rid", logx.RIDFrom(ctx), "id", updated.ID, "alias", updated.Alias,
		"parent", nullable(updated.ParentID), "paths", len(updated.Paths))
	return updated, nil
}

// Delete 按 id 删除；条目不存在返回 ErrNotFound
func (s *Store) Delete(ctx context.Context, id string) error {
	tag, err := s.conn.Exec(ctx,
		"DELETE FROM registry WHERE id=$1", id)
	if err != nil {
		return fmt.Errorf("删除失败: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	slog.Info("delete", "rid", logx.RIDFrom(ctx), "id", id, "rows", tag.RowsAffected())
	return nil
}

// nullable 指针转可打印值（nil → 空串）
func nullable(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// checkParent 父条目存在性校验（parentId 为空=顶层，跳过）
func (s *Store) checkParent(ctx context.Context, parentID *string) error {
	if parentID == nil || *parentID == "" {
		return nil
	}
	var exists bool
	if err := s.conn.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM registry WHERE id = $1)", *parentID).Scan(&exists); err != nil {
		return fmt.Errorf("校验父条目失败: %w", err)
	}
	if !exists {
		return fmt.Errorf("父条目不存在")
	}
	return nil
}

// translateErr 把 pgx 错误映射为业务错误：
// 唯一约束冲突(23505) → ErrAliasTaken；无行 → ErrNotFound
func translateErr(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrAliasTaken
	}
	return err
}

// ListParams 台账分页查询参数（api 层解析后传入；limit/offset 的合法性由 api 层校验）
//   - ParentID：nil 或空串 = 顶层（parent_id IS NULL）；非空 = 该文件夹的直接子条目
//   - Q：空 = 不过滤；非空 = alias 或 note 包含匹配（大小写不敏感，通配符按字面）
//   - IsDir：nil = 不过滤；非 nil = 只看文件夹/只看文件
type ListParams struct {
	ParentID *string
	Q        string
	IsDir    *bool
	Limit    int
	Offset   int
}

// likeEscaper 转义 LIKE/ILIKE 通配符（PG 默认转义符是反斜杠），让用户输入的百分号/下划线/反斜杠按字面匹配。
// 与 internal/tag 的同名逻辑各自维护一份：registry 不依赖 tag 包。
var likeEscaper = strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`)

// escapeLike 把搜索词里的 LIKE 通配符转义成字面量
func escapeLike(q string) string { return likeEscaper.Replace(q) }

// isUUIDText 粗校验 UUID 文本（8-4-4-4-12 带连字符，或 32 位无连字符的十六进制）。
// parent_id 是非空但格式非法的字符串时按「查不到」处理，不把它送进 SQL 触发 22P02 变 500。
func isUUIDText(s string) bool {
	hexOnly := func(c byte) bool {
		return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
	}
	switch len(s) {
	case 36:
		for i := 0; i < len(s); i++ {
			switch i {
			case 8, 13, 18, 23:
				if s[i] != '-' {
					return false
				}
			default:
				if !hexOnly(s[i]) {
					return false
				}
			}
		}
		return true
	case 32:
		for i := 0; i < len(s); i++ {
			if !hexOnly(s[i]) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// ListPaged 按父级/关键字/类型分页查询台账，返回当页数据与匹配总数。
// 排序：文件夹优先，随后 alias 升序（alias 有唯一约束，分页顺序稳定）。
// parent_id 不是合法 UUID 时不报错，直接返回空页（前端传脏值应表现为「查不到」）。
func (s *Store) ListPaged(ctx context.Context, p ListParams) ([]Entry, int, error) {
	where := make([]string, 0, 3)
	args := make([]any, 0, 5)

	if p.ParentID == nil || *p.ParentID == "" {
		where = append(where, "parent_id IS NULL")
	} else {
		if !isUUIDText(*p.ParentID) {
			return []Entry{}, 0, nil
		}
		args = append(args, *p.ParentID)
		where = append(where, fmt.Sprintf("parent_id = $%d", len(args)))
	}
	if q := strings.TrimSpace(p.Q); q != "" {
		args = append(args, escapeLike(q))
		n := len(args)
		// alias / note / tags（jsonb 数组任一元素）任一包含即命中，保留改造前「名字/描述/标签都能搜」的能力；
		// 三处转义符统一是反斜杠（PG LIKE 默认值，这里写显式 ESCAPE 以便阅读）
		where = append(where, fmt.Sprintf(
			"(alias ILIKE '%%'||$%d||'%%' OR note ILIKE '%%'||$%d||'%%'"+
				" OR EXISTS (SELECT 1 FROM jsonb_array_elements_text(COALESCE(tags, '[]'::jsonb)) AS t"+
				" WHERE t ILIKE '%%'||$%d||'%%' ESCAPE '\\'))", n, n, n))
	}
	if p.IsDir != nil {
		args = append(args, *p.IsDir)
		where = append(where, fmt.Sprintf("is_dir = $%d", len(args)))
	}
	clause := " WHERE " + strings.Join(where, " AND ")

	var total int
	if err := s.conn.QueryRow(ctx, "SELECT count(*) FROM registry"+clause, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("统计台账数失败: %w", err)
	}

	pageArgs := append(append([]any{}, args...), p.Limit, p.Offset)
	rows, err := s.conn.Query(ctx,
		"SELECT "+entryCols+" FROM registry"+clause+
			fmt.Sprintf(" ORDER BY is_dir DESC, alias LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2),
		pageArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("查询台账失败: %w", err)
	}
	defer rows.Close()

	out := make([]Entry, 0)
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("扫描台账失败: %w", err)
		}
		out = append(out, *e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// ListFolders 全部文件夹的轻量列表（只有 id/alias/parentId），扁平按 alias 升序；
// 供前端建树、面包屑与子条目计数，避免为了拿层级把全部条目连同 paths/tags 一起拉下来。
func (s *Store) ListFolders(ctx context.Context) ([]FolderRef, error) {
	rows, err := s.conn.Query(ctx,
		"SELECT id, alias, parent_id FROM registry WHERE is_dir ORDER BY alias")
	if err != nil {
		return nil, fmt.Errorf("查询文件夹列表失败: %w", err)
	}
	defer rows.Close()

	out := make([]FolderRef, 0)
	for rows.Next() {
		var f FolderRef
		if err := rows.Scan(&f.ID, &f.Alias, &f.ParentID); err != nil {
			return nil, fmt.Errorf("扫描文件夹列表失败: %w", err)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}
