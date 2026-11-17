# AGENTS.md — file-ledger 项目约定

> 本文件是这个仓库的作业规范，优先级高于工作区根 `AGENTS.md` 的通用部分（根文件仍适用于删除、git 铁律等硬约束）。
> 改完本文件要重新 `read` 一次（或新会话）才会被注入。

## 1. 项目定位（先读这段再动手）

Windows 本地「文件台账」工具：单 exe（Go 后端 + go:embed 内嵌 Vue3 前端）+ PostgreSQL 存台账，双击即用，服务 `http://127.0.0.1:18765`。

**核心概念 = 虚拟网盘**：

- **文件夹 = 虚拟文件夹 = 纯分类容器**（2026-11-07 定版）：不填路径、列表里不出「浏览」按钮，只负责把散落的条目归拢分类
- **条目**（文件或文件夹记录）才带路径，`local`（本地目录）/ `mount`（挂载盘符）/ `virtual`（网盘指向，如 `ali:game/rpg`）三类都是一等公民
- 所以：**目录浏览的对象只能是台账条目自己的路径**，不是文件夹的路径

产品口径变更必须先问老板，别自行"优化"。

## 2. 目录与分层

| 位置 | 职责 |
|---|---|
| `main.go` | 入口：配置、建库建表、起服务、开浏览器、挂 embed |
| `internal/registry` | 台账模型 + PostgreSQL 存储 |
| `internal/fsx` | 只读列目录（路径规范化、防穿越、排序、截断） |
| `internal/api` | REST 处理器与路由（registry.go / fs.go / tags.go） |
| `internal/tag` | 标签字典模型 + PostgreSQL 存储（改名/删除事务同步 `registry.tags`） |
| `internal/db`、`internal/logx` | 建库建表、结构化日志（rid） |
| `internal/test` | 黑盒测试（HTTP 端到端） |
| `web/src/views` | 页面，**只做编排** |
| `web/src/components` | 可复用组件；某页面私有的子组件放 `components/<页面名>/` |
| `web/src/composables` | 数据与业务逻辑（调 api、派生状态） |
| `web/src/utils` | 跨组件复用的纯函数 |
| `web/src/constants` | 枚举等常量（标签与配色已改由后端 `tag` 表管理，见 `composables/useTags.ts`） |
| `doc/` | 01 需求 / 02 选型 / 03 架构 / 04 计划 / 05 进度 |

**前端分层约定（高内聚低耦合，老板 2026-11-17 要求）**：

- 组件**只收 props、只抛事件**，不直接调接口、不管业务规则
- 数据加载与增删改放 `composables/`；页面只串线
- 同一段纯逻辑只能有一份：两个组件都要用 → 提到 `utils/` 或 `composables/`
- 新增/删除前端文件时，同步更新 `doc/05-进度.md`「前端结构」节

## 3. 文档同步（硬要求，别留"以后再补"）

**改动行为、接口、结构或约定时，同一次 push 里必须同步文档**：

| 改了什么 | 必须同步 |
|---|---|
| 功能/进度/完成状态 | `doc/05-进度.md`（状态表 + 今日完成 + 遗留待办） |
| 目录结构、API 边界、数据模型、配置字段 | `doc/03-架构设计.md` |
| 用户可见功能、构建/运行方式 | `README.md` |
| 产品口径（如文件夹语义） | `doc/01-需求说明.md` + `doc/05-进度.md` |
| 阶段划分/验收标准 | `doc/04-开发计划.md` |

- 文档与代码分开提交也行，但**必须在同一批 push 内完成**，提交信息用 `doc: …`
- 文档里"规划中/未实现"的东西要显式标注状态，别写成既有能力

## 4. 构建与产物

- **exe 不进仓库**（老板 2026-11-17 定）：`.\build.ps1` 输出到 `D:\Development\build\file-ledger\`；`-OutDir` 改位置，`-WithWeb` 连前端一起构建
- `web/dist` **要入库**：单 exe 靠 `go:embed` 嵌它；改前端必须 `cd web && npm run build` 并提交新的 dist
- **dsh 的 bash/pwsh 子进程读不到用户级环境变量**：直接 `go build` 会去 proxy.golang.org 拉依赖并超时。要么用 `build.ps1`，要么先 `export GOPATH=D:/Development/gopath GOMODCACHE=D:/Development/gopath/pkg/mod GOCACHE=D:/Development/gocache`
- Go 在 `D:\Development\go\bin`；PostgreSQL 便携版在 `D:\Development\pgsql`（数据目录 `D:\Development\pgsql-data`）

## 5. 验证门槛（提交前自己先跑）

- 后端：`go build ./...`、`go vet ./...`、`go test ./...` 全绿；`gofmt` 干净
- 前端：`cd web && npm run build`（含 `vue-tsc` 类型检查）
- 有 UI 行为改动：重建 exe 后实测（curl 首页 200 + 内嵌 bundle hash 与新 dist 一致 + 相关接口 200）

## 6. git 铁律（继承工作区根 AGENTS.md）

- 一修改一 push：每处改动立刻 `commit` + `push origin main`，一个 commit 只说一件事
- 提交信息中文：`范围: 改了什么(为什么)`，范围用 `api` / `web` / `db` / `test` / `doc` / `chore`
- 动仓库前先 `git status` + `git log --oneline -3` + `git remote -v`；发现不是自己改的东西先问
- **禁止** `git reset --hard` / `git checkout -- .` / `git clean -fd` / `git push --force`
- push 失败报原文并停下；判成功看 `git status -sb` 无 `ahead`，或 `git log -1 origin/main` = HEAD

## 7. 删除与凭据

- **删任何文件/目录前必须先问老板并逐条列路径**，同意只对当次有效；`build.ps1` 重建时 vite 清理旧 dist 资源属于构建产物替换，不用问，但要在提交里说清楚
- `config.json` 在 exe 旁、已被 `.gitignore` 覆盖；不要把数据库密码等凭据写进代码、文档或提交信息

## 8. 本地起服务（自测用）

1. 数据库：双击 `D:\Development\start-pg.bat`；或后台跑 `D:\Development\pgsql\bin\postgres.exe -D D:/Development/pgsql-data -p 5432`（**必须后台**，当前台命令会把工具拖超时并杀进程）
2. 应用：`D:\Development\build\file-ledger\file-ledger.exe` → `http://127.0.0.1:18765`
3. 停：先关 exe 再停 PG（`pg_ctl -D D:/Development/pgsql-data stop -m fast`）

## 9. 当前状态与下一步

状态表、遗留待办、下一步看 `doc/05-进度.md`（唯一权威）。批次 1 步骤 0-4 已完成，下一步是步骤 5：虚拟路径复制/跳转 + 盘符在线离线徽标。
