package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"file-ledger/internal/logx"
)

var slog = logx.New("registry")

// Store 台账存储：pgx 连接
type Store struct {
	conn *pgx.Conn
}

func NewStore(conn *pgx.Conn) *Store {
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
