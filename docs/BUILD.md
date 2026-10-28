# 构建 CoupleLink 1.0-r9

当前安卓发布入口改为 `apps/mobile-web`，使用 JDK 21 和 `scripts/build-web-android.ps1`。详细步骤见 [Web 安卓客户端](WEB-ANDROID.md)。以下 Flutter 步骤保留用于旧客户端维护，不再作为 r8 默认 APK 来源。

## 固定环境

- Flutter 3.47.8 stable / Dart 3.13.7
- Node.js 24，npm；Vela `aiot-toolkit` 2.0.5
- JDK 17、Android SDK 36 / 35、Build Tools 36.0.0、NDK 28.2.13676358、CMake 3.22.1
- Gradle 9.3.1、Android Gradle Plugin 9.1.0（Flutter 模板配置）
- Go 1.27.3；数据库集成回归需要隔离的 PostgreSQL 17/18

首次构建前通过 Android SDK Manager 安装上述依赖并接受相应许可。`apps/mobile/android/local.properties` 为机器本地路径，不提交。

在 PowerShell 中从根目录运行：

```powershell
./scripts/build.ps1 -FlutterDirectory '你的Flutter路径' -JavaDirectory '你的JDK17路径' -AndroidSdkDirectory '你的AndroidSDK路径' -Preview
```

也可使用本项目 `.tools/` 下已准备好的本地环境直接运行 `./scripts/build.ps1 -Preview`。脚本执行依赖解析、静态检查、测试、签名构建、RPK 构建和产物校验；任一步失败即停止。

## 单项命令

```powershell
node scripts/check-contracts.mjs
cd apps/mobile
flutter analyze
flutter test
flutter build apk --release
flutter build web --no-web-resources-cdn
```

Vela 工程中执行 `npm ci` 与 `npm run release`。调试用 `npm run build` 会使用工具链调试证书，不能替代配套 Release 签名包。

## 签名

`scripts/prepare-signing.ps1` 首次生成本地 RSA 密钥，导出同证书的 Vela PEM。密钥及随机口令保存在忽略的 `.secrets/`；后续构建复用，不覆盖已有密钥。请将该目录单独安全备份，保留手机和手环覆盖更新的能力，切勿加入 Git 或交付压缩包。

公开信息可用 Android `apksigner verify --print-certs` 和 OpenSSL 读取证书指纹。RPK 签名证书与 APK 相同，不表示本轮已经接入 Wearable SDK。

## 预览

完成 Web 构建后，从根目录执行 `node scripts/serve-preview.mjs`，打开 http://127.0.0.1:5178 。服务仅监听本机。
`node scripts/inspect-preview.mjs` 使用 Playwright 与本机 Edge 生成截图；可通过 `COUPLELINK_EDGE` 指定其他兼容 Chromium 路径。

## 版本

本轮所有发布名称为 `1.0-r7`，Flutter/npm 的机器格式为 `1.0.0-r7`。后续使用 `node scripts/set-revision.mjs N` 同步应用、服务端、锁文件和显示版本；重编本轮包不递增。未经用户明确要求不改主版本 `1.0`。

服务端可单独执行 `./scripts/build-server.ps1 -GoCommand 'Go可执行文件路径'`，输出 Windows/Linux x64 二进制到 `.tools/server-release`。设置 `COUPLELINK_TEST_DATABASE_URL` 后脚本会运行真正的数据库集成测试；未设置时该项会明确跳过。完整构建脚本支持同名 `-GoCommand` 参数。

## 指定局域网地址

`./scripts/build.ps1 -LocalHttpHosts '你的服务器私有IPv4地址'` 将同一配置交给 Flutter 和 Android。直接 Flutter 构建时使用 `--dart-define=COUPLELINK_LOCAL_HTTP_HOSTS=地址`。支持逗号分隔的明确地址，拒绝公共 IP、通配域和整个网段。

Android Gradle 的 Variant API 注册生成资源任务，生成精确域名规则；静态和生成的 base-config 都保持 cleartextTrafficPermitted=false。没有配置时只允许原有回环调试地址。本轮构建值是 10.74.205.253,192.168.137.1；它是部署配置，不写死在业务代码中。

## 文字聊天依赖与联调

Drift 2.35.1、drift_flutter 0.3.1、centrifuge 0.21.0 固定于 pubspec，实际 SQLite 版本固定于锁文件。Web 必须部署匹配的 drift_worker.js 与 sqlite3.wasm；版本、来源和散列见第三方声明。Android 原生 SQLite 由依赖的构建钩子打包。

独立测试服务器建议使用 `start-local-server.ps1 -ProjectName couplelink-qa-r4 -BindAddress 127.0.0.1 -Port 18081`。仅在这个隔离服务上设置 `COUPLELINK_TEST_API_ORIGIN=http://127.0.0.1:18081` 后运行 `node scripts/inspect-chat.mjs`；该脚本会创建测试账号、消息与绑定，不要指向个人实际使用库。
