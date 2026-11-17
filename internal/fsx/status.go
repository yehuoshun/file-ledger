package fsx

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// 盘符在线/离线探测（步骤5：列表「离线」徽标）。纯只读：只对盘符根做 os.Stat。

// ErrInvalidDrive 盘符参数不合法（drives 为空、元素不是「A-Z 字母 + 可选冒号」、元素过多）。
var ErrInvalidDrive = errors.New("盘符参数不合法")

// MaxDrives 单次请求允许的盘符元素个数上限（A-Z 共 26 个）。
// 按逗号切分后的元素个数计（先于去重），否则重复元素可绕过上限。
const MaxDrives = 26

// ProbeTimeout 单个盘符的探测超时；掉线的网络盘 os.Stat 可能长时间阻塞，
// 超时按离线处理（Reason 为 ReasonTimeout）。
const ProbeTimeout = 2 * time.Second

// 离线原因（只在 Online=false 时写进 DriveStatus.Reason，在线时保持空串被 omitempty 略去）。
const (
	// ReasonOffline 探测返回错误（盘符不存在/已拔出/无权限）。
	ReasonOffline = "盘符不在线"
	// ReasonTimeout 探测在 ProbeTimeout 内未返回。
	ReasonTimeout = "探测超时"
)

// DriveStatus 单个盘符的在线状态。
// Reason 仅离线时出现（json omitempty），在线时前端不应看到该字段。
type DriveStatus struct {
	Drive  string `json:"drive"`
	Online bool   `json:"online"`
	Reason string `json:"reason,omitempty"`
}

// ParseDrives 解析 GET /api/fs/status?drives= 的参数（逗号分隔，如 "D:,Z:"）。
// 元素接受 "D:" 或 "D"，大小写不敏感，统一归一为 "D:"；去重保序。
// 空串/空元素/非法元素/元素个数超过 MaxDrives 一律返回 ErrInvalidDrive。
func ParseDrives(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("%w: drives 为空", ErrInvalidDrive)
	}
	parts := strings.Split(raw, ",")
	if len(parts) > MaxDrives {
		return nil, fmt.Errorf("%w: 盘符个数 %d 超过上限 %d", ErrInvalidDrive, len(parts), MaxDrives)
	}
	seen := make(map[string]struct{}, len(parts))
	drives := make([]string, 0, len(parts))
	for _, part := range parts {
		d, err := normalizeDrive(strings.TrimSpace(part))
		if err != nil {
			return nil, err
		}
		if _, ok := seen[d]; ok {
			continue
		}
		seen[d] = struct{}{}
		drives = append(drives, d)
	}
	return drives, nil
}

// normalizeDrive 校验单个盘符写法并归一化为 "D:"：只接受 1 个字母或「字母 + 冒号」。
func normalizeDrive(s string) (string, error) {
	if len(s) == 1 && isAlpha(s[0]) {
		return strings.ToUpper(s) + ":", nil
	}
	if len(s) == 2 && isAlpha(s[0]) && s[1] == ':' {
		return strings.ToUpper(s[:1]) + ":", nil
	}
	return "", fmt.Errorf("%w: %q 不是盘符", ErrInvalidDrive, s)
}

// ProbeDrives 并发探测各盘符，返回顺序与入参一致（入参应由 ParseDrives 归一化过）。
// 探测方式：os.Stat(drive + 分隔符)，成功即在线；出错即离线；超过 ProbeTimeout 记为超时离线。
func ProbeDrives(ctx context.Context, drives []string) []DriveStatus {
	out := make([]DriveStatus, len(drives))
	var wg sync.WaitGroup
	for i, d := range drives {
		wg.Add(1)
		go func(i int, d string) {
			defer wg.Done()
			out[i] = probeDrive(ctx, d)
		}(i, d)
	}
	wg.Wait()
	return out
}

// probeDrive 探测单个盘符：stat 放到独立 goroutine，超时即判离线。
// 结果通道带缓冲，超时后 stat 迟到返回也不会阻塞（最坏留下一个阻塞的 stat goroutine）。
func probeDrive(ctx context.Context, drive string) DriveStatus {
	root := drive + string(filepath.Separator)
	done := make(chan error, 1)
	go func() {
		_, err := os.Stat(root)
		done <- err
	}()

	timer := time.NewTimer(ProbeTimeout)
	defer timer.Stop()
	select {
	case err := <-done:
		if err == nil {
			return DriveStatus{Drive: drive, Online: true}
		}
		return DriveStatus{Drive: drive, Online: false, Reason: ReasonOffline}
	case <-timer.C:
		return DriveStatus{Drive: drive, Online: false, Reason: ReasonTimeout}
	case <-ctx.Done():
		return DriveStatus{Drive: drive, Online: false, Reason: ReasonTimeout}
	}
}
