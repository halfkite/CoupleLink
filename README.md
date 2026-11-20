# CoupleLink · 1.0-r12

两人自用、Android 优先、自建服务器的情侣空间。

本轮（r12）：账号改为「一个组一个密钥」，服务器根路径直接打开情侣端网页，`/admin` 是管理页；管理页用 `admin` 账号登录，初始密码在首次启动的服务端控制台生成并要求登录后立即修改。密钥明文可在管理页随时查看、点击复制；登录时可选择男/女标签，双方可以给对方设置备注（服务器保存）；管理页支持浏览器内导出/导入 JSON 备份（含组信息、密钥、备注与全部聊天，导入校验版本并支持覆盖或合并）；管理页和登录页都适配手机浏览器，局域网可直接访问。`npm run release` 一次输出 Web 包、签名 APK 和 Windows/Linux 服务端二进制。详见 `docs/SERVER.md`。

本轮为 **Web 安卓客户端测试版**：Vue + Capacitor 页面随 APK 离线打包，复用 Go/PostgreSQL 服务；真实文字聊天、离线队列、未读红点和情侣互动接入安卓原生通知、振动与响铃。注册密码至少 8 位，允许纯数字；账号没有数量上限和设置码。位置、宠物、媒体和手环通信尚未接入。迁移与使用边界见 `docs/WEB-ANDROID.md`。

## 版本规则

根目录 `VERSION` 是唯一版本来源。当前 `1.0-r12`，后续 `1.0-r13`；用户明确更换前保持主版本 `1.0`。同轮内部修正不另增修订号。Android/Vela `versionCode` 使用修订数字；npm/Flutter 使用 `1.0.0-rN`，界面与文件名显示 `1.0-rN`。

本轮账号页新增独立连接测试。此 Android 包允许电脑热点 `192.168.137.1` 与原 Wi-Fi 地址 `10.74.205.253` 的 HTTP；当前服务器监听热点地址。

## 工程

- `apps/mobile-web`：当前 Vue + Capacitor 安卓客户端，包含 Java 原生通知、后台接收、安全存储和反馈桥接。
- `apps/mobile`：保留的 Flutter 客户端与旧本机缓存；本轮也补齐通知图标资源保留规则。
- `apps/band-vela`：Vela JS 快应用，212×520 屏幕、106 设计宽度。
- `packages/contracts`：共享动作标识与 UI 能力约定。
- `server`、`deploy`：Go API、数据库迁移与 Compose/HTTPS 配置；部署边界见 `docs/SERVER.md`。
- `scripts`：版本检查、产物打包及构建脚本。
- `docs`：架构、调研、验收及后续开发范围。
- `artifacts`：安装包与 SHA-256 清单，不进入源码。

构建见 `docs/BUILD.md`，安装验收见 `docs/ACCEPTANCE.md`。

手环 RPK 用 AstroBox 安装。后续目标链路为 CoupleLink 安卓桥接 → 小米运动健康 → Vela。本轮不声明具备真实通信能力，SDK 获取、签名、权限及真机双向交互需独立验证。

## License / 使用许可

CoupleLink 原创代码采用 [非商业源码许可证](LICENSE)：除原作者 halfkite 外，其他人不可商业使用，除非取得作者另行书面授权。第三方依赖保留各自许可证，见 docs/THIRD_PARTY.md。本项目不属于 OSI 定义的开源软件。

