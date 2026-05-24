// mail-send — Minimal SMTP email sender designed for AI agents.
//
// Zero external dependencies. Compiles to a single static binary.
// Supports JSON output, explicit exit codes, stdin pipe, attachments.
package main

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/quotedprintable"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Default config paths, searched in order.
var configPaths = []string{
	"mail-send.json",
	"~/.config/mail-send/config.json",
	"~/.mail-send.json",
}

const version = "0.1.0"

const (
	ExitOK      = 0
	ExitUsage   = 1
	ExitConfig  = 2
	ExitNetwork = 3
	ExitAuth    = 4
	ExitSend    = 5
)

type Result struct {
	Success    bool     `json:"success"`
	Error      string   `json:"error,omitempty"`
	ExitCode   int      `json:"exit_code,omitempty"`
	Recipients []string `json:"recipients,omitempty"`
	Timestamp  string   `json:"timestamp,omitempty"`
}

// SmtpConfig is the on-disk JSON config (SMTP credentials only).
type SmtpConfig struct {
	Host string `json:"host"`
	Port string `json:"port,omitempty"`
	User string `json:"user"`
	Pass string `json:"pass"`
	From string `json:"from,omitempty"`
}

type Config struct {
	Host     string
	Port     string
	User     string
	Pass     string
	From     string
	To       []string
	CC       []string
	Subject  string
	Body     string
	HTML     bool
	JSON     bool
	Attach   []string
	ConfPath string // --config override
}

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		printHelp()
		os.Exit(0)
	}
	if args[0] == "--help" || args[0] == "-h" {
		printHelp()
		os.Exit(0)
	}
	if args[0] == "--version" || args[0] == "-v" {
		fmt.Println("mail-send " + version)
		os.Exit(0)
	}

	// Sub-command: init — generate a template config file.
	if args[0] == "init" {
		initConfig()
		return
	}

	c := parseArgs(args)

	// Priority: CLI flags > config file > env vars
	// CLI flags are already set by parseArgs. Fill gaps from config file, then env.
	if c.Host == "" || c.User == "" || c.Pass == "" {
		sc, path, err := loadSmtpConfig(c.ConfPath)
		if err != nil {
			fail(c.JSON, "config: "+err.Error(), ExitConfig)
		}
		if sc != nil {
			if c.Host == "" {
				c.Host = sc.Host
			}
			if c.Port == "" {
				c.Port = sc.Port
			}
			if c.User == "" {
				c.User = sc.User
			}
			if c.Pass == "" {
				c.Pass = sc.Pass
			}
			if c.From == "" {
				c.From = sc.From
			}
			_ = path
		}
	}

	// Env fallbacks (lowest priority)
	if c.Host == "" {
		c.Host = os.Getenv("SMTP_HOST")
	}
	if c.Port == "" {
		if p := os.Getenv("SMTP_PORT"); p != "" {
			c.Port = p
		} else {
			c.Port = "587"
		}
	}
	if c.User == "" {
		c.User = os.Getenv("SMTP_USER")
	}
	if c.Pass == "" {
		c.Pass = os.Getenv("SMTP_PASS")
	}
	if c.From == "" {
		if f := os.Getenv("SMTP_FROM"); f != "" {
			c.From = f
		} else {
			c.From = c.User
		}
	}

	if err := validate(c); err != nil {
		fail(c.JSON, err.Error(), ExitUsage)
	}

	if c.Body == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			fail(c.JSON, "read stdin: "+err.Error(), ExitUsage)
		}
		c.Body = string(data)
	}

	if c.Body == "" {
		fail(c.JSON, "body is empty", ExitUsage)
	}

	msg, err := buildMessage(c)
	if err != nil {
		fail(c.JSON, "build message: "+err.Error(), ExitConfig)
	}

	result, code := sendMail(c, msg)
	outputResult(c.JSON, result)
	os.Exit(code)
}

// ---------------------------------------------------------------------------
// Argument parsing
// ---------------------------------------------------------------------------

