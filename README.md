# MAClawBot

**Multi-Agents ClawBot** - 将微信ClawBot连接到多个 AI Agent，并支持动态切换。

## 功能特性

- **动态 Agent 管理** - 无需重启即可添加新 Agent
- **多 Agent 支持** - 同时运行 Hermes、OpenClaw、Claude 或任何自定义 Agent
- **命令式切换** - 通过简单命令切换 Agent
- **持久化配置** - Agent 配置和用户偏好保存到 JSON 文件
- **协议兼容** - 完全兼容 iLink Bot API 协议
- **媒体支持** - 支持文本、语音、图片、视频、文件等多种消息类型
- **线程安全** - 完整的并发控制和竞态检测

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

>> 可使用`Hermes Agent` 或 `OpenClaw` 现有的 `ILINK_TOKEN` 
>> - HermesAgent -> `~/.hermes/.env` 文件中的 `WEIXIN_TOKEN`
>> - OpenClaw -> `~/.openclaw/openclaw-weixin/accounts/xxxx-im-bot.json` 中的 `token`

- 连接Hermes Agent：修改 `~/.hermes/.env` 设置 `WEIXIN_BASE_URL=http://127.0.0.1:19998/`
- 连接OpenClaw：修改 `~/.openclaw/openclaw-weixin/accounts/xxxx-im-bot.json` 设置 `{ "baseUrl": "http://127.0.0.1:19999/"}`

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
| `/clawbot set <name>` | 切换到指定 Agent |
| `/clawbot del <name>` | 删除自定义 Agent |
| `/clawbot info [name]` | 查看 Agent 详情 |

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

### 切换 Agent

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

Agent 配置保存在 `maclawbot_state.json`：

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
| `status_shown` | 用户是否已看过欢迎消息 |

## 项目结构

```
maclawbot/
├── cmd/maclawbot/
│   └── main.go                 # 主程序入口，消息轮询和路由
├── internal/
│   ├── config/                 # 配置加载（环境变量）
│   │   └── config.go
│   ├── ilink/                  # iLink API 客户端
│   │   └── client.go           # HTTP 客户端，请求头生成
│   ├── proxy/                  # HTTP 代理服务器
│   │   ├── proxy.go            # 代理处理器和管理器
│   │   └── queue.go            # 线程安全消息队列
│   └── router/                 # 消息路由逻辑
│       ├── message.go          # 消息解析和命令处理
│       └── state.go            # 状态管理（线程安全）
├── .env.example                # 环境变量示例
└── README.md                   # 项目文档
```

## 测试

### 运行测试
```bash
# 运行所有测试
go test ./...

# 运行测试并显示详细信息
go test -v ./...

# 使用 Race Detector 检测竞态条件
go test -race ./...

# 查看测试覆盖率
go test -cover ./...
```

## 开发指南

### 构建选项
```bash
# 开发版本（包含调试信息）
go build -o maclawbot ./cmd/maclawbot

# 生产版本（优化编译）
go build -ldflags="-s -w" -o maclawbot ./cmd/maclawbot

# 查看版本信息
go build -ldflags="-X main.Version=2.1.0" -o maclawbot ./cmd/maclawbot
./maclawbot -version
```

## 部署指南

### 使用 systemd (Linux)

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

### 使用 launchd (macOS)

macOS 使用 `launchd` 作为服务管理器，可以通过创建 plist 文件实现自启动。

#### 方法一：手动创建 plist 文件

1. **创建 LaunchAgents 目录**（如果不存在）：
```bash
mkdir -p ~/Library/LaunchAgents
```

2. **创建 plist 文件**：
```bash
cat > ~/Library/LaunchAgents/com.maclawbot.agent.plist << 'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.maclawbot.agent</string>
    
    <key>ProgramArguments</key>
    <array>
        <string>/path/to/maclawbot/maclawbot</string>
    </array>
    
    <key>WorkingDirectory</key>
    <string>/path/to/maclawbot</string>
    
    <key>RunAtLoad</key>
    <true/>
    
    <key>KeepAlive</key>
    <true/>
    
    <key>StandardOutPath</key>
    <string>/path/to/maclawbot/maclawbot.log</string>
    
    <key>StandardErrorPath</key>
    <string>/path/to/maclawbot/maclawbot.log</string>
    
    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key>
        <string>/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin</string>
    </dict>
</dict>
</plist>
EOF
```

3. **替换路径**：
```bash
# 将 /path/to/maclawbot 替换为实际路径
sed -i '' 's|/path/to/maclawbot|'$(pwd)'|g' ~/Library/LaunchAgents/com.maclawbot.agent.plist
```

4. **加载服务**：
```bash
# 加载并启动服务
launchctl load ~/Library/LaunchAgents/com.maclawbot.agent.plist

# 检查服务状态
launchctl list | grep maclawbot

# 查看日志
tail -f ~/Library/LaunchAgents/../maclawbot.log
```

