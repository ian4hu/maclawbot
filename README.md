# MAClawBot

**Multi-Agents ClawBot** - 在同一个微信账号上运行多个 AI Agent，并动态切换。

## 功能特性

- **动态 Agent 管理** - 无需重启即可添加新 Agent
- **多 Agent 支持** - 同时运行 Hermes、OpenClaw、Claude 或任何自定义 Agent
- **用户级路由** - 每个用户可设置不同的默认 Agent
- **命令式切换** - 通过简单命令切换 Agent
- **持久化配置** - Agent 配置和用户偏好保存到 JSON 文件

## 快速开始

### 构建

```bash
git clone <repo-url> maclawbot
cd maclawbot
go build -o maclawbot ./cmd/maclawbot
```

### 配置

```bash
cp .env.example .env
# 编辑 .env，填入你的 ILINK_TOKEN
```

`.env` 配置项：

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| `ILINK_BASE_URL` | iLink API 地址 | `https://ilinkai.weixin.qq.com` |
| `ILINK_TOKEN` | iLink 认证 Token | **必填** |
| `STATE_FILE` | 状态文件路径 | `./maclawbot_state.json` |
| `LOG_FILE` | 日志文件路径 | `./maclawbot.log` |
| `LONG_POLL_TIMEOUT` | 长轮询超时(秒) | `35` |

### 运行

```bash
./maclawbot
```

## 命令

### Clawbot 命令

| 命令 | 说明 |
|------|------|
| `/clawbot` | 显示帮助 |
| `/clawbot list` | 列出所有 Agent |
| `/clawbot new <name> [tag]` | 创建新 Agent |
| `/clawbot set <name>` | 切换到指定 Agent（仅影响当前用户） |
| `/clawbot del <name>` | 删除自定义 Agent |
| `/clawbot info [name]` | 查看 Agent 详情 |

### 旧版兼容命令

| 命令 | 说明 |
|------|------|
| `/hermes` | 切换到 Hermes |
| `/openclaw` | 切换到 OpenClaw |
| `/whoami` | 显示当前状态 |

## 使用示例

### 创建新 Agent

```
/clawbot new claude
```

输出：
```
Agent **claude** created on port **20000** (tag: [Claude]).

Configure your gateway to use:
http://127.0.0.1:20000

Run `/clawbot set claude` to switch to this agent.
```

### 创建带自定义 Tag 的 Agent

```
/clawbot new claude [Claude Code]
```

输出：
```
Agent **claude** created on port **20001** (tag: [Claude Code]).
```

### 切换 Agent（仅影响你自己）

```
/clawbot set claude
```

### 查看所有 Agent

```
/clawbot list
```

输出：
```
**Available Agents:**

- **hermes**: port 19998 (default)
- **openclaw**: port 19999
- **claude**: port 20000

**Commands:**
- `/clawbot new <name>` - Create new agent
- `/clawbot set <name>` - Switch to agent
- `/clawbot del <name>` - Remove agent
- `/clawbot list` - List all agents
```

## 架构

```
                     iLink API
                        |
               MAClawBot (轮询持有 token)
                        |
        ┌───────────────┼───────────────┐
        │               │               │
   127.0.0.1      127.0.0.1      127.0.0.1
     :19998         :19999         :20000
     Hermes        OpenClaw        Claude
        │               │               │
        └───────────────┴───────────────┘
                        │
                  各自 Gateway
                  (认为直连 iLink)
```

### 工作原理

1. **MAClawBot** 作为唯一的 iLink Token 持有者，持续轮询 iLink 获取消息
2. 收到消息后，根据用户的设置路由到对应的 Agent 队列
3. 每个 Agent 有独立的本地 HTTP 代理服务器，监听不同端口
4. Agent Gateway 连接对应的代理端口，认为自己在直连 iLink
5. Agent 的回复通过代理服务器转发回 iLink

## Agent 配置

Agent 配置和用户路由保存在 `maclawbot_state.json`：

```json
{
  "agents": {
    "hermes": {
      "name": "hermes",
      "port": 19998,
      "tag": "[Hermes Agent]",
      "enabled": true
    },
    "openclaw": {
      "name": "openclaw",
      "port": 19999,
      "tag": "[OpenClaw]",
      "enabled": true
    },
    "claude": {
      "name": "claude",
      "port": 20000,
      "tag": "[Claude]",
      "enabled": true
    }
  },
  "user_agents": {
    "user_id_1": "claude",
    "user_id_2": "hermes"
  },
  "status_shown": {
    "user_id_1": true,
    "user_id_2": true
  }
}
```

### 字段说明

| 字段 | 说明 |
|------|------|
| `agents` | 所有注册的 Agent |
| `agents[].port` | 该 Agent 的代理端口 |
| `agents[].tag` | 消息前缀标签 |
| `agents[].enabled` | 是否启用 |
| `user_agents` | 用户 -> Agent 的映射（用户级路由） |
| `status_shown` | 用户是否已看过欢迎消息 |

## 项目结构

```
maclawbot/
├── cmd/maclawbot/
│   └── main.go           # 主程序入口
├── internal/
│   ├── config/           # 配置加载
│   ├── ilink/            # iLink API 客户端
│   ├── proxy/            # HTTP 代理服务器
│   │   ├── proxy.go      # 代理处理器和管理器
│   │   └── queue.go      # 消息队列
│   └── router/           # 路由逻辑
│       ├── message.go    # 消息解析和命令处理
│       └── state.go      # 状态管理
├── .env.example          # 环境变量示例
├── install.sh            # 安装脚本
├── fix_hermes_splitting.sh # Hermes 消息分割修复
└── README.md
```

## 系统服务

### 使用 systemd

```bash
# 创建服务文件
sudo nano /etc/systemd/system/maclawbot.service

[Unit]
Description=MAClawBot Multi-Agent Proxy
After=network.target

[Service]
Type=simple
User=<your-user>
WorkingDirectory=/path/to/maclawbot
ExecStart=/path/to/maclawbot/maclawbot
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl enable maclawbot
sudo systemctl start maclawbot
sudo journalctl -u maclawbot -f  # 查看日志
```

### 使用安装脚本

```bash
curl -fsSL https://raw.githubusercontent.com/<user>/maclawbot/main/install.sh | bash
```

## 故障排除

### Agent Gateway 连接被拒绝

1. 检查端口是否被占用：`netstat -tlnp | grep <port>`
2. 检查防火墙设置：`sudo ufw allow <port>`
3. 确认 Gateway 配置的 URL 是 `http://127.0.0.1:<port>`

### 消息未路由到正确的 Agent

1. 检查 `maclawbot_state.json` 中的 `user_agents` 配置
2. 使用 `/clawbot info` 查看当前设置
3. 使用 `/clawbot set <name>` 重新设置

### Hermes 消息被分段发送

运行 `fix_hermes_splitting.sh` 修复：

```bash
bash fix_hermes_splitting.sh
sudo systemctl restart hermes
```

## License

MIT