func parseArgs(args []string) *Config {
	c := &Config{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--from":
			c.From = val(args, &i)
		case "--to":
			c.To = splitList(val(args, &i))
		case "--cc":
			c.CC = splitList(val(args, &i))
		case "--subject":
			c.Subject = val(args, &i)
		case "--body":
			c.Body = val(args, &i)
		case "--html":
			c.HTML = true
		case "--smtp-host":
			c.Host = val(args, &i)
		case "--smtp-port":
			c.Port = val(args, &i)
		case "--smtp-user":
			c.User = val(args, &i)
		case "--smtp-pass":
			c.Pass = val(args, &i)
		case "--attach":
			c.Attach = append(c.Attach, val(args, &i))
		case "--config":
			c.ConfPath = val(args, &i)
		case "--json":
			c.JSON = true
		default:
			fmt.Fprintf(os.Stderr, "unknown argument: %s\n", args[i])
			os.Exit(ExitUsage)
		}
	}
	return c
}

func val(args []string, i *int) string {
	*i++
	if *i >= len(args) {
		fmt.Fprintf(os.Stderr, "missing value for %s\n", args[*i-1])
		os.Exit(ExitUsage)
	}
	return args[*i]
}

func splitList(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Config file loading
// ---------------------------------------------------------------------------

// expandPath handles ~ and $HOME in paths.
func expandPath(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[2:])
	}
	return p
}

func loadSmtpConfig(customPath string) (*SmtpConfig, string, error) {
	paths := configPaths
	if customPath != "" {
		paths = []string{customPath}
	}

	for _, p := range paths {
		p = expandPath(p)
		data, err := os.ReadFile(p)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, p, fmt.Errorf("read %s: %w", p, err)
		}
		var sc SmtpConfig
		if err := json.Unmarshal(data, &sc); err != nil {
			return nil, p, fmt.Errorf("parse %s: %w", p, err)
		}
		return &sc, p, nil
	}

	// No config found is not an error — env vars or flags may provide values.
	return nil, "", nil
}

func initConfig() {
	target := expandPath("~/.config/mail-send/config.json")
	dir := filepath.Dir(target)

	if _, err := os.Stat(target); err == nil {
		fmt.Fprintf(os.Stderr, "Config already exists: %s\n", target)
		os.Exit(ExitConfig)
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir: %v\n", err)
		os.Exit(ExitConfig)
	}

	tmpl := SmtpConfig{
		Host: "smtp.example.com",
		Port: "587",
		User: "your-email@example.com",
		Pass: "your-password-or-app-token",
		From: "your-email@example.com",
	}
	data, _ := json.MarshalIndent(tmpl, "", "  ")
	data = append(data, '\n')

	if err := os.WriteFile(target, data, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "write: %v\n", err)
		os.Exit(ExitConfig)
	}

	fmt.Printf("Created %s\nEdit it with your SMTP credentials.\n", target)
}

func validate(c *Config) error {
	if len(c.To) == 0 {
		return fmt.Errorf("missing --to (recipient emails, comma-separated)")
	}
	if c.Subject == "" {
		return fmt.Errorf("missing --subject")
	}
	if c.Body == "" {
		return fmt.Errorf("missing --body (use '-' for stdin)")
	}
	if c.Host == "" {
		return fmt.Errorf("missing SMTP host (use --smtp-host, config file, or SMTP_HOST)")
	}
	if c.User == "" {
		return fmt.Errorf("missing SMTP user (use --smtp-user, config file, or SMTP_USER)")
	}
	if c.Pass == "" {
		return fmt.Errorf("missing SMTP password (use --smtp-pass, config file, or SMTP_PASS)")
	}
	return nil
}

// ---------------------------------------------------------------------------
// LOGIN auth implementation (not in stdlib, required by Outlook/Yahoo/QQ)
// ---------------------------------------------------------------------------

type smtpLoginAuth struct {
	username, password string
	step               int
}

func loginAuth(username, password string) smtp.Auth {
	return &smtpLoginAuth{username, password, 0}
}

func (a *smtpLoginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", nil, nil
}

func (a *smtpLoginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	switch a.step {
	case 0:
		a.step++
		return []byte(a.username), nil
	case 1:
		a.step++
		return []byte(a.password), nil
	default:
		return nil, fmt.Errorf("unexpected server challenge")
	}
}

