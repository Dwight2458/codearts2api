# CodeArts2API

> 华为云 CodeArts Agent（盘古助手/码道）的 OpenAI 兼容代理。**无需运行 CodeArts Agent
> 客户端**，纯 Go 直连华为云 API，多账号轮转 + token 自动续期 + WebUI。

## 与 workbuddy2api / traework2api / catpaw2api 同形式

| 能力 | traework2api | catpaw2api | codearts2api |
| --- | --- | --- | --- |
| 上游 | TRAE SOLO 云端通道 | CatPaw 云端 HTTP | 华为云 snap-access 盘古引擎 |
| 凭证 | auths/trae-*.json | auths/catpaw-*.json | auths/codearts-*.json |
| 登录 | login.sh 回调链接 | login.sh 浏览器登录 | login.sh OAuth2 PKCE（回调 + ticket 轮询） |
| 签到/续期 | 每日自动签到 | 自动领注册奖励 + 申请额度 | token 自动 refresh 续期 + 主动保活 |
| 接口 | /v1/chat/completions /v1/models | 同左 | 同左 |
| 依赖 | Go（零三方） | Go（零三方） | Go（零三方） |

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

浏览器打开 **http://127.0.0.1:7866/** 即 WebUI：账号/token 状态、续期调度配置、
对话测试（流式/非流式）。多轮上下文按账号自动续接（chat_id 分组）；也可用请求头
`X-Codearts-Chat-Id: <chatId>` 或 body 里 `conversation_id` 显式指定会话。

## Token 续期与保活（对应自动签到）

CodeArts Agent 没有每日签到，也没有「申请额度」按钮——免费额度按月重置
（[个人用量](https://codearts.huaweicloud.com/portal/settings/personal-usage)）。

本项目的**增强版自动续期调度器**：
- **主动保活**：定期发送轻量级请求保持会话活跃，防止因长时间闲置导致失效
- **积极刷新策略**：token 剩余少于 1 小时即主动刷新，而非被动等待
- **并发控制**：单账号最大并发数限制（默认 5），避免触发上游并发会话上限错误

```json
{ 
  "watch": { 
    "enabled": true, 
    "poll_minutes": 30, 
    "refresh_skew_minutes": 30,
    "keepalive_interval_minutes": 15   // 保活心跳间隔（新增）
  },
  "max_concurrent": 5,                  // 单账号最大并发数（新增）
  "keepalive_window": "10m"             // 保活窗口，超过此时间无活动则发送心跳（新增）
}
```

手动工具：

```bash
./bin/codearts2api-credit               # 账号登录态日报（token 剩余/过期时间/并发数）
./bin/codearts2api-credit -json
./bin/codearts2api-apply                # 批量刷新临期/过期 token
./bin/codearts2api-apply -force         # 强制刷新所有 token
```

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

