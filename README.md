# file-ledger 文件台账管理工具

Windows 本地工具：像 Windows 资源管理器一样管理你的常用路径——本地目录、挂载盘符、网盘指向统一收录，散乱的文件归拢成树。

## 功能

- **Windows 式文件夹树**：虚拟文件夹（如「游戏」）是系统内逻辑分类容器，把散落在 `D:\games`、`Z:\rpg`、网盘各处的条目归拢管理；文件夹可嵌套，子条目跟随父移动
- **台账条目**：别名 + 多条路径（本地/挂载/网盘指向混合）+ 文件夹/文件 + 大小 + 标签 + 描述
- **单 exe**：前端内嵌，双击即用，自动建库建表（PostgreSQL），自动开浏览器（127.0.0.1:18765）
- **标签由数据库管理**：标签字典存 PostgreSQL（名称 + 配色），设置页可新增 / 改名 / 改色 / 删除；保存条目时输入的新标签自动登记进字典，改名与删除会同步到所有条目
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
- [x] 目录浏览（步骤 4：台账路径 → 只读抽屉浏览，面包屑 / 进入 / 上级）
- [x] 标签字典后端化（`tag` 表 + `/api/tags` 接口 + 设置页标签管理）
- [ ] 虚拟路径复制/跳转（步骤 5）
- [ ] 图片预览（批次 2）
- [ ] 打磨打包（批次 3）

## 构建

产物不落在仓库目录里（exe 不进项目目录），统一输出到仓库外的构建目录：

```powershell
cd web; npm run build        # 前端构建（dist 入库，单 exe 需要）
cd ..; .\build.ps1           # 编译到 D:\Development\build\file-ledger\file-ledger.exe
```

`build.ps1` 参数：`-OutDir <目录>` 换输出位置，`-WithWeb` 连前端一起构建（等价于先跑 npm run build）。
等价的手工命令（输出位置可按需改）：`go build -o D:\Development\build\file-ledger\file-ledger.exe .`

运行：双击 `D:\Development\build\file-ledger\file-ledger.exe` → 自动建库建表 → 浏览器打开 http://127.0.0.1:18765

配置：exe 旁 `config.json`（端口、挂载盘符、PostgreSQL 连接），首次运行自动生成。
