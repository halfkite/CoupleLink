# 1.0-r10 震动根因修复（2026-11-15）

## 结论：根因已由真机系统证据确认

上一版把“测试震动”接到原生 `Vibrator` 的修复确实进了交付包（解开 1.0-r9-direct-feedback 的 APK，`assets/public/assets/index-*.js` 里按钮走的是 `feedback({sound:false})`，不是通知渠道），但手机仍然不振动。本轮用只读诊断在真机上定位到系统层原因，并据此修复。

诊断设备：Xiaomi 24122RKC7C，HyperOS 3（OS3.0.305.0.WOMCNXM），Android 16 / API 36。

【BUG表象】点击「我的 → 测试震动」提示“已请求系统执行，请检查实际反馈”，手机没有任何振动；后台接收正常，系统通知有提示音但没有振动。

【真实根因】两个独立缺陷叠加，均已在真机上取到证据：

1. **前台反馈使用了触感用途，被“系统触感”开关整类丢弃。**
   r9 为了区分“前台触觉”和“通知”，把直接反馈改成 `VibrationAttributes.USAGE_TOUCH`。该机的 `settings get system haptic_feedback_enabled` 为 `0`（系统触感关闭），`dumpsys vibrator_manager` 随之显示 `TOUCH = OFF, default: MEDIUM`（`UNKNOWN` 同为 OFF，通知/响铃/闹钟为 MEDIUM）。logcat 记录应用（uid=10537 = `app.couplelink.mobile`）当天三次请求全部被系统丢弃：

   ```
   Vibration for uid=10537 and with attrs=VibrationAttributes{mUsage=TOUCH, ...}
   ended with status IGNORED_FOR_SETTINGS
   ```

   即：应用调用成功、系统受理、马达未被驱动，`Vibrator.vibrate()` 不抛异常也不返回失败，所以 UI 只能显示“已请求”。用户自行打开“系统触感”后，同一个按钮立即振动，确证该门控就是直接原因。

2. **通知类别启用了振动，却没有振动模式。**
   `dumpsys notification --noredact` 实测渠道 `couplelink_alerts_v1` 为 `mVibrationEnabled=true, mVibrationPattern=null, mVibrationEffect=null`。渠道只“允许振动”而不给出模式时，平台不会为通知提交振动请求（落回通知自身的空振动），于是“有通知、有提示音、无振动”。这也解释了后台互动一直没有振动。

【修复方案】

- 应用内反馈不再使用 `USAGE_TOUCH`，改为与提醒语义一致的**通知用途**：Android 13+ 用 `VibrationAttributes.USAGE_NOTIFICATION`，8.0–12 用 `AudioAttributes.USAGE_NOTIFICATION`，7.1 及以下用 `vibrate(long[], int)`；统一显式波形 `{0, 250, 150, 250}`。这样前台反馈不再受“系统触感”这类触感开关影响，也与真实提醒走同一类系统策略。
- 通知渠道升级为 `couplelink_alerts_v2`，创建时显式 `enableVibration(true)` + `setVibrationPattern(...)`。渠道创建后振动设置不可变，所以必须换 ID；旧的 `couplelink_alerts_v1` 与既有历史类别按原逻辑删除，不重建用户已调过的其它类别。
- 诊断新增「系统触感」（读取 `Settings.System.HAPTIC_FEEDBACK_ENABLED`），关闭时在诊断区给出明确说明；技术诊断 JSON 同时输出渠道是否带振动模式，便于下次直接判断。

【修改代码】

- `apps/mobile-web/android/app/src/main/java/app/couplelink/mobile/InteractionAlert.java`：渠道 ID v2、共享振动波形常量。
- `apps/mobile-web/android/app/src/main/java/app/couplelink/mobile/DeviceRuntime.java`：反馈改用通知用途；渠道显式振动模式；清理 v1；诊断增加“系统触感”和渠道振动模式。
- `apps/mobile-web/src/App.vue`、`src/device.ts`、`src/strings.ts`：版本 1.0-r10，诊断区新增“系统触感”与提示。
- `apps/mobile-web/android/app/src/test/java/app/couplelink/mobile/InteractionAlertTest.java`：新增渠道带振动波形的回归断言。
- `scripts/set-revision.mjs`：补齐漏掉的 `apps/mobile-web` 版本文件，避免下次改版本号时合同检查失败。

