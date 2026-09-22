package main

import (
	"net/http"
	"strconv"
	"strings"
)

// pathID 从 URL 路径参数中解析整数 ID（Go 1.22 路由变量）
func pathID(r *http.Request, name string) int64 {
	v, _ := strconv.ParseInt(r.PathValue(name), 10, 64)
	return v
}

func queryInt(r *http.Request, name string) int64 {
	v, _ := strconv.ParseInt(r.FormValue(name), 10, 64)
	return v
}

// ─── 项目管理 ───────────────────────────────────────────────────────────────

// GET /projects
func handleProjectsList(w http.ResponseWriter, r *http.Request) {
	uid := getSession(r).UserID()
	type Page struct {
		Base
		Projects []Project
	}
	render(w, "projects_list.html", Page{Base: baseData(r, "projects"), Projects: getProjectsByUser(uid)})
}

// GET/POST /projects/create
func handleProjectCreate(w http.ResponseWriter, r *http.Request) {
	s := getSession(r)
	uid := s.UserID()
	if r.Method == http.MethodPost {
		if !requireCSRF(w, r) {
			return
		}
		name := strings.TrimSpace(r.FormValue("name"))
		desc := strings.TrimSpace(r.FormValue("description"))
		if name == "" {
			s.Flash("error", "请输入项目名称")
		} else if _, err := createProject(uid, name, desc); err != nil {
			s.Flash("error", "创建项目失败: "+err.Error())
		} else {
			s.Flash("success", "项目创建成功")
			http.Redirect(w, r, "/projects", http.StatusFound)
			return
		}
		http.Redirect(w, r, "/projects/create", http.StatusFound)
		return
	}
	type Page struct {
		Base
		Title  string
		Proj   *Project
		Name   string
		Desc   string
		Action string
	}
	render(w, "projects_form.html", Page{
		Base: baseData(r, "projects"), Title: "创建项目",
		Action: "/projects/create",
	})
}

// GET/POST /projects/{id}/edit
func handleProjectEdit(w http.ResponseWriter, r *http.Request) {
	s := getSession(r)
	uid := s.UserID()
	id := pathID(r, "id")
	proj := getProjectByID(id)
	if proj == nil || proj.UserID != uid {
		s.Flash("error", "项目不存在")
		http.Redirect(w, r, "/projects", http.StatusFound)
		return
	}
	if r.Method == http.MethodPost {
		if !requireCSRF(w, r) {
			return
		}
		name := strings.TrimSpace(r.FormValue("name"))
		desc := strings.TrimSpace(r.FormValue("description"))
		if name == "" {
			s.Flash("error", "请输入项目名称")
		} else if err := updateProject(id, name, desc); err != nil {
			s.Flash("error", "更新项目失败: "+err.Error())
		} else {
			s.Flash("success", "项目更新成功")
			http.Redirect(w, r, "/projects", http.StatusFound)
			return
		}
		http.Redirect(w, r, "/projects/"+strconv.FormatInt(id, 10)+"/edit", http.StatusFound)
		return
	}
	type Page struct {
		Base
		Title  string
		Proj   *Project
		Name   string
		Desc   string
		Action string
	}
	render(w, "projects_form.html", Page{
		Base: baseData(r, "projects"), Title: "编辑项目", Proj: proj,
		Name:   proj.Name,
		Desc:   proj.Description,
		Action: "/projects/" + strconv.FormatInt(id, 10) + "/edit",
	})
}

// POST /projects/{id}/delete
func handleProjectDelete(w http.ResponseWriter, r *http.Request) {
	s := getSession(r)
	uid := s.UserID()
	id := pathID(r, "id")
	proj := getProjectByID(id)
	if proj == nil || proj.UserID != uid {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "项目不存在"})
		return
	}
	if proj.IsDefault {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "不能删除默认项目"})
		return
	}
	if !requireCSRF(w, r) {
		return
	}

	defaultProj := getDefaultProject(uid)
	if defaultProj == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "系统错误：找不到默认项目"})
		return
	}
	defaultTask := getDefaultTask(defaultProj.ID)
	if defaultTask == nil {
		defaultTask = getDefaultTaskGlobal(uid)
	}
	if defaultTask == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "系统错误：找不到默认任务"})
		return
	}

	// 转移工时记录到默认项目的默认任务
	transferEntriesToDefault(id, defaultProj.ID, defaultTask.ID)
	// 软删除项目下所有任务
	softDeleteTasksByProject(id)
	// 软删除项目
	softDeleteProject(id)

	s.Flash("success", "项目已删除，工时记录已转移")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ─── 任务管理 ───────────────────────────────────────────────────────────────

