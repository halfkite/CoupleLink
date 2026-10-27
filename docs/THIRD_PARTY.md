# 第三方资源

## Noto Sans SC

手机端随包提供 Noto Sans SC 可变字体，避免中文 UI 预览依赖在线字体回退。

- 来源：https://github.com/google/fonts/tree/main/ofl/notosanssc
- 原始文件：`NotoSansSC[wght].ttf`，本地仅重命名为 `NotoSansSC-Regular.ttf`，未修改字体内容。
- Git blob SHA-1：`fb0637bafbcd804fe32152370a1225990745b4bc`
- 许可：SIL Open Font License 1.1；完整文本位于 `apps/mobile/assets/fonts/OFL.txt`，也随应用资源打包。

## 图标与依赖

Material Icons 随 Flutter 的 Material 组件使用，依赖许可由 Flutter 许可注册机制收集。
应用启动图标、宠物和示意地图为本项目代码绘制。
其他依赖版本固定于 `pubspec.lock` 与各 `package-lock.json`，未复制调研项目的应用源码。

1.0-r2 新增 Flutter `http`（BSD-3-Clause）、`flutter_secure_storage`（BSD-3-Clause），服务端 `pgx/v5`（MIT）、Go `x/crypto`（BSD-3-Clause）；完整版本和间接依赖固定于 `pubspec.lock`、`server/go.mod` 与 `server/go.sum`。调试用便携 PostgreSQL 18.4 来自 [embedded-postgres](https://github.com/leinelissen/embedded-postgres)，只安装在忽略的工具目录，不随应用发布。PostgreSQL 与 Caddy 容器镜像由部署时拉取，分别遵守 PostgreSQL License 和 Apache-2.0。