【回归校验】

- 23 项 TS 单元测试、`vue-tsc --noEmit`、Vite 生产构建通过。
- 6 项原生 JVM 策略测试通过（含新增的渠道振动波形断言）。
- Release APK 以同一发布证书签名，`versionCode=10`、`versionName=1.0-r10`。
- 真机验收：安装 r10 后点击「测试震动」，再用只读 logcat 确认出现 `mUsage=NOTIFICATION` 且状态为播放完成（而不是 `IGNORED_FOR_SETTINGS`）。本次只读诊断没有修改手机任何设置；是用户自己打开了“系统触感”，本轮修复不依赖该设置。

## 边界

- 系统“振动/系统触感”全部关闭时，任何应用都无法让马达工作；应用能做的是改用正确用途、给出真实状态，而不是伪造成功。
- 通知类别升级到 v2 会重置该类别上一次的系统级自定义（振动模式、重要性），这是修改振动模式的必要代价；其它类别不受影响。
- 本轮的“已请求”仍然是调用回执；真机是否振动以安装后的 logcat 记录为准，不以构建成功代替。

---

# 1.0-r9 直接震动调用修正（2026-10-28）

## 历史：当时仍未解决

用户随后明确反馈“我的 → 测试震动”依然没有实际震动。下文仅记录已经修正的源码调用路径，不能作为问题已解决的证据。该次设备实际执行路径与系统处理结果在 1.0-r10 才取到。

【BUG表象】：用户反馈仍无法直接调用震动。“测试震动”按钮看起来像本机测试，实际走互动通知。

【真实根因】：已确认的接线错误是 App.vue 在 sound=false 时转去 native.interaction，原生 Vibrator 分支完全未被按钮调用；该分支又把主动前台触觉标记为通知用途，并以通知的静音/勿扰条件提前退出。以上属于确定的代码问题；真实手机未振动的全部原因当时仍未证实。

【修复方案】：同一个测试按钮直接调用 device.feedback(false) → CoupleDevice.feedback → DeviceRuntime.feedback → Vibrator；不发布通知，不检查通知许可，不注册或新增渠道。只允许前台调用，检查振动设备与 VIBRATE app-op；Android33+使用 VibrationAttributes.USAGE_TOUCH，较旧版本使用 AudioAttributes.USAGE_ASSISTANCE_SONIFICATION。声音测试仍调用原有响铃分支；收到消息和远程互动仍遵守统一通知渠道。没有更改手机或系统设置。

依据：[Android 官方触觉 API 文档](https://developer.android.com/develop/ui/views/haptics/haptics-apis)区分前台触觉与通知效果，并要求按用途标记。平台仍可依据触觉、勿扰或厂商策略抑制马达输出；未使用告警或电话用途绕过系统限制。

【修改代码】：direct-feedback.diff 为相对上次1.0-r9-fix交付的三处完整源码差异；source-changes.diff 为相对项目基线的完整源码差异。源码涉及 App.vue、strings.ts、DeviceRuntime.java。回归脚本：apps/mobile-web/tests/feedback-browser.cjs，可运行 node apps/mobile-web/tests/feedback-browser.cjs（需要工作区既有Playwright和本机Edge）。

【回归校验】：本机浏览器模拟 Android 桥，令通知权限为关闭，点击测试震动四次分别验证 requested、unsupported、suppressed、failed；记录均为feedback({sound:false})，未调用notify、interaction、permission或diagnostics；响铃仍调用feedback({sound:true})。23项TS回归、5项原生JVM策略测试、Vue生产构建和签名APK构建通过。此回归证明按钮接线和结果显示，不证明真实马达振动成功。没有使用ADB、连接设备或安装到手机。