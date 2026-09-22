package main

import (
	"bytes"
	"encoding/csv"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// GET/POST /export
// 自定义日期范围导出 CSV（含 BOM 头，兼容 Excel 中文）
func handleExport(w http.ResponseWriter, r *http.Request) {
	s := getSession(r)
	uid := s.UserID()

	startStr := r.FormValue("start_date")
	endStr := r.FormValue("end_date")

	if r.Method == http.MethodPost && startStr != "" && endStr != "" {
		if !requireCSRF(w, r) {
			return
		}
		startDate := parseDateOr(startStr, time.Time{})
		endDate := parseDateOr(endStr, time.Time{})
		if startDate.IsZero() || endDate.IsZero() {
			s.Flash("error", "日期格式无效")
			http.Redirect(w, r, "/export", http.StatusFound)
			return
		}
		if startDate.After(endDate) {
			s.Flash("error", "起始日期不能晚于结束日期")
			http.Redirect(w, r, "/export", http.StatusFound)
			return
		}

		entries := getEntriesByDateRange(uid, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))

		var buf bytes.Buffer
		buf.Write([]byte{0xEF, 0xBB, 0xBF}) // BOM
		cw := csv.NewWriter(&buf)
		cw.Write([]string{"日期", "项目名称", "任务名称", "工作内容", "工时(分钟)", "节假期加班", "记录ID"})
		for _, e := range entries {
			entryDate := parseDateOr(e.EntryDate, time.Time{})
			holidayOT := ""
			if !entryDate.IsZero() && isOffDay(entryDate) {
				holidayOT = "是"
			}
			cw.Write([]string{
				e.EntryDate,
				e.ProjectName,
				e.TaskName,
				e.Content,
				strconv.Itoa(e.Minutes),
				holidayOT,
				strconv.FormatInt(e.ID, 10),
			})
		}
		cw.Flush()

		filename := "workTime_" + startDate.Format("2006-01-02") + "_" + endDate.Format("2006-01-02") + ".csv"
		filenameEncoded := url.PathEscape(filename)
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition",
			"attachment; filename="+filenameEncoded+"; filename*=UTF-8''"+filenameEncoded)
		w.Write(buf.Bytes())
		return
	}

	type Page struct {
		Base
		StartDate string
		EndDate   string
	}
	render(w, "export.html", Page{
		Base:      baseData(r, "export"),
		StartDate: "2020-01-01",
		EndDate:   todayStr(),
	})
}
