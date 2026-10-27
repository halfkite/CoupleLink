# CoupleLink 1.0-r2 服务器测试部署

本轮提供账号、情侣关系、设备会话和手动增量同步基础。聊天、定位、推送及手环桥接没有接入。本版本用于隔离测试环境；自动备份、加密恢复演练及正式安全审计尚未完成，不建议存入正式私人资料。

## Docker Compose

使用发布包的 `server-bundle` 或源码根目录，在其中的 `deploy` 下复制 `.env.example` 为 `.env`。分别生成独立的数据库密码和设置码，至少各 32 个随机字节转十六进制；不要使用示例值。数据库密码仅用 URL 安全字符。设置码用于创建最初两个账号，不能当成账号密码。

```powershell
cd server-bundle/deploy
Copy-Item .env.example .env
# 编辑 .env，填入独立随机值、你控制的域名。
docker compose up -d --build
```

本机 API 为 `http://127.0.0.1:18080`，`GET /healthz` 应返回 `status: ok`、`api_version: 1`。数据库不发布主机端口，容器 API 以非 root 账号运行。只允许本机回环使用 HTTP；Android 模拟器可以访问 `http://10.0.2.2:18080`，实体手机应填写 HTTPS 域名。

启用 HTTPS 前将域名 A/AAAA 指向服务器，并开放 80/443；`COUPLELINK_DOMAIN` 不带协议或路径：

```powershell
docker compose --profile https up -d --build
```

Caddy 申请证书并代理 API。证书申请、域名和外网访问需在实际服务器验证，本机无法代替公网验收。原生 Android 无需 CORS 配置；浏览器联调须将完整来源（例如 `http://127.0.0.1:5178`）加入空格分隔的 `COUPLELINK_ALLOWED_ORIGINS`，没有通配来源。

不要执行 `docker compose down -v`，该命令会删除数据库卷。发布文件及普通日志不包含实际设置码、数据库密码、会话令牌或精确位置。

## 独立运行

提供 Windows x64 与 Linux x64 二进制，迁移 SQL 已嵌入。安装 PostgreSQL 17 或 18 并新建专用数据库，使用进程环境配置 `DATABASE_URL`、`COUPLELINK_SETUP_CODE`；可选 `COUPLELINK_LISTEN_ADDR`（默认 `127.0.0.1:18080`）。不要将包含密码的连接串写入命令行、共享截图或公开日志。Linux 二进制可能需先赋予执行权限。由反向代理提供 HTTPS。

启动时迁移持有数据库事务锁，已应用迁移不会重复执行；数据库版本高于程序支持版本时拒绝启动。正式升级前应备份数据库，而不是直接用旧程序覆盖新数据库。

## 手机联调

1. 我的 → 顶部双人卡片，填写 HTTPS 地址，切换注册；填写账号、昵称、至少 12 字符的密码及设置码。最多创建两个账号。
2. 一人生成邀请，另一人输入邀请、恋爱起点和 IANA 时区（如 `Asia/Hong_Kong`）。邀请 15 分钟有效，单次使用；新邀请立即撤销旧邀请。
3. 双方点击“刷新同步”，查看真实昵称、恋爱起点与天数。本轮不提供实时长连接或后台自动同步。
4. 撤销某个设备后，它下一次请求立即被服务端拒绝；解绑立即结束服务端关系，另一端刷新后清除关系。离线客户端仍可能显示旧缓存，不能承诺离线屏幕立即消失。
5. Android 会话保存到系统安全存储；浏览器测试只放内存，刷新页面需要重新登录。密码不会本地保存。

## 回归与备份边界

`server/internal/api/integration_test.go` 需要 `COUPLELINK_TEST_DATABASE_URL`。测试只创建随机临时 schema，并清理自己创建的 schema；不要指向生产数据库。覆盖迁移重复执行、注册上限、并发邀请、解绑版本、事件游标、到期邀请及跨账号设备权限。

本机的 `.tools/postgres-r2-data` 和 `couplelink_dev` 仅放测试账号；便携 PostgreSQL 不安装全局服务。开发准备脚本见 `scripts/prepare-test-postgres.ps1`，二进制来源 `@embedded-postgres/windows-x64@18.4.0-beta.17`（依赖安装在忽略的 `.tools/pg`）。它不是正式备份方案。

正式使用前必须用匹配 PostgreSQL 版本的 `pg_dump` 创建逻辑备份，在隔离数据库中用 `pg_restore` 真正恢复并核验账号、关系和事件游标，再安排加密、保留周期和异地保存。本轮尚未完成这项演练，不将构建成功描述为具备生产恢复能力。
