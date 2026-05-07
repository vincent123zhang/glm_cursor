# Cursor GLM Proxy

这是一个面向 Cursor 的 GLM/BigModel 代理服务。它提供 OpenAI 兼容接口，让 Cursor 或其他 OpenAI API 兼容客户端可以通过本地代理调用 GLM 模型。

> 安全提示：仓库不包含任何真实 API Key。请复制 `.env.example` 到 `.env` 后填写自己的密钥，不要提交 `.env`、`.env.glm` 或 `.env.local`。

## 功能

- 兼容 OpenAI `/v1/chat/completions` 和 `/v1/models`
- 默认转发到 BigModel OpenAI 兼容接口
- 默认上游模型为 `glm-5.1`
- 默认对 Cursor 暴露的模型名为 `gpt-4o`
- 支持流式响应、CORS、HTTP/2
- 支持工具调用/function calling 的请求转换
- 支持 Go 本地运行和 Docker 运行

## 环境要求

- Go 1.19 或更高版本
- GLM/BigModel API Key
- Cursor 或其他 OpenAI API 兼容客户端

## 配置

复制示例配置：

```bash
cp .env.example .env
```

编辑 `.env`，只需要填写一行：

```env
GLM_API_KEY=your_glm_api_key_here
```

默认情况下，客户端访问本代理时也使用同一个 Key，即请求头：

```text
Authorization: Bearer your_glm_api_key_here
```

## 本地运行

安装依赖：

```bash
go mod download
```

启动 GLM 代理：

```bash
go run proxy-glm.go
```

默认监听：

```text
http://localhost:9000
```

## Docker 运行

构建 GLM 镜像：

```bash
docker build -t cursor-glm .
```

运行容器：

```bash
docker run -p 9000:9000 --env-file .env cursor-glm
```

## Cursor 配置

代理启动后，在 Cursor 中把 OpenAI API Base URL 设置为：

```text
http://localhost:9000/v1
```

API Key 填写你的 `GLM_API_KEY`。

如果你通过 ngrok、Cloudflare Tunnel 等方式暴露公网地址，则 Base URL 设置为：

```text
https://your-public-domain/v1
```

## API

```text
GET  /v1/models
POST /v1/chat/completions
```

## 可选配置

通常不需要设置这些变量。只有在你想覆盖默认行为时再添加到 `.env`：

```env
GLM_BASE_URL=https://open.bigmodel.cn/api/paas/v4
GLM_MODEL=glm-5.1
GLM_CURSOR_MODEL=gpt-4o
GLM_PROXY_API_KEY=your_proxy_api_key_here
GLM_PROXY_ADDR=:9000
```

其中 `GLM_PROXY_API_KEY` 用于单独设置客户端访问本代理的 Key；不设置时默认复用 `GLM_API_KEY`。

## 安全建议

- 不要提交 `.env`、`.env.glm`、`.env.local`
- 公开部署时建议单独设置 `GLM_PROXY_API_KEY`
- 不要把本服务直接暴露到公网，除非你已经做好鉴权和访问控制
- API Key 泄露后请立即到服务商后台撤销并重新生成

## License

本项目使用 GPLv2 许可证，详见 [LICENSE.md](LICENSE.md)。
