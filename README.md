# StarCrystal Go API (Scaffold)

This is a lightweight Go backend scaffold for the mini-game aggregation app.

## Auth integration notes

For Google/Facebook Android login flow and phone registration implementation details, see:

- `../StarCrystal2022/AUTH_INTEGRATION_NOTES.md`
- WebView rewarded ads & `watchId` flow: `../StarCrystal2022/doc/ADS_INTEGRATION.md`

## Implemented

- `GET /` (simple ok response for browser open)
- `GET /favicon.ico` (returns 204 to avoid noisy 404 logs)
- `GET /healthz`
- `GET /api/v1/games` (requires `appVersion` and `platform` query params; each game item in `data.games` includes `name`, `note`, `iconLink`, `coverUrl`, `entryType`, `entryUrl`, `gameId`, …)
- `GET /api/v1/wallet/balance` (sample response)
- `GET /api/v1/wallet/ledger` (sample response)

## Ads (rewarded, WebView + shared backend)

- `POST /api/v1/ads/start` — bearer auth; creates `watchId`, returns today's/total completion counts snapshot.
- `POST /api/v1/ads/complete` — bearer auth; consumes `watchId`, applies reward (`AD_REWARD_GOLD`, `AD_REWARD_MONEY`), returns updated counts + economy fields.

Requires MySQL schema tables `auth_ad_watch_sessions`, `auth_ad_completions` in `sql/starcrystal_auth_mysql.sql`.

Basic anti-bot / pacing (single-process): min watch interval before `complete`, optional daily completion cap, per-account + per-IP per-minute limits, max concurrent unconsumed watches, optional `slot` allowlist (`internal/antifraud`, see `doc/ADS_INTEGRATION.md` §4.2 in the Unity repo).

**Registration → ad rewards**: same-day thresholds by `deviceId` optional body field + server IP (`REG_ACCOUNTS_PER_DEVICE_PER_DAY`, `REG_ACCOUNTS_PER_IP_PER_DAY`); offending new accounts store `ad_rewards_disabled`, then `1430` Forbidden on `/api/v1/ads/start|complete`. Columns are in `sql/starcrystal_auth_mysql.sql` (full rebuild). Details §4.3 in `../StarCrystal2022/doc/ADS_INTEGRATION.md`.

### Tests

- 完整分层说明、路由清单与维护约定见 **`doc/TESTING.md`**。

- **快速**：`cd server-go && go test ./...`

- **MySQL 集成**（可选）：  

  ```powershell
  $env:STARCRYSTAL_INTEGRATION_MYSQL = 'USER:PWD@tcp(127.0.0.1:3306)/starcrystal_auth?parseTime=true&loc=Local'
  go test ./internal/integration -tags=integration -count=1
  ```

## Reserved Endpoints (placeholder)

- `POST /api/v1/ads/callback/{network}`
- `POST /api/v1/welfare/redeem-token-gift`（兑换 Token 为礼品；替代原 `wallet/withdraw/*`）
- `GET /api/v1/welfare/redeem-gift/{redeemId}`（兑换单状态查询，占位）

These endpoints still return placeholder responses unless implemented later.

## Run

HTTP 监听地址**仅**由 **`release/configs/starcrystal.json`** 的 **`apiListenHost`** / **`apiListenPort`** 决定（至少配置其一则生效）；两者都未配置时进程监听 **`:8080`**（所有网卡）。环境变量 **`API_ADDR` 已废弃**，若仍设置会在日志中出现 **`ignored`** 警告；启动脚本会在拉起进程前去掉子进程继承的 **`API_ADDR`**。

```bash
cd server-go
go run ./cmd/api
```

## Build (Windows)

```powershell
cd server-go\release
..\tools\scripts\build.ps1
```

Build output directory: `release/`（与脚本同目录）
- Executable: `release/starcrystalsvr.exe`
- Config folder: `release/configs/`

Start auth-enabled local server (with SMS mock code in response):

```powershell
cd server-go\release
.\startsvr.ps1
```

Start auth server with MySQL account-password login enabled:

```powershell
cd server-go\release
.\startsvr.ps1 -AuthMySqlDsn "star_auth:star_auth_123456@tcp(127.0.0.1:3306)/starcrystal_auth?charset=utf8mb4&parseTime=true&loc=Local"```

If you use the portable MySQL instance started by this repo scripts (port `3307`):

```powershell
cd server-go\release
.\startsvr.ps1 -AuthMySqlDsn "star_auth:star_auth_123456@tcp(127.0.0.1:3307)/starcrystal_auth?charset=utf8mb4&parseTime=true&loc=Local"```

（如需换 HTTP 端口，只改 **`release/configs/starcrystal.json`** 中的 **`apiListenHost` / `apiListenPort`**。）

