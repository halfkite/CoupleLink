# 阿里云部署（47.116.38.184:30710）

## 结构

| 组件 | 位置 | 说明 |
|---|---|---|
| 服务端 | `/www/couplelink/bin/couplelink` | Linux amd64 静态二进制，systemd 托管 |
| 服务配置 | `/www/couplelink/couplelink.env` | 环境变量（数据库、端口、目录、公网地址），权限 640 |
| systemd 单元 | `/etc/systemd/system/couplelink.service` | 开机自启，崩溃自动重启 |
| 数据库 | Docker 容器 `couplelink-db`（postgres:16-alpine） | 仅监听 `127.0.0.1:55432`，数据在 `/www/couplelink/pgdata` |
| 上传图片 | `/www/couplelink/media` | `COUPLELINK_DATA_DIR` |
| 客户端下载 | `/www/couplelink/downloads` | 放 `CoupleLink-Android-<版本>.apk`，`/update` 自动取最新 |
| 管理凭据 | `/www/couplelink/admin.txt` | 每次启动打印到 journalctl |
| 备份 | `/www/couplelink/backups` | 每天 03:15 pg_dump，保留 7 天 |

## 常用命令

```bash
systemctl status couplelink
systemctl restart couplelink
journalctl -u couplelink -f
journalctl -u couplelink | grep 管理页密码     # 找回管理密码
docker logs --tail 50 couplelink-db
```

## 发布新版本

```bash
scp -i aliyun.pem artifacts/<版本>/server/couplelink-linux-amd64 root@47.116.38.184:/www/couplelink/bin/couplelink
scp -i aliyun.pem artifacts/<版本>/CoupleLink-Android-<版本>.apk root@47.116.38.184:/www/couplelink/downloads/
ssh -i aliyun.pem root@47.116.38.184 "chown couplelink:couplelink /www/couplelink/bin/couplelink; systemctl restart couplelink"
```

## 备份与恢复

```bash
/usr/local/bin/couplelink-backup.sh
docker exec -i couplelink-db pg_restore -U couplelink -d couplelink --clean --if-exists < 备份.dump
```

## 注意

- 数据库只绑 `127.0.0.1:55432`，不对公网暴露。
- 端口 30710 已在阿里云安全组放通（实测公网可达）。
- 该机器同时运行宝塔面板（80/443/888）与 frps，本服务互不影响。
- 若管理密码需要重置：停服后删除 `/www/couplelink/admin.txt`，重启即生成新密码。