package service

import (
	"bufio"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"starcrystal/server/internal/logger"
)

var smtpEnvOnce sync.Once
var smtpMockCfgOnce sync.Once
var smtpMockCfgEnabled bool

type verifyEmailPurpose string

const (
	verifyEmailPurposeRegister verifyEmailPurpose = "register"
	verifyEmailPurposeReset    verifyEmailPurpose = "reset"
)

func ensureSMTPEnvLoaded(traceID string) {
	smtpEnvOnce.Do(func() {
		p := resolveSMTPEnvFilePath()
		if p == "" {
			logger.DebugTrace(traceID, logger.TopicAuth, "smtp env file not found, fallback to process env")
			return
		}
		n, err := loadEnvFileIntoProcess(p)
		if err != nil {
			logger.Error(logger.TopicAuth, "smtp env load failed: path=%s err=%v", p, err)
			return
		}
		logger.Info(logger.TopicAuth, "smtp env loaded: path=%s vars=%d", p, n)
	})
}

func resolveSMTPEnvFilePath() string {
	if custom := strings.TrimSpace(os.Getenv("AUTH_SMTP_ENV_FILE")); custom != "" {
		if isFile(custom) {
			return custom
		}
	}

	candidates := []string{
		filepath.Join("configs", "smtp.local.env"),
		filepath.Join("release", "configs", "smtp.local.env"),
	}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(exeDir, "configs", "smtp.local.env"),
			filepath.Join(exeDir, "..", "configs", "smtp.local.env"),
		)
	}
	for _, p := range candidates {
		if isFile(p) {
			return p
		}
	}
	return ""
}

func isFile(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func loadEnvFileIntoProcess(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	count := 0
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		if key == "" {
			continue
		}
		val := strings.TrimSpace(line[eq+1:])
		if len(val) >= 2 && strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"") {
			val = strings.TrimSpace(val[1 : len(val)-1])
		}
		_ = os.Setenv(key, val)
		count++
	}
	if err := sc.Err(); err != nil {
		return count, err
	}
	return count, nil
}

func maskEmail(email string) string {
	e := strings.TrimSpace(strings.ToLower(email))
	at := strings.IndexByte(e, '@')
	if at <= 1 || at >= len(e)-1 {
		return e
	}
	name := e[:at]
	domain := e[at+1:]
	if len(name) <= 2 {
		return name[:1] + "*" + "@" + domain
	}
	return name[:1] + strings.Repeat("*", len(name)-2) + name[len(name)-1:] + "@" + domain
}

// deliverRegisterVerifyEmail 走正式发信：必须配置完整 SMTP 并成功投递；无配置且未开启 mock 时返回错误，API 不假装已发送。
// 本机/测试可设 AUTH_SMS_MOCK=1 或 AUTH_EMAIL_MOCK=1 跳过真实发信，仅打日志并允许响应里带 devVerifyCode。
func deliverRegisterVerifyEmail(to, code string) error {
	return deliverRegisterVerifyEmailWithTrace(to, code, "")
}

func deliverRegisterVerifyEmailWithTrace(to, code, traceID string) error {
	return sendVerificationEmailUnifiedEntryWithTrace(to, code, verifyEmailPurposeRegister, traceID)
}

