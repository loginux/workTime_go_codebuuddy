package main

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// parseHHMM 解析 HH:MM 格式字符串，返回分钟数（0:00 起算），失败返回 -1
func parseHHMM(s string) int {
	parts := strings.SplitN(strings.TrimSpace(s), ":", 2)
	if len(parts) != 2 {
		return -1
	}
	h, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || h < 0 || h > 24 || m < 0 || m > 59 {
		return -1
	}
	return h*60 + m
}

// calcMinutes 计算起止时间之间的分钟数（排除午休 12:00-13:00），无效返回 -1
func calcMinutes(startStr, endStr string) int {
	start := parseHHMM(startStr)
	end := parseHHMM(endStr)
	if start < 0 || end < 0 {
		return -1
	}
	total := end - start

	// 午休时间：12:00 (720) - 13:00 (780)
	const lunchStart, lunchEnd = 12 * 60, 13 * 60
	if start < lunchEnd && end > lunchStart {
		overlapStart := start
		if lunchStart > overlapStart {
			overlapStart = lunchStart
		}
		overlapEnd := end
		if lunchEnd < overlapEnd {
			overlapEnd = lunchEnd
		}
		if overlapEnd-overlapStart > 0 {
			total -= overlapEnd - overlapStart
		}
	}
	if total < 0 {
		return -1
	}
	return total
}

func todayStr() string {
	return time.Now().Format("2006-01-02")
}

func parseDateOr(s string, fallback time.Time) time.Time {
	if d, err := time.ParseInLocation("2006-01-02", s, time.Local); err == nil {
		return d
	}
	return fallback
}

type timeEntryPage struct {
	Base
	Title        string
	Entry        *TimeEntry
	EntryDate    string
	ProjectID    int64
	Projects     []Project
	SelectedTask int64
	StartTime    string
	EndTime      string
	Minutes      string
	Content      string
	Action       string
	DayURL       string
}

// GET/POST /time_entries/create
func handleEntryCreate(w http.ResponseWriter, r *http.Request) {
	s := getSession(r)
	uid := s.UserID()
	projects := getProjectsByUser(uid)

	page := timeEntryPage{
		Base:     baseData(r, ""),
		Title:    "录入工时",
		Projects: projects,
		StartTime: "09:00",
		EndTime:   "18:00",
		Action:   "/time_entries/create",
	}

	// 如果指定了日期参数，预填
	if dateParam := r.FormValue("date"); dateParam != "" && r.Method == http.MethodGet {
		page.EntryDate = parseDateOr(dateParam, time.Now()).Format("2006-01-02")
	}

	if r.Method == http.MethodPost {
		if !requireCSRF(w, r) {
			return
		}
		page.EntryDate = r.FormValue("entry_date")
		page.ProjectID = queryInt(r, "project_id")
		taskID := queryInt(r, "task_id")
		page.StartTime = r.FormValue("start_time")
		page.EndTime = r.FormValue("end_time")
		page.Minutes = strings.TrimSpace(r.FormValue("minutes"))
		page.Content = r.FormValue("content")

		entryDate, err := time.ParseInLocation("2006-01-02", page.EntryDate, time.Local)
		if err != nil {
			s.Flash("error", "请选择日期")
			render(w, "time_entries_form.html", page)
			return
		}
		if entryDate.After(time.Now()) {
			s.Flash("error", "日期不可晚于今天")
			render(w, "time_entries_form.html", page)
			return
		}

		// 计算分钟数：优先用表单提交的分钟数，否则从起止时间计算
		minutes := 0
		if v, err := strconv.Atoi(page.Minutes); err == nil && v > 0 {
			minutes = v
		} else if calc := calcMinutes(page.StartTime, page.EndTime); calc > 0 {
			minutes = calc
		} else {
			s.Flash("error", "请填写工时分钟数，或确保结束时间晚于开始时间")
			render(w, "time_entries_form.html", page)
			return
		}

		// 验证项目属于当前用户
		proj := getProjectByID(page.ProjectID)
		if proj == nil || proj.UserID != uid {
			s.Flash("error", "项目不存在")
			render(w, "time_entries_form.html", page)
			return
		}
		if err := createTimeEntry(uid, page.ProjectID, taskID, page.EntryDate, minutes, page.Content); err != nil {
			s.Flash("error", "记录失败: "+err.Error())
			render(w, "time_entries_form.html", page)
			return
		}
		s.Flash("success", "工时记录成功")
		http.Redirect(w, r, "/views/day?date="+page.EntryDate, http.StatusFound)
		return
	}

	// GET 时根据 URL 的 project_id 预选项目
	if r.Method == http.MethodGet {
		page.ProjectID = queryInt(r, "project_id")
	}
	page.DayURL = "/views/day?date=" + todayStr()
	render(w, "time_entries_form.html", page)
}

