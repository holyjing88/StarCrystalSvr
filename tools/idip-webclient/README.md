# StarCrystal IDIP Web Client

> **仓库路径**：`starcrystalsvr/tools/idip-webclient/`（服务端组成部分）  
> **Linux 部署与云验收**：[doc/部署与验收.md](doc/部署与验收.md)

内网运营工具：对接 `server-go` 的 `/idip/v1/*` 接口，并提供回归测试（Vitest 26 条）。

**API 文档**

- 仓库根 [doc/IDIP_API.md](../../doc/IDIP_API.md) — 权威
- [doc/H5游戏发布与运营登录设计.md](doc/H5游戏发布与运营登录设计.md) — 运营登录、H5 上传、games.json

## 快速开始

```bash
cd tools/idip-webclient
npm install
npm run dev          # http://localhost:5174
npm run regression   # 对 IDIP_BASE_URL 跑 Vitest
```

Linux 生产部署：

```bash
sudo ENABLE_HTTPS=1 bash tools/scripts/idip-webclient/deploy-idip-webclient.sh
```

## 开发 UI（详）

```bash
cd tools/idip-webclient
npm install
npm run dev
```

浏览器打开终端里提示的地址（默认 **http://localhost:5174**）。API Base **留空** 时 Vite 会代理 **`/idip`** 与 **`/api`** 到 `127.0.0.1:8080`（回归里建游客账号需要 `/api`）。也可直接填 `http://127.0.0.1:8080`。

若网页「运行全部回归」只有 **7/10** 通过，常见原因是浏览器仍连着**旧 dev**（改 `vite.config.ts` 前启动、未代理 `/api`），失败项多为 IDP-003、IDP-006、TSK-IDP-PROGRESS。请按下一节关掉旧进程后重新 `npm run dev`。

### 端口占用：关掉旧 Vite

再执行 `npm run dev` 时若出现 `Port 5174 is in use, trying another one...`，说明 **5174 上还有旧实例**（例如 http://localhost:5175 是新开的，5174 仍是旧配置）。请只保留**最新**这一次 dev，并关掉占 5174 的进程。

**方式一（推荐）**：找到当初跑 `npm run dev` 的终端窗口，焦点在该窗口后按 **`Ctrl+C`**，等待进程退出。

**方式二（Windows PowerShell）**：查占用 5174 的 PID 并结束：

```powershell
# 查看谁在监听 5174
netstat -ano | findstr :5174

# 将 <PID> 换成上一行 LISTENING 对应的最后一列数字
taskkill /PID <PID> /F
```

或一行结束占用 5174 的进程（需管理员权限时以管理员打开 PowerShell）：

```powershell
Get-NetTCPConnection -LocalPort 5174 -ErrorAction SilentlyContinue |
  ForEach-Object { Stop-Process -Id $_.OwningProcess -Force -ErrorAction SilentlyContinue }
```

然后重新启动：

```bash
npm run dev
```

确认终端输出为 `Local: http://localhost:5174/`（无 “trying another one”），浏览器也访问 **5174**（不要继续用已关闭的旧标签页端口）。

## 回归测试（网页三类 Tab）

「回归测试」页内三个子 Tab，分别对应已实现的自动化：

| 子 Tab | 实现 | 网页内执行 |
|--------|------|------------|
| **IDIP 回归** | `src/tests/cases.ts` · `npm run regression` | 浏览器直接调 IDIP/API（10 条） |
| **服务端回归** | `src/regression/serverCatalog.ts`（17 条 / 15 go test） | dev 下 `POST /dev/regression/server` |
| **客户端回归** | `src/regression/clientCatalog.ts`（32 条） | dev 下 `POST /dev/regression/client`（需 `UNITY_EDITOR`） |

详见 [`doc/回归套件.md`](doc/回归套件.md)。

服务端/客户端回归仅在 **`npm run dev`** 时可用（Vite 插件拉起本机 `go test` / Unity batch）。

**常见失败原因**

| 现象 | 处理 |
|------|------|
| 服务端全部 `go test exit 255` | 旧 Vite 未加载修复：关掉 5174 旧进程后重新 `npm run dev`（Windows 勿让 `-run` 被 cmd 当管道） |
| 客户端提示「已有 Unity Editor 打开」 | 默认会**自动结束**占用该工程的 `Unity.exe`；仍失败则手动关 Unity 或设 `SC_FORCE_CLOSE_UNITY=0` 自行管理 |
| 点击客户端回归后页面像卡死 | 已改为**后台任务 + 轮询**：按钮会显示阶段（结束 Unity / EditMode / PlayMode）；全程约 10–40 分钟，请勿关 `npm run dev` |
| 客户端找不到测试 | 确认已重启 dev；工程能编译；结果在 `idip-webclient/tmp-regression/*.xml` |

## 自动化回归（CLI）

需 server-go 已启动：

```bash
npm run regression
# 或
IDIP_BASE_URL=http://127.0.0.1:8080 IDIP_KEY=change-me-in-production npm test
```

用例定义：`src/tests/cases.ts`；文档对照：`doc/测试用例.md`、`doc/API与用例对照.md`（与仓库根 `doc/后台测试用例-v7.1.md` §I/§L 同步）。

**浏览器一键验收**（需先 `npm run dev`，端口与 `VITE_URL` 一致）：

```bash
# PowerShell 示例：dev 在 5174
$env:VITE_URL="http://localhost:5174"
npm run acceptance
```

## 构建

```bash
npm run build
npm run preview
```

生产部署请将静态站与 API 同在内网，或由网关将 `/idip` 反代到 server-go。

## 接口一览

| 方法 | 路径 |
|------|------|
| POST | `/idip/v1/gold/set-user` |
| GET | `/idip/v1/gold/month-user` |
| POST | `/idip/v1/gold/recalc-server-delta-total` |
| GET | `/idip/v1/welfare/month-token-pool` |
| POST | `/idip/v1/welfare/set-month-token-pool` |
| POST | `/idip/v1/welfare/run-monthly-settlement` |
| GET | `/idip/v1/tasks/definitions` |
| POST | `/idip/v1/tasks/tier-policy` |
| POST | `/idip/v1/tasks/definition/upsert` |
| GET | `/idip/v1/tasks/user-progress` |
