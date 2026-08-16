# CodeArts2API

> 华为云 CodeArts Agent（盘古助手/码道）的 OpenAI 兼容代理。**无需运行 CodeArts Agent
> 客户端**，纯 Go 直连华为云 API，多账号轮转 + token 自动续期 + WebUI。

## 与 workbuddy2api / traework2api 同形式

| 能力 | traework2api | codearts2api |
| --- | --- | --- |
| 上游 | TRAE SOLO 云端通道 | 华为云 snap-access 盘古引擎 |
| 凭证 | auths/trae-*.json | auths/codearts-*.json |
| 登录 | login.sh 回调链接 | login.sh OAuth2 PKCE（回调 + ticket 轮询） |
| 接口 | /v1/chat/completions /v1/models | 同左 |
| 依赖 | Go（零三方） | Go（零三方） |

## 参考项目

本项目是 [Sliverkiss](https://github.com/Sliverkiss) 同系列开源项目的延伸实现，架构与运维形态参考了以下仓库：

- [workbuddy2api](https://github.com/Sliverkiss/workbuddy2api) — WorkBuddy CN OpenAI 兼容反代（账号池 / 轮转 / 签到架构）
- [traework2api](https://github.com/Sliverkiss/traework2api) — TRAE Work OpenAI 兼容反代（零依赖 Go 骨架）
- [qoderwork2api](https://github.com/Sliverkiss/qoderwork2api) — QoderWork CN OpenAI 兼容反代（OAuth 授权流程）

感谢原作者的开源与优秀设计。

## 快速开始（Ubuntu / Linux）

```bash
make linux            # bin/ 下 4 个 Linux 静态二进制
make test
```

### 登录（华为云账号）

```bash
# 本机有浏览器
./login.sh

# 服务器（无浏览器）：打印链接，任意机器浏览器打开，ticket 轮询下发
./login.sh -print-only

# 凭证落盘 auths/codearts-{user_id}.json
```

### 启动

```bash
cp config.example.json config.json
export CA2A_API_KEY=你的随机密钥
./bin/codearts2api -config config.json
```

### 验证 + WebUI

```bash
curl http://127.0.0.1:7866/healthz
curl http://127.0.0.1:7866/v1/models -H "Authorization: Bearer $CA2A_API_KEY"
curl -X POST http://127.0.0.1:7866/v1/chat/completions \
  -H "Authorization: Bearer $CA2A_API_KEY" -H "Content-Type: application/json" \
  -d '{"model":"snap-chat","messages":[{"role":"user","content":"你好"}]}'
```

浏览器打开 **http://127.0.0.1:7866/** 即 WebUI：账号/token 状态、对话测试（流式/非流式）。多轮上下文按账号自动续接（chat_id 分组）；也可用请求头
`X-Codearts-Chat-Id: <chatId>` 或 body 里 `conversation_id` 显式指定会话。

## 并发控制优化

为解决**会话并发限制**问题，本项目实现了：

1. **Per-Account 并发计数器**：跟踪每个账号的活跃请求数
2. **智能降级策略**：超过限流时快速切换到其他账号，不阻塞请求
3. **可配置参数**：通过 `max_concurrent` 调整单账号最大并发数

当遇到上游并发超限错误（`tm.00001041`）时，系统会：
- 将该账号暂时软冷却（默认 60 秒）
- 立即尝试下一个健康账号
- 记录日志便于排查

## 逆向依据（详见 docs/reverse-engineering.md）

- 聊天：`POST https://snap-access.cn-north-4.myhuaweicloud.com/v1/chat/chat`
  **AK/SK `SDK-HMAC-SHA256` 签名**（不是 x-auth-token），SSE 逐行 `data:` JSON
- 登录：`https://codearts.huaweicloud.com/authorize`（OAuth2 PKCE）→ 本地回调 →
  `sts.cn-north-4.myhuaweicloud.com/v1/oauth2/tokens`（DPoP 签名）换 STS 临时
  AK/SK + security_token + refresh_token；兜底通道
  `snap-manager/v1/login/ticket?ticket_id=&secret=` 轮询
- 刷新：`POST sts.cn-north-4.myhuaweicloud.com/v1/oauth2/tokens` `grant_type=refresh_token`
- messages 格式：`[{type:"text",text}]`（无 role）；chat_id 为 32 位 hex

## 部署（systemd / Docker）

```bash
sudo mkdir -p /opt/codearts2api && sudo cp -r bin config.example.json auths /opt/codearts2api/
sudo cp deploy/codearts2api.service /etc/systemd/system/
# 编辑 /opt/codearts2api/.env 写 CA2A_API_KEY，改好 config.json
sudo systemctl daemon-reload && sudo systemctl enable --now codearts2api

# 或 Docker
export CA2A_API_KEY=你的随机密钥
mkdir -p auths data
docker compose up -d --build
```

## 环境变量配置

除了 `config.json`，还支持以下环境变量覆盖：

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `CA2A_API_KEY` | API 访问密钥 | - |
| `CA2A_LISTEN` | 监听地址 | `:7866` |
| `CA2A_AUTH_DIR` | 凭证目录 | `./auths` |
| `CA2A_STATE_FILE` | 状态文件 | `./data/state.json` |
| `CA2A_DEFAULT_MODEL` | 默认模型 | `glm-5.2` |
| `CA2A_OAUTH_CALLBACK_HOST` | OAuth 回调主机 | - |
| `CA2A_WATCH_ENABLED` | 调度器开关 | `true` |
| `CA2A_WATCH_POLL_MINUTES` | 轮询间隔（分钟） | `30` |
| `CA2A_WATCH_REFRESH_SKEW` | 提前刷新时间（分钟） | `30` |
| `CA2A_WATCH_KEEPALIVE_INTERVAL` | 保活间隔（分钟） | `15` |
| `CA2A_MAX_CONCURRENT` | 单账号最大并发 | `5` |
| `CA2A_KEEPALIVE_WINDOW` | 保活窗口 | `10m` |

## 目录结构

```
cmd/server/        HTTP 服务（config + main）
cmd/login/         华为云 OAuth2 PKCE 登录
cmd/credit/        账号登录态日报（新增并发信息）
cmd/apply/         批量 token 续期（使用 pool 包）
internal/auth/     auth 文件读写
internal/upstream/ 云端客户端（登录/聊天/SSE）+ 逆向常量
internal/pool/     账号池（token 校验/自动刷新/冷却/并发控制）
internal/scheduler/ token 续期看门狗（新增保活机制）
internal/server/   OpenAI 兼容路由
internal/webui/    内嵌 WebUI 控制台
deploy/            systemd unit 样例
docs/              逆向过程与接口清单
```

## 免责声明

仅供学习和研究使用。使用者需遵守华为云服务条款，自行承担使用风险。

## License

MIT