// GET/POST /time_entries/{id}/edit
func handleEntryEdit(w http.ResponseWriter, r *http.Request) {
	s := getSession(r)
	uid := s.UserID()
	id := pathID(r, "id")
	entry := getTimeEntryByID(id)
	if entry == nil || entry.UserID != uid {
		s.Flash("error", "记录不存在")
		http.Redirect(w, r, "/views/day", http.StatusFound)
		return
	}

	page := timeEntryPage{
		Base:      baseData(r, ""),
		Title:     "编辑工时",
		Entry:     entry,
		Projects:  getProjectsByUser(uid),
		StartTime: "09:00",
		EndTime:   "18:00",
		Action:    "/time_entries/" + strconv.FormatInt(id, 10) + "/edit",
	}

	if r.Method == http.MethodPost {
		if !requireCSRF(w, r) {
			return
		}
		page.EntryDate = r.FormValue("entry_date")
		page.ProjectID = queryInt(r, "project_id")
		taskID := queryInt(r, "task_id")
		page.StartTime = r.FormValue("start_time")
		page.EndTime = r.FormValue("end_time")
		page.Minutes = strings.TrimSpace(r.FormValue("minutes"))
		page.Content = r.FormValue("content")

		entryDate, err := time.ParseInLocation("2006-01-02", page.EntryDate, time.Local)
		if err != nil {
			s.Flash("error", "请选择日期")
			render(w, "time_entries_form.html", page)
			return
		}
		if entryDate.After(time.Now()) {
			s.Flash("error", "日期不可晚于今天")
			render(w, "time_entries_form.html", page)
			return
		}

		minutes := 0
		if v, err := strconv.Atoi(page.Minutes); err == nil && v > 0 {
			minutes = v
		} else if calc := calcMinutes(page.StartTime, page.EndTime); calc > 0 {
			minutes = calc
		} else {
			s.Flash("error", "请填写工时分钟数，或确保结束时间晚于开始时间")
			render(w, "time_entries_form.html", page)
			return
		}

		proj := getProjectByID(page.ProjectID)
		if proj == nil || proj.UserID != uid {
			s.Flash("error", "项目不存在")
			render(w, "time_entries_form.html", page)
			return
		}
		if err := updateTimeEntry(id, page.ProjectID, taskID, page.EntryDate, minutes, page.Content); err != nil {
			s.Flash("error", "更新失败: "+err.Error())
			render(w, "time_entries_form.html", page)
			return
		}
		s.Flash("success", "工时记录已更新")
		http.Redirect(w, r, "/views/day?date="+page.EntryDate, http.StatusFound)
		return
	}

	// GET：预填表单
	page.EntryDate = entry.EntryDate
	page.ProjectID = entry.ProjectID
	page.SelectedTask = entry.TaskID
	page.Minutes = strconv.Itoa(entry.Minutes)
	page.Content = entry.Content
	render(w, "time_entries_form.html", page)
}

// POST /time_entries/{id}/delete
func handleEntryDelete(w http.ResponseWriter, r *http.Request) {
	s := getSession(r)
	uid := s.UserID()
	id := pathID(r, "id")
	entry := getTimeEntryByID(id)
	if entry == nil || entry.UserID != uid {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "记录不存在"})
		return
	}
	if !requireCSRF(w, r) {
		return
	}
	deleteTimeEntry(id)
	s.Flash("success", "工时记录已删除")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// GET /time_entries/tasks_for_project?project_id=N
// 获取某项目下的任务列表（API，用于前端动态加载）
func handleTasksForProject(w http.ResponseWriter, r *http.Request) {
	s := getSession(r)
	uid := s.UserID()
	projectID := queryInt(r, "project_id")
	if projectID == 0 {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	// 校验项目归属
	proj := getProjectByID(projectID)
	if proj == nil || proj.UserID != uid {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	tasks := getTasksByProject(projectID)
	out := make([]map[string]any, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, map[string]any{"id": t.ID, "name": t.Name})
	}
	writeJSON(w, http.StatusOK, out)
}