5. **管理服务**：
```bash
# 停止服务
launchctl unload ~/Library/LaunchAgents/com.maclawbot.agent.plist

# 重新启动服务
launchctl unload ~/Library/LaunchAgents/com.maclawbot.agent.plist
launchctl load ~/Library/LaunchAgents/com.maclawbot.agent.plist

# 查看服务详细信息
launchctl list com.maclawbot.agent
```

#### 方法二：使用快捷脚本（推荐）

创建一个快速安装脚本：

```bash
cat > install-macos-service.sh << 'SCRIPT'
#!/bin/bash

# macOS 自启动服务安装脚本
set -e

# 获取当前目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PLIST_FILE="$HOME/Library/LaunchAgents/com.maclawbot.agent.plist"

echo "🔧 安装 MAClawBot macOS 自启动服务..."

# 创建 LaunchAgents 目录
mkdir -p "$HOME/Library/LaunchAgents"

# 生成 plist 文件
cat > "$PLIST_FILE" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.maclawbot.agent</string>
    
    <key>ProgramArguments</key>
    <array>
        <string>${SCRIPT_DIR}/maclawbot</string>
    </array>
    
    <key>WorkingDirectory</key>
    <string>${SCRIPT_DIR}</string>
    
    <key>RunAtLoad</key>
    <true/>
    
    <key>KeepAlive</key>
    <true/>
    
    <key>StandardOutPath</key>
    <string>${SCRIPT_DIR}/maclawbot.log</string>
    
    <key>StandardErrorPath</key>
    <string>${SCRIPT_DIR}/maclawbot.log</string>
    
    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key>
        <string>/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin</string>
    </dict>
</dict>
</plist>
EOF

# 卸载旧服务（如果存在）
if launchctl list | grep -q "com.maclawbot.agent"; then
    echo "🔄 停止旧服务..."
    launchctl unload "$PLIST_FILE" 2>/dev/null || true
fi

# 加载新服务
echo "🚀 启动服务..."
launchctl load "$PLIST_FILE"

echo "✅ MAClawBot 自启动服务安装完成！"
echo "📝 日志文件: ${SCRIPT_DIR}/maclawbot.log"
echo ""
echo "管理命令："
echo "  查看状态: launchctl list | grep maclawbot"
echo "  查看日志: tail -f ${SCRIPT_DIR}/maclawbot.log"
echo "  停止服务: launchctl unload $PLIST_FILE"
echo "  启动服务: launchctl load $PLIST_FILE"
SCRIPT

# 添加执行权限
chmod +x install-macos-service.sh

echo "✅ 安装脚本已创建：install-macos-service.sh"
echo "运行以下命令安装服务："
echo "  ./install-macos-service.sh"
```

## 故障排除

### Agent Gateway 连接被拒绝

1. 检查端口是否被占用：`netstat -tlnp | grep <port>`
2. 检查防火墙设置：`sudo ufw allow <port>`
3. 确认 Gateway 配置的 URL 是 `http://127.0.0.1:<port>`
4. 查看日志确认代理服务器启动：`grep "Starting agent" maclawbot.log`

### 消息未路由到正确的 Agent

1. 检查 `maclawbot_state.json` 中的 `default` 字段
2. 使用 `/clawbot info` 查看当前设置
3. 使用 `/clawbot set <name>` 重新设置
4. 查看日志：`grep "Msg from" maclawbot.log`

### 会话过期 (errcode: -14)

1. 检查日志：`grep "errcode" maclawbot.log`
2. 会话过期后需要重新扫描二维码登录
3. 清除状态文件：`rm maclawbot_state.json`（可选）
4. 重启 maclawbot 并重新登录

## 最佳实践

### 安全建议
1. **保护 Token** - 不要在代码中硬编码 `ILINK_TOKEN`
2. **使用 .env 文件** - 将敏感信息保存在 `.env` 中
3. **限制访问** - 代理服务器只监听 `127.0.0.1`，不暴露到公网
4. **定期更新** - 保持 maclawbot 和依赖包的最新版本
5. **日志轮转** - 配置 logrotate 防止日志文件过大

### 性能优化
1. **合理设置超时** - `LONG_POLL_TIMEOUT=35` 是最佳值
2. **监控资源使用** - 定期检查内存和 CPU 使用率
3. **Agent 数量** - 建议不超过 10 个 Agent
4. **消息队列** - 默认 200 条容量，可根据需要调整
5. **并发控制** - 使用 RWMutex 保证线程安全的同时提高性能

### 运维建议
1. **使用 systemd** - 配置自动重启和日志管理
2. **监控告警** - 设置日志监控，及时发现异常
3. **定期备份** - 备份 `maclawbot_state.json` 配置文件
4. **灰度发布** - 新功能先在测试环境验证
5. **文档更新** - 保持文档与代码同步

## 相关链接

- [iLink Bot API 文档](https://www.wechatbot.dev/en/protocol)
- [WeChat Bot Developer](https://www.wechatbot.dev/)

## License

MIT
