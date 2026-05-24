# mail-send

极简 SMTP 邮件发送命令行工具，专为 AI Agent 设计。

- 零外部依赖，单文件 Go 二进制 (~4.5MB)
- JSON 结构化输出，明确退出码，无交互式操作
- 配置文件 / 环境变量 / CLI 参数三种配置方式
- 纯文本、HTML、附件

## 安装

```bash
cd mail
go build -ldflags="-s -w" -o mail-send .
install -m 755 mail-send ~/.local/bin/mail-send
```

或一键安装（含 Claude Code skill）：

```bash
./install.sh
```

## 快速开始

### 1. 生成配置文件

```bash
mail-send init
```

生成 `~/.config/mail-send/config.json`（权限 0600），编辑填入你的 SMTP 凭据：

```json
{
  "host": "smtp.gmail.com",
  "port": "587",
  "user": "you@gmail.com",
  "pass": "your-app-password",
  "from": "you@gmail.com"
}
```

### 2. 发送邮件

```bash
# 纯文本
mail-send --to user@example.com --subject "Hello" --body "Hi there"

# 管道输入
echo "Disk usage 95%" | mail-send --to ops@example.com --subject "Alert" --body -

# HTML
mail-send --to team@example.com --subject "Report" --body "<h1>Q1 Report</h1>" --html

# 附件
mail-send --to user@example.com --subject "Log" --body "see attached" --attach server.log
```

## 配置

### 优先级

CLI 参数 > 配置文件 > 环境变量

### 配置文件搜索路径（按顺序）

| 路径 | 用途 |
|------|------|
| `./mail-send.json` | 项目目录 |
| `~/.config/mail-send/config.json` | 用户全局（推荐） |
| `~/.mail-send.json` | 家目录 |

用 `--config <path>` 指定自定义配置文件。

### 环境变量

```bash
SMTP_HOST    # SMTP 服务器地址
SMTP_PORT    # 端口，默认 587
SMTP_USER    # 用户名
SMTP_PASS    # 密码
SMTP_FROM    # 发件人（默认等同 SMTP_USER）
```

### 常见 SMTP 配置

| 服务商 | host | port | 备注 |
|--------|------|------|------|
| Gmail | smtp.gmail.com | 587 | 需要[应用专用密码](https://support.google.com/accounts/answer/185833) |
| Outlook / Office 365 | smtp.office365.com | 587 | |
| QQ 邮箱 | smtp.qq.com | 587 | 需要[授权码](https://service.mail.qq.com/detail/0/52) |
| 163 邮箱 | smtp.163.com | 465 | 端口 465 = 隐式 TLS |
| 腾讯企业邮 | smtp.exmail.qq.com | 465 | |

## 全部参数

```
必填
  --to <emails>         收件人，逗号分隔
  --subject <text>      主题
  --body <text>         正文，"-" 从 stdin 读取

SMTP 配置
  --smtp-host <host>    服务器地址
  --smtp-port <port>    端口，默认 587（587=STARTTLS, 465=隐式TLS）
  --smtp-user <user>    用户名
  --smtp-pass <pass>    密码
  --from <email>        发件人（默认等同 smtp-user）
  --config <file>       指定配置文件路径

选项
  --cc <emails>         抄送
  --html                以 HTML 发送
  --attach <file>       附件（可重复）
  --json                JSON 结构化输出
```

## 退出码

| 码 | 含义 |
|----|------|
| 0 | 发送成功 |
| 1 | 参数错误 |
| 2 | 配置缺失 |
| 3 | 网络 / TLS 错误 |
| 4 | 认证失败 |
| 5 | 服务器拒收 |

## JSON 输出

`--json` 模式下，结果输出到 stdout，适合 AI Agent 解析：

**成功**

```json
{"success":true,"recipients":["user@example.com"],"timestamp":"2026-05-22T12:00:00Z"}
```

**失败**

```json
{"success":false,"error":"auth: 535 Authentication failed","exit_code":4}
```

## AI Agent 集成示例

```bash
# 监控脚本 — 磁盘告警
df -h / | awk 'NR==2 && $5+0 > 90 {print $5}' | \
  xargs -I{} mail-send --to ops@example.com --subject "Disk alert: {}" \
    --body "Root partition usage at {}" --json

# CI/CD 通知
mail-send --to dev@example.com --subject "Deploy $CI_COMMIT_SHA done" \
  --body "Pipeline $CI_PIPELINE_ID completed" --json
```