// ---------------------------------------------------------------------------
// SMTP sending
// ---------------------------------------------------------------------------

func sendMail(c *Config, msg []byte) (Result, int) {
	addr := c.Host + ":" + c.Port
	var client *smtp.Client
	var err error

	if c.Port == "465" {
		conn, dialErr := tls.Dial("tcp", addr, &tls.Config{ServerName: c.Host})
		if dialErr != nil {
			return errResult("connect: "+dialErr.Error(), ExitNetwork), ExitNetwork
		}
		client, err = smtp.NewClient(conn, c.Host)
	} else {
		client, err = smtp.Dial(addr)
		if err == nil {
			err = client.StartTLS(&tls.Config{ServerName: c.Host})
		}
	}
	if err != nil {
		return classifyNetErr(err), ExitNetwork
	}
	defer client.Close()

	// Auth: LOGIN (Outlook/Yahoo/QQ require this), fall back to PLAIN
	if err := client.Auth(loginAuth(c.User, c.Pass)); err != nil {
		return errResult("auth: "+err.Error(), ExitAuth), ExitAuth
	}

	// MAIL FROM
	if err := client.Mail(c.From); err != nil {
		return errResult("mail from: "+err.Error(), ExitSend), ExitSend
	}

	// RCPT TO (to + cc)
	recipients := append(append([]string{}, c.To...), c.CC...)
	for _, r := range recipients {
		if err := client.Rcpt(r); err != nil {
			return errResult("rcpt to "+r+": "+err.Error(), ExitSend), ExitSend
		}
	}

	// DATA
	w, err := client.Data()
	if err != nil {
		return errResult("data: "+err.Error(), ExitSend), ExitSend
	}
	if _, err := w.Write(msg); err != nil {
		return errResult("write: "+err.Error(), ExitSend), ExitSend
	}
	if err := w.Close(); err != nil {
		return errResult("close data: "+err.Error(), ExitSend), ExitSend
	}

	client.Quit()

	return Result{
		Success:    true,
		Recipients: recipients,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	}, ExitOK
}

// ---------------------------------------------------------------------------
// MIME message builder
// ---------------------------------------------------------------------------

func buildMessage(c *Config) ([]byte, error) {
	var buf bytes.Buffer

	// Headers
	buf.WriteString("From: " + c.From + "\r\n")
	buf.WriteString("To: " + strings.Join(c.To, ", ") + "\r\n")
	if len(c.CC) > 0 {
		buf.WriteString("Cc: " + strings.Join(c.CC, ", ") + "\r\n")
	}
	buf.WriteString("Subject: " + mime.QEncoding.Encode("utf-8", c.Subject) + "\r\n")
	buf.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	buf.WriteString("MIME-Version: 1.0\r\n")

	hasAttach := len(c.Attach) > 0
	hasHTML := c.HTML

	if !hasAttach && !hasHTML {
		// Simple text/plain
		buf.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
		buf.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
		buf.WriteString("\r\n")
		writeQP(&buf, c.Body)
	} else {
		boundary := genBoundary()
		buf.WriteString("Content-Type: multipart/mixed; boundary=\"" + boundary + "\"\r\n")
		buf.WriteString("\r\n")

		// Body part
		buf.WriteString("--" + boundary + "\r\n")
		ct := "text/plain"
		if hasHTML {
			ct = "text/html"
		}
		buf.WriteString("Content-Type: " + ct + "; charset=utf-8\r\n")
		buf.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
		buf.WriteString("\r\n")
		writeQP(&buf, c.Body)
		buf.WriteString("\r\n")

		// Attachments
		for _, path := range c.Attach {
			data, rerr := os.ReadFile(path)
			if rerr != nil {
				return nil, fmt.Errorf("read %s: %w", path, rerr)
			}
			name := filepath.Base(path)
			buf.WriteString("--" + boundary + "\r\n")
			buf.WriteString("Content-Type: application/octet-stream\r\n")
			buf.WriteString("Content-Transfer-Encoding: base64\r\n")
			buf.WriteString("Content-Disposition: attachment; filename=\"" + mime.QEncoding.Encode("utf-8", name) + "\"\r\n")
			buf.WriteString("\r\n")
			b64 := base64.StdEncoding.EncodeToString(data)
			for i := 0; i < len(b64); i += 76 {
				end := i + 76
				if end > len(b64) {
					end = len(b64)
				}
				buf.WriteString(b64[i:end])
				buf.WriteString("\r\n")
			}
			buf.WriteString("\r\n")
		}
		buf.WriteString("--" + boundary + "--\r\n")
	}

	return buf.Bytes(), nil
}

