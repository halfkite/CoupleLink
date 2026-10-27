# 1.0-r1 验证记录

日期：2026-10-27，Asia/Hong_Kong。

## 已完成

- Flutter 3.47.8 静态检查：无问题。
- Flutter 自动化测试：14 项全部通过。
  - 三主题 × 中文标准字号/中文 200%/英文 200%，320×700 的五个主页面。
  - 损坏或非对象偏好回退；重复/未知卡片、无效语言与文案的恢复边界。
  - 本机演示消息与草稿在切换一级页面后保留。
  - 互动预览明确不是真实投递。
  - 快速修改主题、文案、卡片顺序后最新状态正确保存。
- 版本契约：1.0-r1；203 组中英文 Key 与占位符一致；四种动作标识唯一；八条 Vela 路由对应页面存在。
- Android Release APK 构建成功，包名 `app.couplelink.mobile`，versionName `1.0-r1`，versionCode `1`。
- APK 最低 API 24，target/compile API 36；安装包未声明定位、通知、录音、相册或网络权限。
- APK 签名验证通过；与 Vela Release PEM 的 SHA-256 指纹一致：
  `55ef3a3eddc40216606490cf6e0724519d5327503eca72f1d5a6db0f7fed078c`。
- Vela 2.0.5 Release RPK 构建成功；原生 `.ux` 页面，不是截图打包。
- Flutter Web 构建成功；五页与三主题使用本地 Edge 检查并截图，无 JavaScript 页面异常。复测时阻止全部外部请求，本地渲染资源和随包中文字体仍正常显示。
- Noto Sans SC 字体与官方 Git blob SHA-1 `fb0637bafbcd804fe32152370a1225990745b4bc` 一致，OFL 许可随包附带。

## 已处理的问题及根因

【BUG表象】：窄屏页头溢出、地图面板提示 ListTile 绘制层不正确；双倍字号的纪念日和英文聊天页溢出；跨测试用例加载语言文件停滞；首次切页截图中文字形缺失。

【真实根因】：部分固定横排与固定头部未给放大文本留出可滚动空间；地图背景 DecoratedBox 遮住了上层 ListTile 的 Material 绘制；测试资产缓存 Future 绑定旧的虚拟时钟环境；Web 中文回退字体依赖在线按需加载。

【修复方案】：页头提供弹性宽度；地图列表增加自己的透明 Material；纪念日数据与状态分行；聊天标题及资料纳入消息滚动区域，输入框提示限制为单行；测试资产在 setUpAll 预加载；中文字体随应用分发。

【修改代码】：对应 `apps/mobile/lib/ui/widgets.dart`、`features/map_page.dart`、`features/memories.dart`、`features/messages.dart`、`core/theme.dart`、`main.dart`、`test/ui_test.dart` 与 `pubspec.yaml`。工程为首次实现，无已有业务代码可对比；具体实现保留在这些文件中。

【回归校验】：上述 14 项自动化测试、静态检查及实际浏览器截图均通过。设置、地图、消息、纪念日和主页在三主题与中英文大字号组合下重新验证。

## 尚未验证

- Android 真机安装、键盘/触摸/屏幕阅读器及长时间运行。
- AstroBox 实际安装、手环固件兼容、胶囊屏边缘和字体可读性、返回手势、覆盖升级。
- 实际定位、消息推送、官方 SDK 获取、手环双向通信、振动、健康数据等后续能力。
- GitHub Actions 文件已提供，本轮没有连接远程仓库，未声称 CI 在 GitHub 上执行成功。

浏览器截图用于手机 UI 视觉检查，不能替代 Android 或 Vela 真机验收。
