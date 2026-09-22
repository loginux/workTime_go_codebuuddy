package main

import (
	"net/http"
	"strings"
)

// GET / → 已登录跳日视图，否则跳用户选择
func handleIndex(w http.ResponseWriter, r *http.Request) {
	s := getSession(r)
	if s.IsAuthed() {
		http.Redirect(w, r, "/views/day", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/user_select", http.StatusFound)
}

// GET/POST /user_select
func handleUserSelect(w http.ResponseWriter, r *http.Request) {
	s := getSession(r)
	if r.Method == http.MethodPost {
		if !requireCSRF(w, r) {
			return
		}
		username := strings.TrimSpace(r.FormValue("username"))
		switch {
		case username == "":
			s.Flash("error", "用户名不能为空")
		case getUserByUsername(username) != nil:
			s.Flash("error", "用户名已存在")
		default:
			userID, err := createUser(username)
			if err != nil {
				s.Flash("error", "创建用户失败: "+err.Error())
			} else {
				s.Login(userID)
				s.Flash("success", "用户 "+username+" 创建成功")
				http.Redirect(w, r, "/views/day", http.StatusFound)
				return
			}
		}
		http.Redirect(w, r, "/user_select", http.StatusFound)
		return
	}

	type Page struct {
		Base
		Users []User
	}
	render(w, "auth_user_select.html", Page{Base: baseData(r, ""), Users: getAllUsers()})
}

// GET /login/{id} → 直接以某用户身份登录（无密码）
func handleLoginAs(w http.ResponseWriter, r *http.Request) {
	s := getSession(r)
	id := pathID(r, "id")
	user := getUserByID(id)
	if user == nil {
		s.Flash("error", "用户不存在")
		http.Redirect(w, r, "/user_select", http.StatusFound)
		return
	}
	s.Login(user.ID)
	s.Flash("success", "已切换至用户："+user.Username)
	http.Redirect(w, r, "/views/day", http.StatusFound)
}

// GET /logout
func handleLogout(w http.ResponseWriter, r *http.Request) {
	s := getSession(r)
	s.Logout()
	http.Redirect(w, r, "/user_select", http.StatusFound)
}
