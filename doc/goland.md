# Go 项目编码规范（GoLand / IDE 提示防坑）

> 用途：Go 项目复用。目标：提交的代码**不产生 IDE 告警**、沙箱验证不了的部分**有兜底流程**。
> 来源：file-ledger 项目实测踩坑（2026-10-05 沉淀）。

## 1. 提交前检查链（必跑，全绿才提交）

```bash
gofmt -l .        # 格式（有输出 = 未格式化，gofmt -w 修）
go vet ./...      # 静态检查（errcheck/类型等）
go test ./...     # 单测
go build .        # 编译
```

## 2. 错误处理规范（errcheck 类提示）

| 场景 | 做法 |
|---|---|
| 正常错误 | `if err != nil { return fmt.Errorf("上下文: %w", err) }`——必须处理，**禁止 `_ =` 裸忽略** |
| defer 关闭连接/文件 | **封装 helper**：`db.Close(module, ctx, conn)`——defer 一行，关闭失败在 helper 内部记日志，调用方无返回值可漏 |
| 永不失败的写入（`strings.Builder` 等） | **封装 write helper**：`func write(b *strings.Builder, s string) { _, _ = b.WriteString(s) }`——签名如此（error 恒 nil），统一在封装层忽略，业务代码干净 |
| 响应写入（`json.NewEncoder(w).Encode`） | 接住 + 记日志（失败也影响不了响应） |

原则：**「永不失败」的 API 封装成无错误返回值的形式；会失败的必须处理并带上下文。**

## 3. doc 注释规范（GoLand inspection）

- 导出标识符（函数/方法/类型/变量）注释**必须以标识符名开头**：`// Debug 输出 DEBUG 级别日志。`
- 每个导出方法单独注释，禁止一个注释块盖多个导出方法（`// Debug/Info/Warn/Error ...` 是错的）
- 用法示例放类型级注释，不放方法级
- 注释与被注释项之间**空一行**，否则被当成上一段

## 4. 实测坑（本仓库血泪）

| 坑 | 现象 | 解法 |
|---|---|---|
| pgx 二进制协议 | `timestamptz` 扫 `*string` 报 `cannot scan ... into *string` | 先扫 `time.Time` 再 `Format(RFC3339)` |
| UUID 手工拼接 | 拼出非法格式（8-4-5-3-12） | 严格按 RFC 9562 8-4-4-4-12；**SQL 逻辑沙箱验证不了 → 提交前用 node/python 等价模拟 + 真机首跑观察** |
| `http.ServeMux.HandleFunc` | Go 1.22+ 返回 error？——**不是**，无效模式直接 panic | 按 go.mod 声明的 Go 版本查标准库签名，别信 IDE 旧版 SDK 提示；以 `go vet` 为准 |
| defer 里想处理错误 | 不能 return | 封装 helper（见 §2） |
| edit 批量替换 | 一处 oldText 不匹配 = 整批回滚 | 大改动拆小批；注意 tab/空格精确匹配 |

## 5. 命名与结构约定（简洁版）

- 包名小写单数；`internal/` 下按域分包（registry/api/db/logx）
- 错误定义集中：`var ErrXxx = errors.New("...")`，用 `errors.Is` 判断
- JSON 字段用 `json:"snake_case"` 与前端对齐；omitempty 只用于可空字段
- context 一律第一参数；连接/资源生命周期集中管理
- 第三方依赖最小化：能用标准库不用库；新依赖进 `go.mod` 前说明理由

## 6. GoLand 设置建议

- **SDK 版本与 go.mod 一致**（版本不一致 = errcheck 等提示误报，如 HandleFunc 案例）
- 打开：Editor → Inspections → Go → 勾选 `Unhandled errors`、`Comment format`
- 提交前手动跑一遍 §1 检查链，别依赖 IDE 全绿（IDE 检查 ≠ go vet）
