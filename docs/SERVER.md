# CoupleLink 1.0-r28 服务器：网页、管理页与备份

## 打开方式

- 网页（情侣端聊天）：`http://127.0.0.1:30710/`；手机或局域网用电脑 IP，例如 `http://192.168.137.1:30710/`。Web 包已嵌入服务端二进制，根路径直接可用。
- 管理页：`http://127.0.0.1:30710/admin`（网页“我的”里也有入口）。本机和局域网都能打开，但需要登录。

## 管理页账号密码

- 默认账号 `admin`，12 位初始密码由服务器生成。
- **每次启动都会在控制台打印当前密码**，同时写入密码文件 `.tools/local-server/admin.txt`（可用 `COUPLELINK_ADMIN_FILE` 指定其他路径）。
- 在网页里改了密码后，下一次控制台输出和密码文件都会同步更新；忘记密码时设置 `COUPLELINK_ADMIN_PASSWORD=新密码` 再启动即可重置。
- 首次登录的「修改账号密码」页面可以同时改账号（3–32 位小写字母、数字、下划线或短横线）；改名后控制台输出、密码文件和下次登录都用新账号。
- 为了能再次显示，数据库额外保存一份明文（`password_plain`），密码文件也是明文——只应留在本机，不要把这两个文件发给别人。
- 登录密码本身用 Argon2id 校验；管理页登录状态保存在 HttpOnly、SameSite=Lax 的 Cookie 里，12 小时过期。

## 密钥

- 每个密钥对应一个情侣组，明文保存在数据库，管理页任何时候都能查看，点击即可复制。
- 第一台登录的设备成为成员 A，第二台成为成员 B；同一台设备重复登录保持原成员身份。
- 登录时可以选择男/女，只是显示标签，两台设备各自选择。
- 1.0-r12 之前创建的旧密钥只保存了哈希、无法还原明文，管理页会标注“旧密钥不可显示”，重新生成即可。

## 回收站

- 管理页对某个组点「移入回收站」后，该组密钥立即失效，但组、密钥、备注和聊天记录都保留在管理页底部的「回收站」区域。
- 回收站里的组可以「恢复」（密钥重新可用），也可以「永久删除」（连同该组两个账号、设备、会话、聊天和纪念日一起删除，不可恢复）。

## 备注

- 双方各自可以给 TA 设置备注（40 字以内），保存在服务器，双方都能看到，也会写入导出的备份。

## 数据备份（浏览器内）

- 导出：管理页“导出全部数据（JSON）”，浏览器直接下载 `couplelink-backup-*.json`，包含组信息、密钥明文、备注和全部聊天记录。
- 导入：选择文件并选择“合并”或“覆盖”。合并保留现有数据、只补缺失的组和聊天；覆盖会清空现有组、密钥、聊天后导入。
- 导入会校验 `format=couplelink-backup` 和 `version`；版本高于程序支持范围时拒绝导入，并提示先升级服务端。
- 覆盖导入会一并清空设备与登录会话，手机需要用密钥重新登录。
- 单个组也可以单独备份：每个组卡片上的「导出该组」生成 `couplelink-group` 格式（含该组成员、密钥、备注和全部聊天），「导入聊天」把这个文件里的聊天合并进当前组。导入时按成员位置对应双方，重复导入不会重复插入；跨组导入的消息会重新编号，不影响原组。
- 整库导出会包含回收站里的组（标记 `archived`），导入时还原到回收站。

## 局域网

- 服务端默认监听 `0.0.0.0:30710`，启动时会把本机可用的局域网地址直接打印在控制台（`局域网: http://192.168.137.1:30710/ ...`）。管理页不再只限本机，但由账号密码保护，请只在可信局域网使用；外网访问必须在路由器上把该端口映射到本机，否则公网 IP 无法直连。
- 手机连不上时先确认两件事：①手机和电脑在同一网段（校园网/公司 Wi-Fi 常有 AP 隔离，建议改用电脑热点 `192.168.137.1`）；②Windows 防火墙放行了 TCP 30710——运行 `.tools\local-server\open-lan-port.ps1`（会弹 UAC）或手动执行：

```powershell
New-NetFirewallRule -Name 'CoupleLink-LAN-TCP-30710' -DisplayName 'CoupleLink LAN TCP 30710' -Direction Inbound -Action Allow -Protocol TCP -LocalPort 30710 -RemoteAddress LocalSubnet -Profile Any
```

- 安卓客户端自 r13 起允许任意 HTTP 地址（内网和公网 IP 都可以），内网 IP 变了也不用重新打包；公网走 HTTP 是明文，能用 HTTPS 就优先 HTTPS。
- 服务端会把来自其他设备的页面访问和管理登录写进控制台/`server.log`（`访问 from=...`），可以据此判断是网络没通还是 App 拒绝。
- 首次可能需要在管理员 PowerShell 中放行防火墙：

