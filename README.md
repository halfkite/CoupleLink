# CoupleLink · 1.0-r3

两人自用、Android 优先、自建服务器的情侣空间。

本轮为 **账号与互动测试版**：保留手机五栏、三主题、手环八页 UI、本机反馈及真实账号/情侣绑定。r3 按用户要求取消服务器账号数量上限与注册设置码，并支持为指定局域网服务器生成精确 HTTP 例外。服务器启动与手机填写方式见 `docs/SERVER.md`。聊天、位置、宠物和手环通信仍为演示。

## 版本规则

根目录 `VERSION` 是唯一版本来源。首轮 `1.0-r1`，当前 `1.0-r3`，后续 `1.0-r4`；用户明确更换前保持主版本 `1.0`。同轮内部修正不另增修订号。Android/Vela `versionCode` 使用修订数字；npm/Flutter 使用 `1.0.0-rN`，界面与文件名显示 `1.0-rN`。

## 工程

- `apps/mobile`：Flutter 手机应用，可使用 Web 构建进行桌面 UI 预览。
- `apps/band-vela`：Vela JS 快应用，212×520 屏幕、106 设计宽度。
- `packages/contracts`：共享动作标识与 UI 能力约定。
- `server`、`deploy`：Go API、数据库迁移与 Compose/HTTPS 配置；部署边界见 `docs/SERVER.md`。
- `scripts`：版本检查、产物打包及构建脚本。
- `docs`：架构、调研、验收及后续开发范围。
- `artifacts`：安装包与 SHA-256 清单，不进入源码。

构建见 `docs/BUILD.md`，安装验收见 `docs/ACCEPTANCE.md`。

手环 RPK 用 AstroBox 安装。后续目标链路为 CoupleLink 安卓桥接 → 小米运动健康 → Vela。本轮不声明具备真实通信能力，SDK 获取、签名、权限及真机双向交互需独立验证。
