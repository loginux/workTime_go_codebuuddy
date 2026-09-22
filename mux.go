package main

import "net/http"

func buildMux() http.Handler {
	mux := http.NewServeMux()

	// 静态资源
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(staticFS)))

	// 认证
	mux.HandleFunc("GET /{$}", handleIndex)
	mux.HandleFunc("/user_select", handleUserSelect)
	mux.HandleFunc("GET /login/{id}", handleLoginAs)
	mux.HandleFunc("GET /logout", handleLogout)

	// 项目
	mux.HandleFunc("GET /projects", authMiddleware(handleProjectsList))
	mux.HandleFunc("GET /projects/{$}", authMiddleware(handleProjectsList))
	mux.HandleFunc("/projects/create", authMiddleware(handleProjectCreate))
	mux.HandleFunc("/projects/{id}/edit", authMiddleware(handleProjectEdit))
	mux.HandleFunc("POST /projects/{id}/delete", authMiddleware(handleProjectDelete))

	// 任务
	mux.HandleFunc("GET /tasks", authMiddleware(handleTasksList))
	mux.HandleFunc("GET /tasks/{$}", authMiddleware(handleTasksList))
	mux.HandleFunc("/tasks/create", authMiddleware(handleTaskCreate))
	mux.HandleFunc("/tasks/{id}/edit", authMiddleware(handleTaskEdit))
	mux.HandleFunc("/tasks/{id}/delete", authMiddleware(handleTaskDelete))

	// 工时
	mux.HandleFunc("/time_entries/create", authMiddleware(handleEntryCreate))
	mux.HandleFunc("/time_entries/{id}/edit", authMiddleware(handleEntryEdit))
	mux.HandleFunc("POST /time_entries/{id}/delete", authMiddleware(handleEntryDelete))
	mux.HandleFunc("GET /time_entries/tasks_for_project", authMiddleware(handleTasksForProject))

	// 视图
	mux.HandleFunc("GET /views/day", authMiddleware(handleDayView))
	mux.HandleFunc("GET /views/week", authMiddleware(handleWeekView))
	mux.HandleFunc("GET /views/month", authMiddleware(handleMonthView))

	// 导出
	mux.HandleFunc("/export", authMiddleware(handleExport))

	return sessionMiddleware(mux)
}
