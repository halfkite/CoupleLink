# CoupleLink · 1.0-r1

两人自用、Android 优先、自建服务器的情侣空间。

本轮为 **UI 测试版**：手机五栏、三套主题、首页卡片管理、演示互动与聊天、地图与轨迹演示，以及小米手环 10 八页 Vela 应用。演示数据不代表真实连接；不采集定位、不发送网络消息、不触发远程响铃。

## 版本规则

根目录 `VERSION` 是唯一版本来源。首轮 `1.0-r1`，后续对外交付依次 `1.0-r2`、`1.0-r3`；用户明确更换前保持主版本 `1.0`。同轮内部修正不另增修订号。Android/Vela `versionCode` 使用修订数字；npm/Flutter 机器版本适配为 `1.0.0-r1`，界面与文件名显示 `1.0-r1`。

## 工程

- `apps/mobile`：Flutter 手机应用，可使用 Web 构建进行桌面 UI 预览。
- `apps/band-vela`：Vela JS 快应用，212×520 屏幕、106 设计宽度。
- `packages/contracts`：共享动作标识与 UI 能力约定。
- `scripts`：版本检查、产物打包及构建脚本。
- `docs`：架构、调研、验收及后续开发范围。
- `artifacts`：安装包与 SHA-256 清单，不进入源码。

构建见 `docs/BUILD.md`，安装验收见 `docs/ACCEPTANCE.md`。

手环 RPK 用 AstroBox 安装。后续目标链路为 CoupleLink 安卓桥接 → 小米运动健康 → Vela。本轮不声明具备真实通信能力，SDK 获取、签名、权限及真机双向交互需独立验证。
