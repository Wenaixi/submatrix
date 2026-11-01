
## Gist 保存方法

### 部署

- 创建一个 Gist
- 将 Gist ID 配置到 `config.yaml`
- 将 Gist Token 配置到 `config.yaml`

### Worker 反代 GitHub API（可选）

- 将 [worker](https://github.com/sinspired/subs-check-pro/blob/main/doc/cloudflare/worker.js) 部署到 Cloudflare Workers
- 在“变量和机密”设置：
  - `GITHUB_USER` 为你的 GitHub 用户名
  - `GITHUB_ID` 为你的 Gist ID
  - `AUTH_TOKEN` 为访问密钥
- 将 `github-api-mirror` 配置为你的 Worker 地址

示例：

```
github-api-mirror: "https://your-worker-url/github"
```

### 获取订阅

> 如果配置了 Worker，将 `key` 修改为对应的即可。
> 订阅格式：`https://your-worker-url/gist?key=all.yaml&token=AUTH_TOKEN`

- yaml 格式

```
https://gist.githubusercontent.com/YOUR_GITHUB_USERNAME/YOUR_GIST_ID/raw/all.yaml
```

- base64 编码

```
https://gist.githubusercontent.com/YOUR_GITHUB_USERNAME/YOUR_GIST_ID/raw/base64.txt
```

- 带规则的 mihomo.yaml

```
https://gist.githubusercontent.com/YOUR_GITHUB_USERNAME/YOUR_GIST_ID/raw/mihomo.yaml
```
