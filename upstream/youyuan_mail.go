package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// This service is an internal, signed, registration-only mail bridge. It is
// deliberately absent from the general MCP/tool catalogue.
type yyMailConfig struct {
	RelaySecret string `json:"relay_secret"`
	Host        string `json:"smtp_host"`
	Port        int    `json:"smtp_port"`
	Username    string `json:"smtp_username"`
	Password    string `json:"smtp_authorization_code"`
}
type yyRegistrationMail struct {
	To      string `json:"to"`
	Code    string `json:"code"`
	Purpose string `json:"purpose"`
}

var yyQQMailbox = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}@qq\.com$`)
var yyMailCode = regexp.MustCompile(`^[0-9]{6}$`)

func yyMailRoutes(mux *http.ServeMux, dataDir string) {
	configPath := filepath.Join(dataDir, "mail-config.json")
	mux.HandleFunc("/api/youyuan/mail/send", func(w http.ResponseWriter, r *http.Request) {
		fail := func(status int, message string) {
			w.WriteHeader(status)
			writeJSON(w, map[string]any{"ok": false, "error": message})
		}
		if r.Method != http.MethodPost {
			fail(405, "仅支持 POST")
			return
		}
		var cfg yyMailConfig
		raw, err := os.ReadFile(configPath)
		if err != nil || json.Unmarshal(raw, &cfg) != nil || len(cfg.RelaySecret) < 32 {
			fail(503, "注册发信服务尚未配置")
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 2048))
		if err != nil {
			fail(413, "请求过大")
			return
		}
		stamp := r.Header.Get("X-YouYuan-Mail-Timestamp")
		millis, err := strconv.ParseInt(stamp, 10, 64)
		signature, decodeErr := hex.DecodeString(r.Header.Get("X-YouYuan-Mail-Signature"))
		mac := hmac.New(sha256.New, []byte(cfg.RelaySecret))
		mac.Write([]byte(stamp + "\n"))
		mac.Write(body)
		if err != nil || decodeErr != nil || millis < time.Now().Add(-5*time.Minute).UnixMilli() || millis > time.Now().Add(time.Minute).UnixMilli() || !hmac.Equal(mac.Sum(nil), signature) {
			fail(403, "仅允许游园账号服务发送注册验证码")
			return
		}
		var letter yyRegistrationMail
		if json.Unmarshal(body, &letter) != nil || !yyQQMailbox.MatchString(letter.To) || !yyMailCode.MatchString(letter.Code) || letter.Purpose != "registration" {
			fail(400, "验证码请求无效")
			return
		}
		if cfg.Host == "" || cfg.Username == "" || cfg.Password == "" {
			fail(503, "请配置发信邮箱及 SMTP 授权码")
			return
		}
		if err = yySendRegistrationMail(cfg, letter); err != nil {
			fail(502, "SMTP 发信未成功，请检查配置或稍后再试")
			return
		}
		writeJSON(w, map[string]any{"ok": true, "accepted": true})
	})
}

func yySendRegistrationMail(cfg yyMailConfig, letter yyRegistrationMail) error {
	sender, err := mail.ParseAddress(cfg.Username)
	if err != nil || sender.Address != cfg.Username {
		return fmt.Errorf("发信地址无效")
	}
	port := cfg.Port
	if port == 0 {
		port = 465
	}
	if port != 465 && port != 587 {
		return fmt.Errorf("仅允许加密 SMTP 端口")
	}
	address := net.JoinHostPort(cfg.Host, strconv.Itoa(port))
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	var conn net.Conn
	tlsConfig := &tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12}
	if port == 465 {
		conn, err = tls.DialWithDialer(dialer, "tcp", address, tlsConfig)
	} else {
		conn, err = dialer.Dial("tcp", address)
	}
	if err != nil {
		return err
	}
	defer conn.Close()
	if err = conn.SetDeadline(time.Now().Add(20 * time.Second)); err != nil {
		return err
	}
	client, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		return err
	}
	defer client.Close()
	if port == 587 {
		if err = client.StartTLS(tlsConfig); err != nil {
			return err
		}
	}
	if err = client.Auth(smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)); err != nil {
		return err
	}
	if err = client.Mail(cfg.Username); err != nil {
		return err
	}
	if err = client.Rcpt(letter.To); err != nil {
		return err
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	message := "From: " + cfg.Username + "\r\nTo: " + letter.To + "\r\nSubject: YouYuan registration verification\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n游园注册验证码：" + letter.Code + "\r\n\r\n验证码 10 分钟内有效，仅用于注册本次填写的账号。请勿将验证码告诉他人。如果不是你申请的，请忽略此邮件。\r\n"
	if _, err = io.Copy(writer, strings.NewReader(message)); err != nil {
		return err
	}
	if err = writer.Close(); err != nil {
		return err
	}
	// DATA success is SMTP acceptance. A later QUIT failure must not report an
	// unsent letter after the provider already accepted it.
	_ = client.Quit()
	return nil
}
