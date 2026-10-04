package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Store 台账存储：pgx 连接
type Store struct {
	conn *pgx.Conn
}

func NewStore(conn *pgx.Conn) *Store {
	return &Store{conn: conn}
}

const entryCols = "id, alias, is_dir, size, note, tags, paths, created_at, updated_at"

// scanEntry 把查询行扫成 Entry
func scanEntry(row pgx.Row) (*Entry, error) {
	var (
		e        Entry
		tagsRaw  []byte
		pathsRaw []byte
	)
	if err := row.Scan(&e.ID, &e.Alias, &e.IsDir, &e.Size, &e.Note,
		&tagsRaw, &pathsRaw, &e.CreatedAt, &e.UpdatedAt); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(tagsRaw, &e.Tags); err != nil {
		return nil, fmt.Errorf("解析 tags 失败: %w", err)
	}
	if err := json.Unmarshal(pathsRaw, &e.Paths); err != nil {
		return nil, fmt.Errorf("解析 paths 失败: %w", err)
	}
	return &e, nil
}

// List 台账列表，按别名排序（前端展示顺序稳定）
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

// Create 新增条目；id 由 DB 端 uuid_generate_v7() 生成
func (s *Store) Create(ctx context.Context, e *Entry) (*Entry, error) {
	tags, err := marshalJSONB(e.Tags)
	if err != nil {
		return nil, err
	}
	paths, err := marshalJSONB(e.Paths)
	if err != nil {
		return nil, err
	}

	row := s.conn.QueryRow(ctx,
		`INSERT INTO registry (alias, is_dir, size, note, tags, paths)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING `+entryCols,
		e.Alias, e.IsDir, e.Size, e.Note, tags, paths)
	created, err := scanEntry(row)
	if err != nil {
		return nil, translateErr(err)
	}
	return created, nil
}

// Update 按 id 更新；返回更新后的条目
func (s *Store) Update(ctx context.Context, id string, e *Entry) (*Entry, error) {
	tags, err := marshalJSONB(e.Tags)
	if err != nil {
		return nil, err
	}
	paths, err := marshalJSONB(e.Paths)
	if err != nil {
		return nil, err
	}

	row := s.conn.QueryRow(ctx,
		`UPDATE registry SET alias=$1, is_dir=$2, size=$3, note=$4, tags=$5, paths=$6
		 WHERE id=$7
		 RETURNING `+entryCols,
		e.Alias, e.IsDir, e.Size, e.Note, tags, paths, id)
	updated, err := scanEntry(row)
	if err != nil {
		return nil, translateErr(err)
	}
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