func writeQP(buf *bytes.Buffer, s string) {
	qw := quotedprintable.NewWriter(buf)
	qw.Write([]byte(s))
	qw.Close()
}

func genBoundary() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("=%x", b)
}

// ---------------------------------------------------------------------------
// Output helpers
// ---------------------------------------------------------------------------

func errResult(msg string, code int) Result {
	return Result{Success: false, Error: msg, ExitCode: code}
}

func classifyNetErr(err error) Result {
	msg := err.Error()
	if strings.Contains(msg, "tls") || strings.Contains(msg, "certificate") {
		return errResult("TLS: "+msg, ExitNetwork)
	}
	return errResult("network: "+msg, ExitNetwork)
}

func outputResult(isJSON bool, r Result) {
	if isJSON {
		data, _ := json.Marshal(r)
		fmt.Println(string(data))
		return
	}
	if r.Success {
		fmt.Fprintf(os.Stderr, "OK sent to %v\n", r.Recipients)
	} else {
		fmt.Fprintf(os.Stderr, "ERROR [%d]: %s\n", r.ExitCode, r.Error)
	}
}

func fail(isJSON bool, msg string, code int) {
	outputResult(isJSON, Result{Success: false, Error: msg, ExitCode: code})
	os.Exit(code)
}

func printHelp() {
	fmt.Print(`mail-send — Minimal SMTP sender for AI agents (v` + version + `)

USAGE
  mail-send --to <emails> --subject <text> --body <text|-> [options]
  mail-send init                              generate config template

REQUIRED
  --to <emails>         Recipients (comma-separated)
  --subject <text>      Email subject
  --body <text>         Body text, or "-" to read from stdin

SMTP CONFIG  (priority: flags > config file > env vars)
  --smtp-host <host>    SMTP server  (env: SMTP_HOST)
  --smtp-port <port>    Port, default 587  (env: SMTP_PORT)
                       587 = STARTTLS, 465 = implicit TLS
  --smtp-user <user>    Username  (env: SMTP_USER)
  --smtp-pass <pass>    Password  (env: SMTP_PASS)
  --from <email>        Sender (default: smtp-user, env: SMTP_FROM)
  --config <file>       Config file path (default: auto-detect)

CONFIG FILE
  JSON format, searched in order:
    1. ./mail-send.json
    2. ~/.config/mail-send/config.json
    3. ~/.mail-send.json

  Generate a template:
    mail-send init

  Example config:
    {
      "host": "smtp.gmail.com",
      "port": "587",
      "user": "you@gmail.com",
      "pass": "app-password",
      "from": "you@gmail.com"
    }

OPTIONS
  --cc <emails>         CC recipients
  --html                Body is HTML
  --attach <file>       Attach file (repeatable)
  --json                JSON structured output

EXIT CODES
  0  success
  1  invalid arguments
  2  missing config
  3  network / TLS error
  4  authentication failed
  5  send rejected by server

EXAMPLES
  # First time: generate config
  mail-send init

  # After editing config, send is simple:
  mail-send --to user@example.com --subject "Hi" --body "Hello"

  # Piped body + JSON output
  echo "Disk 95% full" | mail-send \
    --to ops@example.com --subject "Alert" --body - --json

  # Override config with flag
  mail-send --to a@b.com --subject "Test" --body "ok" \
    --smtp-host smtp.other.com

  # Custom config file
  mail-send --to a@b.com --subject "Hi" --body "Hello" \
    --config /path/to/smtp.json
`)
}
