---
name: mail-send
description: Send emails via SMTP from the command line. Use when the user asks to send an email, notify someone, mail a report/file, or trigger an email alert. Triggers on keywords like "send email", "mail", "notify by email", "email this", "send a message to". Requires the smtp-send binary installed and SMTP credentials configured.
---

# Mail Send

Send emails using `smtp-send`, a zero-dependency CLI tool with JSON output and explicit exit codes.

## Prerequisites

Verify the tool is installed and configured:

```bash
which smtp-send || which mail-send
# If missing, build from source:
cd /path/to/mail && go build -ldflags="-s -w" -o smtp-send .

# Check config exists:
cat ~/.config/smtp-send/config.json
# If missing, generate template:
smtp-send init
```

The binary may be symlinked as `mail-send` — both names work.

## Send Email

### Basic usage

```bash
smtp-send --to user@example.com --subject "Hello" --body "Hi there"
```

### Pipe body from command output

```bash
df -h / | smtp-send --to ops@example.com --subject "Disk report" --body -
```

Use `--body -` to read from stdin. Ideal for sending command output, logs, or generated content.

### HTML email

```bash
smtp-send --to team@example.com --subject "Report" --body "<h1>Q1 Report</h1><p>Revenue up 12%</p>" --html
```

### With attachment

```bash
smtp-send --to user@example.com --subject "Logs" --body "see attached" --attach /var/log/app.log
```

Multiple attachments: repeat `--attach`.

### Multiple recipients and CC

```bash
smtp-send --to alice@ex.com,bob@ex.com --cc boss@ex.com --subject "Update" --body "Done"
```

### JSON output (for programmatic use)

Always add `--json` when you need to parse the result:

```bash
smtp-send --to user@example.com --subject "Alert" --body "Disk 95%" --json
# Success: {"success":true,"recipients":["user@example.com"],"timestamp":"..."}
# Failure: {"success":false,"error":"auth: 535 ...","exit_code":4}
```

## Exit Codes

| Code | Meaning | Action |
|------|---------|--------|
| 0 | Sent successfully | Done |
| 1 | Bad arguments | Fix the command flags |
| 2 | Missing config | Run `smtp-send init` and edit credentials |
| 3 | Network/TLS error | Check host, port, connectivity |
| 4 | Auth failed | Check username/password in config |
| 5 | Server rejected | Check recipient address, sender permissions |

## Config Priority

CLI flags > config file > env vars (`SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASS`, `SMTP_FROM`).

Config file search order: `./smtp-send.json` > `~/.config/smtp-send/config.json` > `~/.smtp-send.json`.

## Common SMTP Providers

| Provider | host | port | Note |
|----------|------|------|------|
| Gmail | smtp.gmail.com | 587 | App-specific password required |
| Outlook/365 | smtp.office365.com | 587 | |
| QQ Mail | smtp.qq.com | 587 | Authorization code required |
| 163 Mail | smtp.163.com | 465 | Implicit TLS |
| Tencent ExMail | smtp.exmail.qq.com | 465 | |
