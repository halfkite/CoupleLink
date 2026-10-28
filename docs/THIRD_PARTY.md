# 第三方资源

## 1.0-r8 Web 安卓客户端

Vue 3.5.44 与 Capacitor 8.5.3 均为 MIT 许可，许可证副本见 `docs/licenses/vue-LICENSE.txt` 和 `docs/licenses/capacitor-LICENSE.txt`，发布目录一并提供。Vue 及其运行时间接依赖由 Vite 打入客户端，版本固定在 `apps/mobile-web/package-lock.json`。Vite、TypeScript、vue-tsc、tsx 和 Capacitor CLI 用于开发构建，不作为运行服务安装到手机。

应用与通知爱心图标由本项目代码绘制，并复用旧客户端图标。安卓依赖包含 AndroidX（Apache-2.0）及 Cordova Framework（Apache-2.0），许可证随 APK 的“我的 → 开源许可”及发布目录提供。新 Web 客户端使用系统字体；下列 Flutter 字体与依赖说明保留用于旧客户端和源码维护。

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

## 1.0-r4 文字聊天

Drift 2.35.1、drift_flutter 0.3.1、centrifuge 0.21.0 和 sqlite3.dart 3.5.2 使用 MIT 许可，完整许可位于 docs/licenses；Flutter 构建收集依赖许可。Centrifugo 6.10.2 使用 Apache-2.0，通过 Docker 拉取，不内嵌应用源码。没有复制 GPL-3.0 的 Togetherly 或 Tinode 业务代码。

Web drift_worker.js 复制自已锁定 drift 2.35.1 包根目录。sqlite3.wasm 来自官方 sqlite3.dart 的 sqlite3-3.5.2 GitHub Release： https://github.com/simolus3/sqlite3.dart/releases/download/sqlite3-3.5.2/sqlite3.wasm ，748686 字节，SHA-256 为 13d3f11d05b39ba0618a7115fb41640a5d48b6300f5d3f325f554b42bd6688a4。SQLite 核心采用公共领域许可；Dart 封装许可随包。更新 sqlite3 依赖时须同步 Web 二进制并重新核验。

## 1.0-r7 系统通知

flutter_local_notifications 22.3.3 使用 BSD-3-Clause，flutter_foreground_task 11.0.3 使用 MIT 许可；完整许可文本位于 docs/licenses 并随发布包提供。Android 构建启用 desugar_jdk_libs 2.1.4，其依赖随 Gradle 正常解析；完整依赖版本见锁文件。
