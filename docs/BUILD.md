# 构建 CoupleLink 1.0-r1

## 固定环境

- Flutter 3.47.8 stable / Dart 3.13.7
- Node.js 24，npm；Vela `aiot-toolkit` 2.0.5
- JDK 17、Android SDK 36 / 35、Build Tools 36.0.0、NDK 28.2.13676358、CMake 3.22.1
- Gradle 9.3.1、Android Gradle Plugin 9.1.0（Flutter 模板配置）

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

本轮所有发布名称为 `1.0-r1`，Flutter/npm 的机器格式为 `1.0.0-r1`。下次对外交付再递增修订号；重编本轮包不递增。未经用户明确要求不改主版本 `1.0`。
