# GitHub 调研 · 2026-10-26

| 项目 | 参考与取舍 |
| --- | --- |
| https://github.com/THET1ME-1/Togetherly | Flutter 情侣空间、主题、小组件；PocketBase/Centrifugo/Sembast，依赖有兼容处理，社区较小；GPL-3.0 |
| https://github.com/owntracks/android | Kotlin、GMS/OSS 定位分层；参考采集上传分离；EPL-1.0 |
| https://github.com/Freika/dawarich | 轨迹、行程、家庭共享、导入导出；Rails/PostGIS/Redis/Sidekiq 整套较重；AGPL-3.0 |
| https://github.com/traccar/traccar | 围栏、轨迹事件；Java 通用追踪平台，关系授权需另建；Apache-2.0 |
| https://github.com/tinode/chat | 消息序列、多设备、回执；整套接入增加协议复杂度；GPL-3.0 |
| https://github.com/centrifugal/centrifugo | 私有频道、Dart SDK、重连；后续实时层；Apache-2.0 |
| https://github.com/Vincent-hechuan/codex-quota-band | AstroBox 安装、签名、Wearable SDK；AAR 需官方获取，双向交互另验；MIT |

本轮参考设计，未复制第三方应用源码或图片。

## 官方资料

- https://astrobox.online/zh-hk/downloads
- https://iot.mi.com/vela/quickapp/zh/guide/multi-screens/
- https://iot.mi.com/vela/quickapp/zh/guide/multi-screens/specs.html
- https://iot.mi.com/vela/quickapp/en/features/network/interconnect.html
- https://iot.mi.com/vela/quickapp/zh/tools/toolkit/start.html
- https://developer.android.com/develop/background-work/services/fgs/restrictions-bg-start

手环 10：212×520 像素、106dp；designWidth 为设计基准，本项目用 106。后续 APK 与 RPK 包名和签名保持匹配。