```powershell
cd "D:\ai\ds\新建文件夹"
.\scripts\enable-lan-access.ps1 -BindAddress 192.168.137.1 -Port 30710
```

- Android 客户端只允许安装包内配置的明文 HTTP 地址（localhost、127.0.0.1、10.0.2.2、10.74.205.253、192.168.137.1）；用手机浏览器打开网页不受该限制。

## 一键构建（含服务器）

`npm run release` 依次构建 Web 包（嵌入服务端二进制）、Android Release APK、Windows／Linux 服务端二进制，并复制到 `artifacts/<版本>/`，生成 `SHA256SUMS.txt`。需要 `JAVA_HOME`、`ANDROID_HOME` 与 `GO`（或 PATH 中的 `go`）。

---
# CoupleLink 1.0-r7 服务器测试部署

本轮提供账号、情侣关系、设备会话、真实文字聊天、互动和前后台同步；后台接收需用户在手机主动开启。定位、媒体、专用推送平台及手环桥接没有接入。本版本用于隔离测试环境；已验证一次本机升级前备份恢复，但自动生产备份与正式安全审计尚未完成。

## Docker Compose

使用发布包的 `server-bundle` 或源码根目录，在其中的 `deploy` 下复制 `.env.example` 为 `.env`。生成独立的数据库密码及两个彼此独立的实时服务密钥，每项至少 32 个随机字节转十六进制；不要使用示例值。数据库密码仅用 URL 安全字符。r3 注册不需要设置码，也没有账号数量上限；每对情侣仍由两人绑定，数据按账号和关系隔离。

```powershell
cd server-bundle/deploy
Copy-Item .env.example .env
# 编辑 .env，填入独立随机值、你控制的域名。
docker compose up -d --build
```

默认本机 API 为 `http://127.0.0.1:18080`，`GET /healthz` 应返回 `status: ok`、`api_version: 1`。数据库不发布主机端口，容器 API 以非 root 账号运行。局域网手机接入可设置 `COUPLELINK_BIND_IP` 为电脑的 Wi-Fi IPv4 地址，端口由 `COUPLELINK_PORT` 配置。Android 模拟器使用 `http://10.0.2.2:18080`，实体手机使用电脑 IP，不能使用手机自身的 localhost 或数据库端口 5432。

## 本机手机联调启动

从源码根目录执行 `./scripts/start-local-server.ps1 -BindAddress 电脑的Wi-Fi地址`。脚本生成私有数据库口令，使用独立的 `couplelink-local` Compose 项目和数据卷，等待真实健康检查通过，不使用其他目录现有的数据库。

需要放行 Windows 入站时，以管理员权限执行 `./scripts/enable-lan-access.ps1 -BindAddress 电脑的Wi-Fi地址`，只为该 IP 的 TCP 18080 允许 LocalSubnet。撤销可执行 `Remove-NetFirewallRule -Name CoupleLink-Local-TCP-18080`。不会关闭系统防火墙。

本轮 r7 安装包允许 `192.168.137.1` 和 `10.74.205.253` 两个指定局域网 HTTP 地址。电脑当前服务器绑定热点 `192.168.137.1`，手机连上电脑热点后填写 `http://192.168.137.1:18080`，点击“测试连接”；也可先在手机浏览器访问该地址加 `/healthz`。当前服务器没有监听旧 Wi-Fi 地址，切回旧 Wi-Fi 需要重新绑定。HTTP 会明文传输数据，其他地址及公网仍需 HTTPS 或重新配置指定私网地址。

电脑 IP 改变后应重新启动服务并调整限定防火墙规则。Android 的精确 HTTP 例外属于安装包配置，需重新构建对应地址或改用有效 HTTPS 地址。不要把 0.0.0.0 填入手机，它只表示服务监听范围。

启用 HTTPS 前将域名 A/AAAA 指向服务器，并开放 80/443；`COUPLELINK_DOMAIN` 不带协议或路径：

```powershell
docker compose --profile https up -d --build
```

Caddy 申请证书并代理 API。证书申请、域名和外网访问需在实际服务器验证，本机无法代替公网验收。原生 Android 无需 CORS 配置；浏览器联调须将完整来源（例如 `http://127.0.0.1:5178`）加入空格分隔的 `COUPLELINK_ALLOWED_ORIGINS`，没有通配来源。

不要执行 `docker compose down -v`，该命令会删除数据库卷。发布文件及普通日志不包含数据库密码、会话令牌或精确位置。

## 独立运行

