# file-ledger 文件台账管理工具

Windows 本地工具：像 Windows 资源管理器一样管理你的常用路径——本地目录、挂载盘符、网盘指向统一收录，散乱的文件归拢成树。

## 功能

- **Windows 式文件夹树**：虚拟文件夹（如「游戏」）是系统内逻辑分类容器，把散落在 `D:\games`、`Z:\rpg`、网盘各处的条目归拢管理；文件夹可嵌套，子条目跟随父移动
- **台账条目**：别名 + 多条路径（本地/挂载/网盘指向混合）+ 文件夹/文件 + 大小 + 标签 + 描述
- **单 exe**：前端内嵌，双击即用，自动建库建表（PostgreSQL），自动开浏览器（127.0.0.1:18765）
- **结构化日志**：控制台输出，每请求带 rid，只看日志可定位 bug

## 技术栈

- 后端：Go（单二进制，Windows 直跑；唯一第三方依赖 pgx/v5）
- 前端：Vue3 + Element Plus（`go:embed` 内嵌）
- 数据：PostgreSQL（默认 `127.0.0.1:5432/file_ledger`，config.json 可配）

## 开发进度

- [x] 仓库初始化与文档归档（见 `doc/`）
- [x] 台账 CRUD（PostgreSQL，自动建库建表 + UUID v7）
- [x] 前端联调（真实 API，mock 已移除）
- [x] 文件夹树（Windows 式层级）
- [x] 单 exe 交付形态（前端 embed）
- [x] 日志系统（logx + rid）
- [ ] 目录浏览（步骤 4）
- [ ] 虚拟路径复制/跳转（步骤 5）
- [ ] 图片预览（批次 2）
- [ ] 打磨打包（批次 3）

## 构建

```bash
cd web && npm run build     # 前端构建（dist 入库）
cd .. && go build -o file-ledger.exe   # Windows 交叉编译：GOOS=windows go build
```

运行：双击 `file-ledger.exe` → 自动建库建表 → 浏览器打开 http://127.0.0.1:18765

配置：exe 旁 `config.json`（端口、挂载盘符、PostgreSQL 连接）。