// sendVerificationEmailUnifiedEntryWithTrace is the single email sending entrance.
// AUTH_SMS_MOCK=1: skip real SMTP and simulate success in backend.
func sendVerificationEmailUnifiedEntryWithTrace(to, code string, purpose verifyEmailPurpose, traceID string) error {
	ensureSMTPEnvLoaded(traceID)
	start := time.Now()
	logger.DebugTrace(traceID, logger.TopicAuth, "send verification email start purpose=%s to=%s codeLen=%d", purpose, maskEmail(to), len(strings.TrimSpace(code)))
	if isUnifiedEmailMockMode() {
		logger.Info(logger.TopicAuth, "email mock (%s): verify code for %s: %s", purpose, to, code)
		logger.DebugTrace(traceID, logger.TopicAuth, "send verification email done (mock by AUTH_SMS_MOCK) purpose=%s to=%s cost=%s", purpose, maskEmail(to), time.Since(start))
		return nil
	}
	if strings.TrimSpace(os.Getenv("AUTH_SMTP_ADDR")) == "" {
		return fmt.Errorf("email SMTP 未配置：请设置 AUTH_SMTP_ADDR、AUTH_SMTP_USER、AUTH_SMTP_PASS、AUTH_MAIL_FROM；本机可设 AUTH_SMS_MOCK=1 模拟")
	}

	user := strings.TrimSpace(os.Getenv("AUTH_SMTP_USER"))
	pass := strings.TrimSpace(os.Getenv("AUTH_SMTP_PASS"))
	fromDisplay := strings.TrimSpace(os.Getenv("AUTH_MAIL_FROM"))
	if fromDisplay == "" {
		fromDisplay = user
	}
	if user == "" || pass == "" {
		return fmt.Errorf("incomplete SMTP auth: set AUTH_SMTP_USER and AUTH_SMTP_PASS")
	}
	envelope, err := resolveEnvelopeFrom(fromDisplay, user)
	if err != nil {
		return err
	}

	msg := buildVerificationEmailMessage(envelope, to, fromDisplay, code, purpose)
	addr := strings.TrimSpace(os.Getenv("AUTH_SMTP_ADDR"))
	logger.DebugTrace(traceID, logger.TopicAuth, "send verification email smtp prepare purpose=%s to=%s smtp=%s from=%s userSet=%t passSet=%t mode=%s", purpose, maskEmail(to), addr, envelope, user != "", pass != "", strings.TrimSpace(os.Getenv("AUTH_SMTP_MODE")))
	err = sendSMTPAllWithTrace(addr, user, pass, envelope, []string{to}, []byte(msg), traceID)
	if err != nil {
		logger.DebugTrace(traceID, logger.TopicAuth, "send verification email failed purpose=%s to=%s cost=%s err=%v", purpose, maskEmail(to), time.Since(start), err)
		return err
	}
	logger.DebugTrace(traceID, logger.TopicAuth, "send verification email ok purpose=%s to=%s cost=%s", purpose, maskEmail(to), time.Since(start))
	return nil
}

func isUnifiedEmailMockMode() bool {
	return isAuthSmsMockEnabledFromSMTPFile()
}

func isAuthSmsMockEnabledFromSMTPFile() bool {
	smtpMockCfgOnce.Do(func() {
		p := resolveSMTPEnvFilePath()
		if p == "" {
			smtpMockCfgEnabled = false
			return
		}
		v, ok := readEnvValueFromFile(p, "AUTH_SMS_MOCK")
		smtpMockCfgEnabled = ok && strings.TrimSpace(v) == "1"
	})
	return smtpMockCfgEnabled
}

func readEnvValueFromFile(path, key string) (string, bool) {
	f, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			continue
		}
		k := strings.TrimSpace(line[:eq])
		if k != key {
			continue
		}
		val := strings.TrimSpace(line[eq+1:])
		if len(val) >= 2 && strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"") {
			val = strings.TrimSpace(val[1 : len(val)-1])
		}
		return val, true
	}
	return "", false
}

func resolveEnvelopeFrom(fromHeader, smtpUser string) (string, error) {
	if a, err := mail.ParseAddress(fromHeader); err == nil {
		return a.Address, nil
	}
	// 裸邮箱
	if strings.Contains(fromHeader, "@") {
		return strings.TrimSpace(fromHeader), nil
	}
	if a, err := mail.ParseAddress(smtpUser); err == nil {
		return a.Address, nil
	}
	if strings.Contains(smtpUser, "@") {
		return strings.TrimSpace(smtpUser), nil
	}
	return "", fmt.Errorf("invalid AUTH_MAIL_FROM / user (need a valid email): %q", fromHeader)
}

