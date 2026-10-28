# 1.0-r9 直接震动调用修正（2026-10-28）

## 用户复测结论：仍未解决

用户随后明确反馈“我的 → 测试震动”依然没有实际震动。下文仅记录已经修正的源码调用路径，不能作为问题已解决的证据。前台本机测试不经过服务器或后台接收；尚缺该次设备实际执行路径和系统处理结果，不能确认最终根因。遵守用户不再调试手机的要求，未连接或操作手机，也未再次生成猜测性的试装包。

后续每次对外交付软件改动递增修订号，下一版1.0-r10；不再使用多个同版本修正APK区分交付。

【BUG表象】：用户反馈仍无法直接调用震动。“测试震动”按钮看起来像本机测试，实际走互动通知。

【真实根因】：已确认的接线错误是 App.vue 在 sound=false 时转去 native.interaction，原生 Vibrator 分支完全未被按钮调用；该分支又把主动前台触觉标记为通知用途，并以通知的静音/勿扰条件提前退出。以上属于确定的代码问题；真实手机未振动的全部原因仍未证实。

【修复方案】：同一个测试按钮直接调用 device.feedback(false) → CoupleDevice.feedback → DeviceRuntime.feedback → Vibrator；不发布通知，不检查通知许可，不注册或新增渠道。只允许前台调用，检查振动设备与 VIBRATE app-op；Android33+使用 VibrationAttributes.USAGE_TOUCH，较旧版本使用 AudioAttributes.USAGE_ASSISTANCE_SONIFICATION。声音测试仍调用原有响铃分支；收到消息和远程互动仍遵守统一通知渠道。没有更改手机或系统设置。

依据：[Android 官方触觉 API 文档](https://developer.android.com/develop/ui/views/haptics/haptics-apis)区分前台触觉与通知效果，并要求按用途标记。平台仍可依据触觉、勿扰或厂商策略抑制马达输出；未使用告警或电话用途绕过系统限制。

【修改代码】：direct-feedback.diff 为相对上次1.0-r9-fix交付的三处完整源码差异；source-changes.diff 为相对项目基线的完整源码差异。源码涉及 App.vue、strings.ts、DeviceRuntime.java。回归脚本：apps/mobile-web/tests/feedback-browser.cjs，可运行 node apps/mobile-web/tests/feedback-browser.cjs（需要工作区既有Playwright和本机Edge）。

【回归校验】：本机浏览器模拟 Android 桥，令通知权限为关闭，点击测试震动四次分别验证 requested、unsupported、suppressed、failed；记录均为feedback({sound:false})，未调用notify、interaction、permission或diagnostics；响铃仍调用feedback({sound:true})。23项TS回归、5项原生JVM策略测试、Vue生产构建和签名APK构建通过。此回归证明按钮接线和结果显示，不证明真实马达振动成功。没有使用ADB、连接设备或安装到手机。

## 下载与边界

版本仍为1.0-r9，同轮修正不递增修订号；本次目录为1.0-r9-direct-feedback，原始r9与r9-fix均保留。本次服务端无需更新，不影响已启动的热点服务。

安装后“我的 → 测试震动”是原生直接调用。提示“已请求系统执行”仅表示API请求已提交；如果实际仍无振动，不能宣称已经解决硬件问题。后台远程互动的通知振动不属于这次按钮接线修复的验收结论。
