# Docker Desktop 启动故障记录 · 2026-10-27

【BUG表象】：Docker Desktop 4.86.0.236216 启动弹窗，初始化 Inference manager 时不能删除 `Docker/run/dockerInference`；Windows 报文件无法访问及名称格式错误。

【真实根因】：已定位的故障环节是 Docker 的 Windows 本地通信端点清理/重建失败，导致引擎启动被中止；不是 CoupleLink API 或 Compose 容器报错。端点是零字节且带 ReparsePoint 属性，单凭该属性不能断定文件系统损坏或权限问题。端点为什么变得无法访问，尚未在此机器完成底层定因。

【修复方案】：先确认 Docker 进程退出，保留原运行目录再重建。先后遇到 `dockerInference` 和 `docker-secrets-engine/engine.sock` 错误，后来又在新运行目录复发，证明目录隔离只是一次性绕过，未构成根因修复。保留了两个 `*.couplelink-backup-20261027-r2` 运行目录，没有删除镜像、容器、卷或执行恢复出厂设置。

Docker 维护者在 [官方问题 #625](https://github.com/docker/desktop-feedback/issues/625) 中表示 4.92 已提供这一类问题的修复。基于此准备官方 4.96.3 覆盖更新；[版本来源](https://docs.docker.com/desktop/release-notes/#4963)、[官方 SHA-256](https://desktop.docker.com/win/main/amd64/244472/checksums.txt)。安装包 SHA-256 为 `3d8adad4f6c86583e0b00ea19a76197c51ccf5c83437c9ed775b739756452507`，Windows 签名验证为 Valid，发布者 Docker Inc。

【修改代码】：未修改 CoupleLink 业务代码来掩盖 Docker 故障，未修改 Docker 私有程序或关闭系统安全防护。Docker WSL 已停止时，备份了 `docker_data.vhdx`、`ext4.vhdx` 和设置到项目忽略目录 `.secrets/docker-before-update-20261027`；两份磁盘备份与原件 SHA-256 一致。备份包含私人数据，不随发布包分发。

【回归校验】：第一次官方覆盖安装返回 -5，日志记载 Windows 管理员授权被取消。用户随后明确授权重新发起并确认系统提示，第二次覆盖安装退出码 0，安装日志显示 Installation succeeded。

## 2026-10-27 15:46（香港时间）修复结果

- 已安装 Docker Desktop `4.96.3.244472`，引擎 `29.9.2`，Compose `v5.6.1`。
- 覆盖安装完成、首次启动之前，重新计算原 `docker_data.vhdx`、`ext4.vhdx` 和设置文件的 SHA-256，全部与更新前私人备份一致。
- 第一次启动成功，Desktop 状态 running，引擎可响应请求。
- 确认自动恢复的是已有 CoupleLink 数据库后，执行正常停止（退出码 0）及再次启动（退出码 0），原 `dockerInference` 启动失败没有复现。
- 再次启动前后的容器、镜像和卷清单逐项相同：各 1 个；数据库为 running/healthy，`pg_isready` 返回 accepting connections。
- 未恢复出厂设置、未删除容器/镜像/数据卷、未清空数据库、未修改业务代码或禁用系统安全防护。私人磁盘备份和先前保留的运行目录继续保留。

结论：本次官方覆盖更新后，引擎启动及一次正常停止/启动回归均通过。该结果不代表已证明 Windows 内核、驱动或文件系统层的具体原因，也不替代长期稳定性观察。软件 1.0-r2 原先使用隔离本机 PostgreSQL 的测试记录仍按当时环境保留；原安装包及发布压缩包不因此重新生成。
