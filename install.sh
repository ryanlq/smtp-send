#!/usr/bin/env bash
# install.sh — Build smtp-send binary and install + Claude Code skill
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BINDIR="${1:-$HOME/.local/bin}"
SKILLDIR="${2:-$HOME/.claude/skills}"

# --- Build ---
echo "Building smtp-send..."
cd "$SCRIPT_DIR"
if ! command -v go &>/dev/null; then
    echo "ERROR: go not found. Install Go first: https://go.dev/dl/" >&2
    exit 1
fi
go build -ldflags="-s -w" -o smtp-send .

# --- Install binary ---
mkdir -p "$BINDIR"
install -m 755 smtp-send "$BINDIR/smtp-send"
ln -sf "$BINDIR/smtp-send" "$BINDIR/mail-send" 2>/dev/null || true
echo "Installed: $BINDIR/smtp-send (alias: $BINDIR/mail-send)"

# --- Install Claude Code skill ---
if [ -d "$SKILLDIR" ]; then
    mkdir -p "$SKILLDIR/mail-send"
    cp -r "$SCRIPT_DIR/mail-send/"* "$SKILLDIR/mail-send/"
    echo "Installed skill: $SKILLDIR/mail-send/"
else
    echo "NOTE: $SKILLDIR not found. Skip skill install (not a Claude Code environment)."
fi

# --- Done ---
echo ""
echo "Next steps:"
echo "  1. Run: smtp-send init"
echo "  2. Edit ~/.config/smtp-send/config.json with your SMTP credentials"
echo "  3. Send: smtp-send --to user@example.com --subject Hi --body Hello"
