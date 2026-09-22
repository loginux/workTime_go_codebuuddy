# ⏱ workTime - 工时记录工具

一款轻量级的 Web 端工时记录工具，帮助用户精确回答 **"时间花在哪里"**。

> 本项目是 [WokTime (Python/Flask)](../worktime) 的 Go 语言版本，功能对齐，单二进制分发，无运行时依赖。

---

## 目的与作用

在日常工作中，经常遇到以下问题：

- 一天下来不知道时间花在了哪里
- 汇报工作时说不出每个项目/任务花了多少时间
- 缺乏一个轻便的工具随手记录工时

**workTime** 就是为了解决这些问题而生的。它让用户能够：

1. 快速记录每天在每个项目/任务上花费的时间（分钟级）
2. 通过**日/周/月**视图直观查看时间分布
3. 支持穿透查看——从周/月汇总点击某天直达该天明细
4. 导出 CSV 做进一步分析或汇报

---

## 功能特性

| 模块 | 功能 |
|------|------|
| **用户切换** | 多用户数据隔离，无密码，点击即切换 |
| **项目管理** | 创建/编辑/删除项目；自动创建默认项目（不可删） |
| **任务管理** | 在项目下创建/编辑/删除任务；自动创建默认任务（不可删） |
| **工时录入** | 起止时间自动计算分钟数（自动排除 12:00-13:00 午休），默认 09:00-18:00，也可手动填写 |
| **日视图** | 明细列表 + 按项目/项目-任务双饼图 + 汇总表格 |
| **周视图** | 柱状图（Chart.js，纵轴小时），每日卡片，点击穿透 |
| **月视图** | 自定义起始日期的月度滚动视图，工时色阶（<8.5h / 8.5-9.5 / 9.5-10.5 / 10.5-11.5 / ≥11.5h）、项目标签、法定节假日 |
| **CSV导出** | 自定义日期范围，含 BOM 头兼容 Excel |
| **工时转移** | 删除任务/项目时自动将工时转移至默认实体，数据不丢失 |

---

## 技术栈

| 层级 | 技术 |
|------|------|
| 后端框架 | **Go 标准库 net/http**（Go 1.22+，无第三方 Web 框架） |
| 模板引擎 | **html/template**（编译期嵌入，go:embed） |
| 数据库 | **SQLite**（modernc.org/sqlite，纯 Go 实现，无 CGO） |
| 前端图表 | **Chart.js**（柱状图 + 饼图，CDN 引入） |
| 会话 | HMAC 签名 Cookie（uid + CSRF + flash 消息） |
| CSS | 纯手写，无框架依赖 |

**单二进制分发**：模板与静态资源全部通过 `go:embed` 打包进可执行文件，部署只需一个 exe + 可选的 `holiday/` 目录。

---

## 设计原理

### 数据模型

采用**项目-任务二级结构**，每个项目下可包含多个任务：

```
用户
 ├── 项目1（可多个）
 │    ├── 默认任务（自动创建，不可删）
 │    ├── 任务A
 │    └── 任务B
 └── 项目2
      ├── 默认任务
      └── ...
```

### 核心机制

- **软删除**：项目和任务删除时仅标记 `is_deleted=1`，数据保留在数据库中以供历史统计
- **工时转移**：删除非默认任务 → 工时自动转移至同项目的默认任务；删除非默认项目 → 工时转移至全局默认项目
- **物理删除**：工时记录可物理删除（唯一可彻底删除的数据）
- **默认实体保护**：每个用户有一个"默认项目"、每个项目有一个"默认任务"，可改名但不可删除，确保始终有兜底容器
- **午休扣除**：工时录入时自动计算分钟数，**自动排除 12:00-13:00 午休时间**（如 09:00-18:00 计算为 8 小时而非 9 小时）
- **自动备份**：启动时备份 + 每周六 03:00 定时备份，**保留最近 9 份**备份文件

### 视图逻辑

- **日视图**：按日期查询明细，按项目/任务分组汇总，双饼图可视化占比
- **周视图**：以周一~周日为一周，Chart.js 柱状图（纵轴小时），tooltip 显示 X小时X分钟
- **月视图**：以用户设置的起始日期为起点，范围到下个月同日 -1 天。日工时按 <8.5 / 8.5-9.5 / 9.5-10.5 / 10.5-11.5 / ≥11.5h 色阶显示（法定节假日有加班时至少为 9.5-10.5 档）；当天任一任务名称含"请假"时整格标蓝（该机制不在图例显示）；每天展示涉及的项目标签和法定节假日

---

## 目录结构