func buildVerificationEmailMessage(envelopeFrom, to, fromDisplay, code string, purpose verifyEmailPurpose) string {
	subj := "Your StarCrystal verification code / StarCrystal 验证码"
	body := "Your StarCrystal verification code is:\r\n\r\n" + code + "\r\n\r\n" +
		"该验证码 10 分钟内有效 / This code expires in 10 minutes.\r\n" +
		"如非本人操作请忽略。If you did not request this, ignore this message.\r\n"
	if purpose == verifyEmailPurposeReset {
		subj = "Password reset / StarCrystal 重置密码"
		body = "Your password reset code is:\r\n\r\n" + code + "\r\n\r\n" +
			"此验证码 10 分钟内有效 / This code expires in 10 minutes.\r\n" +
			"如非本人操作请立即修改密码。If you did not request this, secure your account.\r\n"
	}
	b64 := base64.StdEncoding.EncodeToString([]byte(subj))
	return fmt.Sprintf(
		"From: %s\r\n"+
			"To: %s\r\n"+
			"Subject: =?UTF-8?B?%s?=\r\n"+
			"MIME-Version: 1.0\r\n"+
			"Content-Type: text/plain; charset=UTF-8\r\n"+
			"Content-Transfer-Encoding: 8bit\r\n"+
			"\r\n%s",
		fromDisplay, to, b64, body,
	)
}

// sendSMTPAll 按端口/环境选择 465 SMTPS 或 587/25 等 STARTTLS（Go 的 smtp.SendMail）。
func sendSMTPAll(addr, user, pass, envelope string, to []string, msg []byte) error {
	return sendSMTPAllWithTrace(addr, user, pass, envelope, to, msg, "")
}

func sendSMTPAllWithTrace(addr, user, pass, envelope string, to []string, msg []byte, traceID string) error {
	start := time.Now()
	full, host, port, err := normalizeSMTPAddr(addr)
	if err != nil {
		return err
	}
	logger.DebugTrace(traceID, logger.TopicAuth, "sendSMTPAll start host=%s port=%s recipients=%d", host, port, len(to))
	dial := net.Dialer{Timeout: 45 * time.Second}
	portN, _ := strconv.Atoi(port)
	if os.Getenv("AUTH_SMTP_MODE") == "smtps" {
		logger.DebugTrace(traceID, logger.TopicAuth, "sendSMTPAll choose SMTPS by AUTH_SMTP_MODE=smtps")
		err = sendSMTPSWithTrace(&dial, full, host, user, pass, envelope, to, msg, traceID)
		if err != nil {
			logger.DebugTrace(traceID, logger.TopicAuth, "sendSMTPAll SMTPS failed cost=%s err=%v", time.Since(start), err)
		}
		return err
	}
	// 465 常用隐式 TLS
	if portN == 465 {
		logger.DebugTrace(traceID, logger.TopicAuth, "sendSMTPAll choose SMTPS by port=465")
		err = sendSMTPSWithTrace(&dial, full, host, user, pass, envelope, to, msg, traceID)
		if err != nil {
			logger.DebugTrace(traceID, logger.TopicAuth, "sendSMTPAll SMTPS failed cost=%s err=%v", time.Since(start), err)
		}
		return err
	}
	// 587/2525/25: STARTTLS
	auth := smtp.PlainAuth("", user, pass, host)
	logger.DebugTrace(traceID, logger.TopicAuth, "sendSMTPAll choose STARTTLS/SendMail target=%s", full)
	if err := smtp.SendMail(full, auth, envelope, to, msg); err != nil {
		logger.DebugTrace(traceID, logger.TopicAuth, "sendSMTPAll STARTTLS failed cost=%s err=%v", time.Since(start), err)
		return fmt.Errorf("smtp (STARTTLS): %w", err)
	}
	logger.Info(logger.TopicAuth, "email verify sent to %s (smtp %s)", to[0], full)
	logger.DebugTrace(traceID, logger.TopicAuth, "sendSMTPAll done cost=%s", time.Since(start))
	return nil
}

