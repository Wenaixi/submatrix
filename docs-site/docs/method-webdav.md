
# WebDAV 保存方法

> 将检测结果保存到自建或第三方 WebDAV 存储。

## 配置步骤

1) 在 `config.yaml` 设置保存方式：

```yaml
save-method: webdav
```

1) 配置 WebDAV 连接信息：

```yaml
webdav:
  url: "https://webdav.example.com/remote.php/dav/files/USERNAME"
  username: "USERNAME"
  password: "PASSWORD"
  # 可选，保存到服务端的子路径（相对路径）
  path: "/subs-check-pro"
  # 超时（秒，可选）
  timeout: 15
```

- 常见服务：Nextcloud、坚果云、Box、某些 NAS 等。
- 建议为 subs-check-pro 单独创建目录，避免与其他文件混放。

## 生成的文件

保存成功后，将在 WebDAV 目标目录看到：

- all.yaml（Clash/Mihomo 节点）
- mihomo.yaml（带分流规则）
- base64.txt（Base64 订阅）
- history.yaml（历次检测可用节点）

## 订阅访问（示例）

若 WebDAV 服务可通过公开链接分享文件，可使用服务端提供的共享链接进行分发。否则，建议搭配 R2/Gist 或项目内置文件服务发布订阅。

> 提示：WebDAV 是否能直接公开访问取决于服务提供方。