```
worktime_go/
├── main.go                       # 启动入口（--host / --port）
├── config.go                     # 数据目录配置
├── db.go                         # SQLite 连接 + 建表
├── models.go                     # 数据访问层
├── session.go                    # 会话（Cookie 签名 + CSRF + Flash）
├── context.go                    # 会话/登录/CSRF 中间件
├── holiday.go                    # 节假日加载模块
├── backup.go                     # 数据库自动备份
├── mux.go                        # 路由注册
├── handlers_auth.go              # 用户选择/创建/切换
├── handlers_projects.go          # 项目 + 任务 CRUD
├── handlers_time.go              # 工时录入/编辑/删除
├── handlers_views.go             # 日/周/月视图
├── handlers_export.go            # CSV 导出
├── web/
│   ├── templates/                # html/template 模板
│   └── static/                   # CSS / JS
├── .github/workflows/build.yml   # GitHub Actions 多平台构建
└── instance/                     # 运行时自动创建（数据库/备份/密钥）
```

---

## 部署与运行

### 源码运行

```bash
cd worktime_go
go mod tidy          # 首次运行生成 go.sum
go run . --port 5000
# 访问 http://127.0.0.1:5000

# 也可指定监听地址和端口：
go run . --host 0.0.0.0 --port 8080
```

### 编译

```bash
go mod tidy
go build -trimpath -ldflags "-s -w" -o workTime.exe .
```

### 本地手动交叉编译

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -o workTime-linux-armv7 .
```

---

## GitHub Actions 构建

项目内置了 GitHub Actions 工作流（`.github/workflows/build.yml`），**每次推送到 main 分支自动构建**，并自动打递增版本号 tag（v0.1 → v0.2 → ...）发布到 Releases。产物统一命名为 `workTime`（Windows 为 `workTime.exe`）：

| 平台 | 产物 |
|------|------|
| windows/amd64 | `workTime-vX.Y-windows-amd64.exe` |
| linux/amd64 | `workTime-vX.Y-linux-amd64` |
| linux/arm64 | `workTime-vX.Y-linux-arm64` |
| linux/armv7（OpenWrt 等） | `workTime-vX.Y-linux-armv7` |

### 自动触发

推送代码到 main 分支即可：

```bash
git push
```

构建完成后自动创建 Release（含 tag 和全部平台二进制）。

### 手动触发

在 GitHub 仓库页面点击 **Actions** → **Build workTime** → **Run workflow**。

### 部署到 ARM 路由器（OpenWrt）

```bash
scp user@host:/path/to/workTime /root/
chmod +x /root/workTime
/root/workTime --host 0.0.0.0 --port 5000
```

---

## 节假日配置

月视图支持显示法定节假日。**仅读取外部目录**，不内置打包：

- **目录位置**：`二进制同级目录/holiday/`（也兼容工作目录下的 `holiday/`）

数据按年份存放，遵循 [holiday-cn](https://github.com/NateScarlet/holiday-cn) 格式：

```json
{
  "year": 2026,
  "days": [
    {"name": "元旦", "date": "2026-01-01", "isOffDay": true}
  ]
}
```

- `isOffDay: true` — 法定假日（显示）
- `isOffDay: false` — 调休上班日（不显示）

> 部署时，将 `holiday/` 目录放在二进制同级，无需重新编译即可更新节假日。
> 节假日数据可在启动时通过环境变量无刷新更新后重启进程生效（数据目录：`WORKTIME_DATA_DIR` 可自定义数据库存放位置）。

---

## 与 Python 版本的差异

| 项 | WokTime (Python) | workTime (Go) |
|----|------------------|---------------|
| 运行方式 | 需要 Python 环境 / PyInstaller 打包 | 单二进制，零依赖 |
| 打包产物 | ~15MB (PyInstaller) | ~8MB (Go 静态编译) |
| 交叉编译 | 需要 QEMU + Alpine 容器 | `GOOS/GOARCH` 原生支持 |
| 数据库文件 | `instance/woktime.db` | `instance/worktime.db` |
| 节假日目录 | 源码 `app/holiday/`，exe 同级 `holiday/` | 统一二进制同级 `holiday/` |
| 数据/备份/密钥 | — | `instance/`（二进制同级，`WORKTIME_DATA_DIR` 可覆盖） |

---

## 开发背景

本项目基于 Go 标准库构建，数据库使用 SQLite（modernc 纯 Go 驱动，零 CGO 依赖），前端采用服务端渲染（html/template）+ 原生 JS + Chart.js，无需 Node.js 或前端构建工具，开箱即用。