提供 Windows x64 与 Linux x64 二进制，迁移 SQL 和时区库已嵌入。安装 PostgreSQL 17 或 18 并新建专用数据库，使用进程环境配置 `DATABASE_URL`；可选 `COUPLELINK_LISTEN_ADDR`（默认 `127.0.0.1:18080`）。不再需要 `COUPLELINK_SETUP_CODE`。不要将包含密码的连接串写入命令行、共享截图或公开日志。Linux 二进制可能需先赋予执行权限。公网由反向代理提供 HTTPS。

启动时迁移持有数据库事务锁，已应用迁移不会重复执行；数据库版本高于程序支持版本时拒绝启动。正式升级前应备份数据库，而不是直接用旧程序覆盖新数据库。

## 手机联调

1. 我的 → 顶部双人卡片，填写有效 HTTPS 地址或当前安装包允许的局域网 HTTP 地址；切换注册，填写账号、昵称及至少 8 位的密码。没有设置码或两个账号的数量限制。
2. 一人生成邀请，另一人输入邀请、恋爱起点和 IANA 时区（如 `Asia/Hong_Kong`）。邀请 15 分钟有效，单次使用；新邀请立即撤销旧邀请。
3. 双方点击“刷新同步”，查看真实昵称、恋爱起点与天数。绑定后进入消息页即可真实文字聊天；前台有实时提醒和轮询兜底，后台通知尚未接入。
4. 撤销某个设备后，它下一次请求立即被服务端拒绝；解绑立即结束服务端关系，另一端刷新后清除关系。离线客户端仍可能显示旧缓存，不能承诺离线屏幕立即消失。
5. Android 会话保存到系统安全存储；浏览器测试只放内存，刷新页面需要重新登录。密码不会本地保存。

## 回归与备份边界

`server/internal/api/integration_test.go` 需要 `COUPLELINK_TEST_DATABASE_URL`。测试只创建随机临时 schema，并清理自己创建的 schema；不要指向生产数据库。覆盖迁移重复执行、第三账号无设置码注册、重复账号拒绝、并发邀请、解绑版本、事件游标、到期邀请及跨账号设备权限。

本机的 `.tools/postgres-r2-data` 和 `couplelink_dev` 仅放测试账号；便携 PostgreSQL 不安装全局服务。开发准备脚本见 `scripts/prepare-test-postgres.ps1`，二进制来源 `@embedded-postgres/windows-x64@18.4.0-beta.17`（依赖安装在忽略的 `.tools/pg`）。它不是正式备份方案。

正式使用前必须用匹配 PostgreSQL 版本的 `pg_dump` 创建逻辑备份，在隔离数据库中用 `pg_restore` 真正恢复并核验账号、关系和事件游标，再安排加密、保留周期和异地保存。本机 r3→r4 升级前已经用专用数据库执行过一次隔离恢复、schema 版本核对和加密往返核验。备份密钥单独保存于 .secrets/backup-keys；备份与密钥均不随发布包交付。这不代表已具备定时、异地和长期生产恢复能力。

## r4 实时服务与数据库升级

Compose 增加 Centrifugo 6.10.2 容器；只在内部网络开放，通过 Go API 的 /connection/websocket 复用手机填写的端口，不需要额外开放防火墙端口。数据库仍是消息的唯一事实来源，Centrifugo 只发送无正文的刷新提醒。

独立 Go 二进制可仅使用数据库并退回前台轮询；若启用实时服务，须同时配置 COUPLELINK_REALTIME_URL、COUPLELINK_REALTIME_API_KEY、COUPLELINK_REALTIME_SECRET，并使 Centrifugo 使用相同的对应密钥及 deploy/centrifugo.json。任一配置遗漏时程序会拒绝不完整配置。密钥不要写在命令行或发布包。

COUPLELINK_ALLOWED_ORIGINS 同时传给 Go 与 Centrifugo，格式是空格分隔的完整来源，不能写成 JSON 数组。Android 原生不发送浏览器 Origin，不需添加任意通配来源。

r4 使用 schema 2；001 账号迁移与 002 聊天迁移均嵌入二进制，启动时在锁内依次应用。升级前先备份并真实恢复核验；不能用只支持 schema 1 的旧程序直接降级覆盖。离线消息、设备会话与解绑行为见文字聊天说明。

## r5 互动迁移

003_interactions.sql 升级数据库至 schema 3，包含动作、过期时间、接收设备及反馈结果。发送、领取和回执均校验现有会话与关系；同一事件只允许一台接收设备领取。过期事件不会再播放。PostgreSQL 是事件事实来源，实时服务只发送刷新提示；前台每 2 秒兜底拉取。后台推送未接入。升级前已对本机专用库完成加密备份与隔离恢复核验；旧 r4 程序不能直接访问新 schema。
