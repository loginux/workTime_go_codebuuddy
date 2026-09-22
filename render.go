package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"sync"
	"time"
)

//go:embed web/templates
var templateFS embed.FS

//go:embed web/static
var staticFS embed.FS

var funcMap = template.FuncMap{
	"fmtMin": formatMinutes,
	"weekdayCN": func(d time.Time) string {
		return string([]rune("日一二三四五六")[int(d.Weekday())])
	},
	"weekdayIndex": func(d time.Time) int {
		return int(d.Weekday())
	},
	"firstChar": func(s string) string {
		if s == "" {
			return "?"
		}
		return strings.ToUpper(string([]rune(s)[0]))
	},
}

// formatMinutes 将分钟数格式化为 X小时X分钟
func formatMinutes(minutes int) string {
	hours := minutes / 60
	mins := minutes % 60
	switch {
	case hours > 0 && mins > 0:
		return fmt.Sprintf("%d小时%d分钟", hours, mins)
	case hours > 0:
		return fmt.Sprintf("%d小时", hours)
	default:
		return fmt.Sprintf("%d分钟", mins)
	}
}

// Base 所有页面共用的基础数据
type Base struct {
	Nav      string // day / week / month / projects / export / ""
	Authed   bool
	Username string
	CSRF     string
	Flashes  []Flash
	Title    string
}

func baseData(r *http.Request, nav string) Base {
	s := getSession(r)
	b := Base{
		Nav:     nav,
		Authed:  s.IsAuthed(),
		CSRF:    s.CSRFToken(),
		Flashes: s.PopFlashes(),
	}
	if u := getUserByID(s.UserID()); u != nil {
		b.Username = u.Username
	}
	return b
}

var (
	templateCacheMu sync.RWMutex
	templateCache   = map[string]*template.Template{}
)

// render 渲染页面（base.html + 指定页面模板，带并发保护的缓存）
func render(w http.ResponseWriter, page string, data any) {
	t := func() *template.Template {
		templateCacheMu.RLock()
		defer templateCacheMu.RUnlock()
		return templateCache[page]
	}()
	if t == nil {
		t = template.Must(
			template.New("base.html").Funcs(funcMap).
				ParseFS(templateFS, "web/templates/base.html", "web/templates/"+page),
		)
		templateCacheMu.Lock()
		templateCache[page] = t
		templateCacheMu.Unlock()
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "base.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
