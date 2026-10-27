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
