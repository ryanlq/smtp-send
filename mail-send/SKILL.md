---
name: mail-send
description: Send emails via the smtp-send CLI tool. Use when the user asks to send an email, notify someone, mail a report or file, or trigger an email alert. Triggers on "send email", "mail", "notify", "email this", "send a message to", "send attachment".
---

# Mail Send

Send emails using `smtp-send`. Binary name may also be `mail-send` — both work.

## Usage Patterns

**Plain text**
```bash
smtp-send --to user@example.com --subject "Hello" --body "Hi there"
```

**Pipe command output as body** — use `--body -`
```bash
df -h / | smtp-send --to ops@example.com --subject "Disk report" --body -
```

**HTML**
```bash
smtp-send --to team@example.com --subject "Report" \
  --body "<h1>Q1 Report</h1><p>Revenue up 12%</p>" --html
```

**Attachment** — repeat `--attach` for multiple files
```bash
smtp-send --to user@example.com --subject "Logs" \
  --body "see attached" --attach /var/log/app.log
```

**Multiple recipients + CC**
```bash
smtp-send --to alice@ex.com,bob@ex.com --cc boss@ex.com \
  --subject "Update" --body "Done"
```

**JSON output** — add `--json` for programmatic result parsing
```bash
smtp-send --to user@example.com --subject "Alert" --body "Disk 95%" --json
# OK:   {"success":true,"recipients":["user@example.com"],"timestamp":"..."}
# FAIL: {"success":false,"error":"auth: 535 ...","exit_code":4}
```

**Override SMTP config per command**
```bash
smtp-send --to a@b.com --subject "Hi" --body "ok" \
  --smtp-host smtp.other.com --smtp-port 587 \
  --smtp-user me@other.com --smtp-pass secret
```

## Exit Codes

| Code | Meaning | Fix |
|------|---------|-----|
| 0 | Sent | — |
| 1 | Bad arguments | Check flags |
| 2 | Missing config | `smtp-send init` then edit `~/.config/smtp-send/config.json` |
| 3 | Network / TLS | Check host, port, connectivity |
| 4 | Auth failed | Check user/password in config |
| 5 | Server rejected | Check recipient address or sender permissions |

On non-zero exit, read stderr for the error message. With `--json`, parse `exit_code` and `error` fields from stdout.

## Config Priority

CLI flags > config file > env vars.

Env vars: `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASS`, `SMTP_FROM`.

Config file search: `./smtp-send.json` > `~/.config/smtp-send/config.json` > `~/.smtp-send.json`. Use `--config <path>` to override.