// normalizeSMTPAddr 解析 host:port；若未写端口则默认 587。IPv6 请用 [addr]:port。
func normalizeSMTPAddr(addr string) (full, host, port string, err error) {
	a := strings.TrimSpace(addr)
	h, p, e := net.SplitHostPort(a)
	if e == nil {
		return a, h, p, nil
	}
	// 未带端口（如 smtp.gmail.com）
	if !strings.Contains(a, ":") {
		return a + ":587", a, "587", nil
	}
	return "", "", "", fmt.Errorf("AUTH_SMTP_ADDR %q: %v (use host:port, e.g. smtp.gmail.com:587)", a, e)
}

// sendSMTPS 在 465 等端口上使用 TLS 直连（隐式 SMTPS）。
func sendSMTPS(d *net.Dialer, fullAddr, host, user, pass, envelope string, to []string, msg []byte) error {
	return sendSMTPSWithTrace(d, fullAddr, host, user, pass, envelope, to, msg, "")
}

func sendSMTPSWithTrace(d *net.Dialer, fullAddr, host, user, pass, envelope string, to []string, msg []byte, traceID string) error {
	start := time.Now()
	skip := os.Getenv("AUTH_SMTP_INSECURE_SKIP_VERIFY") == "1"
	tlsConf := &tls.Config{
		ServerName:         host,
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: skip,
	}
	logger.DebugTrace(traceID, logger.TopicAuth, "sendSMTPS dial addr=%s host=%s insecureSkipVerify=%t", fullAddr, host, skip)
	conn, err := tls.DialWithDialer(d, "tcp", fullAddr, tlsConf)
	if err != nil {
		return fmt.Errorf("smtp (TLS dial): %w", err)
	}
	defer conn.Close()
	logger.DebugTrace(traceID, logger.TopicAuth, "sendSMTPS connected")
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer c.Close()
	auth := smtp.PlainAuth("", user, pass, host)
	if ok, _ := c.Extension("AUTH"); ok {
		logger.DebugTrace(traceID, logger.TopicAuth, "sendSMTPS AUTH extension available, auth start")
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("smtp AUTH: %w", err)
		}
		logger.DebugTrace(traceID, logger.TopicAuth, "sendSMTPS auth success")
	}
	if err := c.Mail(envelope); err != nil {
		return fmt.Errorf("MAIL FROM: %w", err)
	}
	logger.DebugTrace(traceID, logger.TopicAuth, "sendSMTPS MAIL FROM accepted")
	for _, rcpt := range to {
		logger.DebugTrace(traceID, logger.TopicAuth, "sendSMTPS RCPT TO %s", maskEmail(rcpt))
		if err := c.Rcpt(rcpt); err != nil {
			return fmt.Errorf("RCPT TO %s: %w", rcpt, err)
		}
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("DATA: %w", err)
	}
	if _, err = w.Write(msg); err != nil {
		return err
	}
	if err = w.Close(); err != nil {
		return err
	}
	_ = c.Quit()
	logger.Info(logger.TopicAuth, "email verify sent to %s (SMTPS %s)", to[0], fullAddr)
	logger.DebugTrace(traceID, logger.TopicAuth, "sendSMTPS done cost=%s", time.Since(start))
	return nil
}

// deliverPasswordResetEmail 发「重置密码」邮件；SMTP / mock 规则与 <see cref="deliverRegisterVerifyEmail"/> 相同。
func deliverPasswordResetEmail(to, code string) error {
	return deliverPasswordResetEmailWithTrace(to, code, "")
}

func deliverPasswordResetEmailWithTrace(to, code, traceID string) error {
	return sendVerificationEmailUnifiedEntryWithTrace(to, code, verifyEmailPurposeReset, traceID)
}
