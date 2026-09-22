package main

import (
	"context"
	"net/http"
)

type sessionKeyType struct{}

var sessionKeyCtx sessionKeyType

func withSession(ctx context.Context, s *Session) context.Context {
	return context.WithValue(ctx, sessionKeyCtx, s)
}

func getSession(r *http.Request) *Session {
	if s, ok := r.Context().Value(sessionKeyCtx).(*Session); ok {
		return s
	}
	return &Session{}
}

// authMiddleware 未登录时重定向到用户选择页
func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s := getSession(r)
		if !s.IsAuthed() {
			http.Redirect(w, r, "/user_select", http.StatusFound)
			return
		}
		next(w, r)
	}
}

// requireCSRF 校验 CSRF，失败返回 403 JSON
func requireCSRF(w http.ResponseWriter, r *http.Request) bool {
	s := getSession(r)
	r.ParseForm()
	if !s.CheckCSRF(r) {
		if wantsJSON(r) {
			writeJSON(w, http.StatusForbidden, map[string]any{"error": "CSRF 校验失败，请刷新页面重试"})
		} else {
			s.Flash("error", "CSRF 校验失败，请刷新页面重试")
			back := r.Header.Get("Referer")
			if back == "" {
				back = "/"
			}
			http.Redirect(w, r, back, http.StatusFound)
		}
		return false
	}
	return true
}

func wantsJSON(r *http.Request) bool {
	return r.Header.Get("X-Requested-With") == "XMLHttpRequest" ||
		r.Header.Get("X-CSRFToken") != ""
}