// GET /tasks?project_id=N
func handleTasksList(w http.ResponseWriter, r *http.Request) {
	s := getSession(r)
	uid := s.UserID()
	projectID := queryInt(r, "project_id")
	if projectID == 0 {
		s.Flash("error", "请选择项目")
		http.Redirect(w, r, "/projects", http.StatusFound)
		return
	}
	proj := getProjectByID(projectID)
	if proj == nil || proj.UserID != uid {
		s.Flash("error", "项目不存在")
		http.Redirect(w, r, "/projects", http.StatusFound)
		return
	}
	type Page struct {
		Base
		Proj  *Project
		Tasks []Task
	}
	render(w, "tasks_list.html", Page{Base: baseData(r, "projects"), Proj: proj, Tasks: getTasksByProject(projectID)})
}

// GET/POST /tasks/create?project_id=N
func handleTaskCreate(w http.ResponseWriter, r *http.Request) {
	s := getSession(r)
	uid := s.UserID()
	projectID := queryInt(r, "project_id")
	proj := getProjectByID(projectID)
	if proj == nil || proj.UserID != uid {
		s.Flash("error", "项目不存在")
		http.Redirect(w, r, "/projects", http.StatusFound)
		return
	}
	if r.Method == http.MethodPost {
		if !requireCSRF(w, r) {
			return
		}
		name := strings.TrimSpace(r.FormValue("name"))
		desc := strings.TrimSpace(r.FormValue("description"))
		if name == "" {
			s.Flash("error", "请输入任务名称")
		} else if _, err := createTask(projectID, name, desc); err != nil {
			s.Flash("error", "创建任务失败: "+err.Error())
		} else {
			s.Flash("success", "任务创建成功")
			http.Redirect(w, r, "/tasks?project_id="+strconv.FormatInt(projectID, 10), http.StatusFound)
			return
		}
		http.Redirect(w, r, "/tasks/create?project_id="+strconv.FormatInt(projectID, 10), http.StatusFound)
		return
	}
	type Page struct {
		Base
		Title  string
		Proj   *Project
		Task   *Task
		Name   string
		Desc   string
		Action string
	}
	render(w, "tasks_form.html", Page{
		Base: baseData(r, "projects"), Title: "创建任务", Proj: proj,
		Action: "/tasks/create?project_id=" + strconv.FormatInt(projectID, 10),
	})
}

// GET/POST /tasks/{id}/edit
func handleTaskEdit(w http.ResponseWriter, r *http.Request) {
	s := getSession(r)
	uid := s.UserID()
	id := pathID(r, "id")
	task := getTaskByID(id)
	if task == nil {
		s.Flash("error", "任务不存在")
		http.Redirect(w, r, "/projects", http.StatusFound)
		return
	}
	proj := getProjectByID(task.ProjectID)
	if proj == nil || proj.UserID != uid {
		s.Flash("error", "任务不存在")
		http.Redirect(w, r, "/projects", http.StatusFound)
		return
	}
	if r.Method == http.MethodPost {
		if !requireCSRF(w, r) {
			return
		}
		name := strings.TrimSpace(r.FormValue("name"))
		desc := strings.TrimSpace(r.FormValue("description"))
		if name == "" {
			s.Flash("error", "请输入任务名称")
		} else if err := updateTask(id, name, desc); err != nil {
			s.Flash("error", "更新任务失败: "+err.Error())
		} else {
			s.Flash("success", "任务更新成功")
			http.Redirect(w, r, "/tasks?project_id="+strconv.FormatInt(task.ProjectID, 10), http.StatusFound)
			return
		}
		http.Redirect(w, r, "/tasks/"+strconv.FormatInt(id, 10)+"/edit", http.StatusFound)
		return
	}
	type Page struct {
		Base
		Title  string
		Proj   *Project
		Task   *Task
		Name   string
		Desc   string
		Action string
	}
	render(w, "tasks_form.html", Page{
		Base: baseData(r, "projects"), Title: "编辑任务", Proj: proj, Task: task,
		Name:   task.Name,
		Desc:   task.Description,
		Action: "/tasks/" + strconv.FormatInt(id, 10) + "/edit",
	})
}

// GET/POST /tasks/{id}/delete
// GET 仅返回工时记录数（供前端弹窗提示），POST 执行删除
func handleTaskDelete(w http.ResponseWriter, r *http.Request) {
	s := getSession(r)
	uid := s.UserID()
	id := pathID(r, "id")
	task := getTaskByID(id)
	if task == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "任务不存在"})
		return
	}
	if task.IsDefault {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "不能删除默认任务"})
		return
	}
	proj := getProjectByID(task.ProjectID)
	if proj == nil || proj.UserID != uid {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "任务不存在"})
		return
	}

	entryCount := countEntriesByTask(id)
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, map[string]any{"entry_count": entryCount})
		return
	}
	if !requireCSRF(w, r) {
		return
	}

	if entryCount > 0 {
		// 转移到同项目默认任务
		defaultTask := getDefaultTask(task.ProjectID)
		if defaultTask == nil {
			// 兜底：转移到全局默认项目的默认任务
			defaultTask = getDefaultTaskGlobal(uid)
		}
		if defaultTask != nil {
			transferEntriesToTask(id, defaultTask.ID)
		}
	}
	softDeleteTask(id)

	s.Flash("success", "任务已删除")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "entry_count": entryCount})
}
