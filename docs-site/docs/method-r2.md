
## R2 保存方法

### 部署

- 创建 R2 存储桶
- 将 [worker](https://github.com/sinspired/subs-check-pro/blob/main/doc/cloudflare/worker.js) 部署到 Cloudflare Workers
- 在“变量和机密”设置 `AUTH_TOKEN` 为访问密钥
- 在“绑定”中将 R2 存储桶绑定到 Worker，变量名称设为 `SUB_BUCKET`

### 修改配置文件

- 将 `save-method` 配置为 `r2`
- 将 `worker-url` 设置为你的 Worker 地址
- 将 `worker-token` 设置为你的 Worker token

### 获取订阅

- yaml 格式

```
https://your-worker-url/storage?filename=all.yaml&token=AUTH_TOKEN
```

- base64 编码

```
https://your-worker-url/storage?filename=base64.txt&token=AUTH_TOKEN
```

- 带规则的 mihomo.yaml

```
https://your-worker-url/storage?filename=mihomo.yaml&token=AUTH_TOKEN
```