Run from release directory:

```powershell
cd server-go/release
.\startsvr.ps1
```

## Verify game list API (Windows)

```powershell
cd server-go\release
..\tools\scripts\verify-games-api.ps1
```

The script will:
- start API server on `127.0.0.1:18080`
- call `/healthz`
- call `/api/v1/games` with different `appVersion`
- verify protocol fields: `data.configVersion`, `data.serverTime`, `data.games`

Example version filtering in `release/configs/games.json`:
- `g002` requires `minAppVersion: 1.0.0`
- `g003` requires `minAppVersion: 1.1.0`

## One-command build + verify

```powershell
cd server-go\release
..\tools\scripts\all.ps1
```

Environment variable:

- `LOG_LEVEL` — default **`debug`** if unset (no config/env). **`release/configs/starcrystal.json`** in the repo is also **`debug`**; after a successful **Unity Android Release** (APK/AAB) build, the editor writes that file under sibling **`server-go`** to **`error`** when present. Use `error` in config or env for quieter production logs. **`[api][debug]`** HTTP request/response lines are emitted only when the effective level is **`debug`** (same as other debug logs). Effective level comes from **`release/configs/starcrystal.json`**, else **`LOG_LEVEL`**, else default **`debug`**; **`logger.Init`** is the sole in-process level setter.
- **`API_ADDR`** — **不再用于绑定端口**；请使用 **`starcrystal.json`**。若环境中仍存在 **`API_ADDR`**，进程启动时会记录 **`ignored`** 警告。
- `GAMES_CONFIG` (default `./release/configs/games.json`)
- 静态资源 **`GET /assets/*`** 映射到 **进程当前工作目录**下的 **`./assets`**（例如 **`release/startsvr.ps1`** 将工作目录设为 **`release/`**，则使用 **`release/assets/`**；本地 H5 示例路径 **`server-go/release/assets/h5/`**）
- `AUTH_MYSQL_DSN` (**required** unless `release/configs/starcrystal.json` sets `authMysqlDsn`): auth 与激励广告等持久化依赖 MySQL（表 `auth_accounts` 等）；缺失则在进程启动时退出

## Auth DB (MySQL)

- Schema: `sql/starcrystal_auth_mysql.sql` — 唯一完整 DDL（整库执行即可）
- Unified account ID convention: `type_accountValue`  
  - email example: `email_demo@example.com`  
  - phone example: `phone_+923001234567`
- Player/account persistence goes through **`store.PlayerRepository`** (Repository pattern). Implementations (`store.MySQLPlayerRepository` today; e.g. Scylla-backed repo later) own all SQL/CQL — **`internal/service` must not import `database/sql`**.
- 「Not found」in domain code uses **`store.IsNotFound(err)`**, not driver-specific `sql.ErrNoRows`.
- 表结构以 **业务主键** 为主（无 `AUTO_INCREMENT` 代理主键），详见同文件。

Initialize database/schema/user (after MySQL is installed and running):

Portable MySQL (**mysqld only** — no schema, no API): from `release/` run `..\tools\scripts\dbscripts\mysql\mysql-start.ps1` (or `..\tools\scripts\dbscripts\startdb.ps1`), then `..\tools\scripts\dbscripts\mysql\rebuild-auth-mysql.ps1`, then `.\startsvr.ps1`:

```powershell
cd server-go\release
..\tools\scripts\dbscripts\mysql\mysql-start.ps1
..\tools\scripts\dbscripts\mysql\rebuild-auth-mysql.ps1
.\startsvr.ps1
```

Linux offline: `bash tools/scripts/install-linux.sh`. Docker dev: `bash tools/docker/install-docker.sh` then `tools/docker/docker_svrdev.sh`. See `tools/scripts/readme.txt` and `tools/docker/readme.txt`.

## Quick test

```powershell
curl "http://127.0.0.1:8080/healthz"
curl "http://127.0.0.1:8080/api/v1/games?appVersion=1.0.0&platform=android"
curl "http://127.0.0.1:8080/assets/h5/game1.html"
curl "http://127.0.0.1:8080/api/v1/wallet/balance"
```

Or use PowerShell native cmdlet:

```powershell
Invoke-RestMethod "http://127.0.0.1:8080/api/v1/games?appVersion=1.0.0&platform=android"
```

`release/configs/games.json` 的字段名须为 **camelCase**（`gameId`、`name`、`note` 等）；服务端 `GameItem` 已用 `json` 标签与之一致，否则 `note` 等无法从文件反序列化进内存，回包里也不会出现。

## Game config hot reload behavior

`GET /api/v1/games` reads the config file every time, so if you edit `release/configs/games.json`, the next request immediately returns latest content (no server restart required).
