package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// 会话采用 HMAC 签名的 Cookie（替代 Flask-Login + Flask-WTF），
// 负载包含用户 ID、CSRF 令牌与闪存消息。

const sessionCookieName = "worktime_session"
const sessionMaxAge = 30 * 24 * 3600 // 30 天

var sessionKey []byte

func initSessionSecret() error {
	path := filepath.Join(getDataDir(), "instance", "secret.key")
	if data, err := os.ReadFile(path); err == nil && len(data) >= 32 {
		sessionKey = data
		return nil
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return err
	}
	if err := os.WriteFile(path, key, 0o600); err != nil {
		return err
	}
	sessionKey = key
	return nil
}

type Flash struct {
	Category string `json:"c"`
	Message  string `json:"m"`
}

type sessionPayload struct {
	UID    int64   `json:"uid"`
	CSRF   string  `json:"csrf"`
	Flashs []Flash `json:"flash,omitempty"`
}

type Session struct {
	data  sessionPayload
	dirty bool
}

// sessionWriter 包装 ResponseWriter：在响应头首次固化（WriteHeader/Write）前注入会话 Cookie
type sessionWriter struct {
	http.ResponseWriter
	s        *Session
	injected bool
}

func (sw *sessionWriter) inject() {
	if !sw.injected {
		sw.injected = true
		if sw.s.dirty {
			sw.s.saveHeaders(sw.Header())
		}
	}
}

func (sw *sessionWriter) WriteHeader(code int) {
	sw.inject()
	sw.ResponseWriter.WriteHeader(code)
}

func (sw *sessionWriter) Write(b []byte) (int, error) {
	sw.inject()
	return sw.ResponseWriter.Write(b)
}

// sessionMiddleware 加载会话并在处理器修改后回写 Cookie
func sessionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s := loadSession(r)
		sw := &sessionWriter{ResponseWriter: w, s: s}
		next.ServeHTTP(sw, r.WithContext(withSession(r.Context(), s)))
		sw.inject() // 兜底：handler 未产生任何输出时
	})
}

func loadSession(r *http.Request) *Session {
	s := &Session{}
	c, err := r.Cookie(sessionCookieName)
	if err != nil || c.Value == "" {
		return s
	}
	parts := strings.SplitN(c.Value, ".", 2)
	if len(parts) != 2 {
		return s
	}
	raw, err := base64.URLEncoding.DecodeString(parts[0])
	if err != nil {
		return s
	}
	sig, err := hex.DecodeString(parts[1])
	if err != nil {
		return s
	}
	mac := hmac.New(sha256.New, sessionKey)
	mac.Write(raw)
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return s
	}
	var payload sessionPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return s
	}
	s.data = payload
	return s
}

func (s *Session) saveHeaders(h http.Header) {
	raw, err := json.Marshal(s.data)
	if err != nil {
		return
	}
	mac := hmac.New(sha256.New, sessionKey)
	mac.Write(raw)
	val := base64.URLEncoding.EncodeToString(raw) + "." + hex.EncodeToString(mac.Sum(nil))
	h.AddSetCookie(&http.Cookie{
		Name:     sessionCookieName,
		Value:    val,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   sessionMaxAge,
	})
}

func (s *Session) UserID() int64 { return s.data.UID }

func (s *Session) IsAuthed() bool { return s.data.UID > 0 }

func (s *Session) Login(userID int64) {
	s.data.UID = userID
	s.dirty = true
}

func (s *Session) Logout() {
	s.data.UID = 0
	s.dirty = true
}

// CSRFToken 返回当前会话的 CSRF 令牌，没有则生成
func (s *Session) CSRFToken() string {
	if s.data.CSRF == "" {
		b := make([]byte, 16)
		rand.Read(b)
		s.data.CSRF = hex.EncodeToString(b)
		s.dirty = true
	}
	return s.data.CSRF
}

// CheckCSRF 校验 POST 请求的 CSRF 令牌（表单字段 _csrf 或请求头 X-CSRFToken）
func (s *Session) CheckCSRF(r *http.Request) bool {
	token := r.Header.Get("X-CSRFToken")
	if token == "" {
		token = r.FormValue("_csrf")
	}
	if token == "" || s.data.CSRF == "" {
		// 首次访问时先确保会话持有令牌
		s.CSRFToken()
		return false
	}
	return hmac.Equal([]byte(token), []byte(s.data.CSRF))
}

func (s *Session) Flash(category, message string) {
	s.data.Flashs = append(s.data.Flashs, Flash{category, message})
	s.dirty = true
}

// PopFlashes 取出并清空闪存消息
func (s *Session) PopFlashes() []Flash {
	f := s.data.Flashs
	if len(f) > 0 {
		s.data.Flashs = nil
		s.dirty = true
	}
	return f
}
