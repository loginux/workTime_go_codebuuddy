package main

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ─── 图表数据辅助 ───────────────────────────────────────────────────────────

// series 保持插入顺序的 label→分钟 汇总，用于 Chart.js 饼图
type series struct {
	labels []string
	data   []int
	index  map[string]int
}

func newSeries() *series {
	return &series{index: map[string]int{}}
}

func (s *series) add(label string, v int) {
	if i, ok := s.index[label]; ok {
		s.data[i] += v
		return
	}
	s.index[label] = len(s.labels)
	s.labels = append(s.labels, label)
	s.data = append(s.data, v)
}

func (s *series) isEmpty() bool { return len(s.labels) == 0 }

// json 返回可直接嵌入 <script> 的 JSON（template.JS 跳过转义）
func (s *series) json() (template.JS, template.JS) {
	lb, _ := json.Marshal(s.labels)
	dt, _ := json.Marshal(s.data)
	return template.JS(lb), template.JS(dt)
}

// shiftMonth 将日期向前/向后推一个月，自动处理月末边界
func shiftMonth(d time.Time, delta int) time.Time {
	total := d.Year()*12 + int(d.Month()) - 1 + delta
	year := total / 12
	month := time.Month(total%12 + 1)
	// 下月 0 号 = 本月最后一天
	lastDay := time.Date(year, month+1, 0, 0, 0, 0, 0, d.Location()).Day()
	day := d.Day()
	if day > lastDay {
		day = lastDay
	}
	return time.Date(year, month, day, 0, 0, 0, 0, d.Location())
}

func mondayOf(d time.Time) time.Time {
	offset := (int(d.Weekday()) + 6) % 7 // 周一=0
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, d.Location()).AddDate(0, 0, -offset)
}

func parseDateParam(r *http.Request, fallback time.Time) time.Time {
	return parseDateOr(r.FormValue("date"), fallback)
}

// ─── 日视图 ─────────────────────────────────────────────────────────────────

type dayPageData struct {
	Base
	ViewDate       string
	WeekdayCN      string
	PrevDate       string
	NextDate       string
	Entries        []TimeEntry
	Summary        []PTSummary
	TotalMinutes   int
	HasChart       bool
	ChartProjLabel template.JS
	ChartProjData  template.JS
	ChartTaskLabel template.JS
	ChartTaskData  template.JS
}

// GET /views/day
func handleDayView(w http.ResponseWriter, r *http.Request) {
	uid := getSession(r).UserID()
	today := time.Now()
	viewDate := parseDateParam(r, today)

	entries := getEntriesByDate(uid, viewDate.Format("2006-01-02"))
	summary := getProjectTaskSummary(uid, viewDate.Format("2006-01-02"))

	total := 0
	for _, e := range entries {
		total += e.Minutes
	}

	// 饼图数据
	projSeries := newSeries()
	taskSeries := newSeries()
	for _, sm := range summary {
		projSeries.add(sm.ProjectName, sm.TotalMinutes)
		taskSeries.add(sm.ProjectName+"-"+sm.TaskName, sm.TotalMinutes)
	}

	page := dayPageData{
		Base:        baseData(r, "day"),
		ViewDate:    viewDate.Format("2006-01-02"),
		WeekdayCN:   "日一二三四五六"[int(viewDate.Weekday()) : int(viewDate.Weekday())+1],
		PrevDate:    viewDate.AddDate(0, 0, -1).Format("2006-01-02"),
		NextDate:    viewDate.AddDate(0, 0, 1).Format("2006-01-02"),
		Entries:     entries,
		Summary:     summary,
		TotalMinutes: total,
		HasChart:    len(summary) > 0,
	}
	page.ChartProjLabel, page.ChartProjData = projSeries.json()
	page.ChartTaskLabel, page.ChartTaskData = taskSeries.json()
	render(w, "views_day.html", page)
}

// ─── 周视图 ─────────────────────────────────────────────────────────────────

type weekDayCard struct {
	DateISO    string
	MD         string
	Label      string
	Minutes    int
	IsToday    bool
}

type weekPageData struct {
	Base
	Monday         string
	Sunday         string
	PrevWeek       string
	NextWeek       string
	WeekDays       []weekDayCard
	WeekTotal      int
	BarLabels      template.JS
	BarData        template.JS
	BarDates       template.JS
	HasChart       bool
	ChartProjLabel template.JS
	ChartProjData  template.JS
	ChartTaskLabel template.JS
	ChartTaskData  template.JS
	DayURLPrefix   string
}

// GET /views/week
func handleWeekView(w http.ResponseWriter, r *http.Request) {
	uid := getSession(r).UserID()
	today := time.Now()
	baseDate := parseDateParam(r, today)

	monday := mondayOf(baseDate)
	sunday := monday.AddDate(0, 0, 6)

	entries := getEntriesByDateRange(uid, monday.Format("2006-01-02"), sunday.Format("2006-01-02"))

	weekLabels := []string{"一", "二", "三", "四", "五", "六", "日"}
	cards := make([]weekDayCard, 7)
	for i := range cards {
		d := monday.AddDate(0, 0, i)
		sameDay := func(a, b time.Time) bool {
			return a.Year() == b.Year() && a.YearDay() == b.YearDay()
		}
		cards[i] = weekDayCard{
			DateISO: d.Format("2006-01-02"),
			MD:      strconv.Itoa(int(d.Month())) + "/" + strconv.Itoa(d.Day()),
			Label:   "周" + weekLabels[i],
			IsToday: sameDay(d, today),
		}
	}

	weekTotal := 0
	projSeries := newSeries()
	taskSeries := newSeries()
	barLabels := make([]string, 7)
	barData := make([]int, 7)
	barDates := make([]string, 7)
	for _, e := range entries {
		idx := -1
		for i := range cards {
			if cards[i].DateISO == e.EntryDate {
				idx = i
				break
			}
		}
		if idx >= 0 {
			cards[idx].Minutes += e.Minutes
			barData[idx] += e.Minutes
		}
		weekTotal += e.Minutes
		projSeries.add(e.ProjectName, e.Minutes)
		taskSeries.add(e.ProjectName+"-"+e.TaskName, e.Minutes)
	}
	for i := range cards {
		barLabels[i] = cards[i].MD + " (" + cards[i].Label + ")"
		barDates[i] = cards[i].DateISO
	}

	page := weekPageData{
		Base:        baseData(r, "week"),
		Monday:      monday.Format("2006-01-02"),
		Sunday:      sunday.Format("2006-01-02"),
		PrevWeek:    monday.AddDate(0, 0, -7).Format("2006-01-02"),
		NextWeek:    monday.AddDate(0, 0, 7).Format("2006-01-02"),
		WeekDays:    cards,
		WeekTotal:   weekTotal,
		HasChart:    !projSeries.isEmpty(),
		DayURLPrefix: "/views/day?date=",
	}
	page.BarLabels = mustJSONJS(barLabels)
	page.BarData = mustJSONJS(barData)
	page.BarDates = mustJSONJS(barDates)
	page.ChartProjLabel, page.ChartProjData = projSeries.json()
	page.ChartTaskLabel, page.ChartTaskData = taskSeries.json()
	render(w, "views_week.html", page)
}

// ─── 月视图 ─────────────────────────────────────────────────────────────────

type monthCell struct {
	Day     int
	DateISO string
	Minutes int
	Projects []string
	HasLeave bool
	IsToday  bool
	Holiday  string
	OffDay   string // "yes" / "no" / ""（未知）
	Level    string
}

type monthPageData struct {
	Base
	FirstDay       string
	LastDay        string
	PrevMonth      string
	NextMonth      string
	Weeks          [][]*monthCell
	MonthTotal     int
	HasChart       bool
	ChartProjLabel template.JS
	ChartProjData  template.JS
	ChartTaskLabel template.JS
	ChartTaskData  template.JS
	DayURLPrefix   string
}

// GET /views/month
func handleMonthView(w http.ResponseWriter, r *http.Request) {
	uid := getSession(r).UserID()
	today := time.Now()
	defaultStart := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, today.Location())
	firstDay := parseDateParam(r, defaultStart)

	// 起始日期 → 下个月同日 - 1天（例如 6/5→7/4, 1/31→2/28）
	lastDay := shiftMonth(firstDay, 1).AddDate(0, 0, -1)

	entries := getEntriesByDateRange(uid, firstDay.Format("2006-01-02"), lastDay.Format("2006-01-02"))

	type dailyInfo struct {
		minutes  int
		projects []string
		projSet  map[string]bool
		hasLeave bool
	}
	dailyMap := map[string]*dailyInfo{}
	monthTotal := 0
	projSeries := newSeries()
	taskSeries := newSeries()
	for _, e := range entries {
		info, ok := dailyMap[e.EntryDate]
		if !ok {
			info = &dailyInfo{projSet: map[string]bool{}}
			dailyMap[e.EntryDate] = info
		}
		info.minutes += e.Minutes
		monthTotal += e.Minutes
		if strings.Contains(e.TaskName, "请假") {
			info.hasLeave = true
		}
		pt := e.ProjectName + "-" + e.TaskName
		if !info.projSet[pt] {
			info.projSet[pt] = true
			info.projects = append(info.projects, pt)
		}
		projSeries.add(e.ProjectName, e.Minutes)
		taskSeries.add(e.ProjectName+"-"+e.TaskName, e.Minutes)
	}

	// 构建网格（按周排列，从 first_day 的星期几开始）
	startWeekday := (int(firstDay.Weekday()) + 6) % 7 // 周一=0
	sameDay := func(a, b time.Time) bool {
		return a.Year() == b.Year() && a.YearDay() == b.YearDay()
	}
	var weeks [][]*monthCell
	currentDay := firstDay
	for row := 0; row < 6; row++ {
		var week []*monthCell
		for col := 0; col < 7; col++ {
			if currentDay.After(lastDay) {
				week = append(week, nil)
				continue
			}
			if row == 0 && col < startWeekday {
				week = append(week, nil)
				continue
			}
			info := dailyMap[currentDay.Format("2006-01-02")]
			if info == nil {
				info = &dailyInfo{}
			}
			cell := &monthCell{
				Day:      currentDay.Day(),
				DateISO:  currentDay.Format("2006-01-02"),
				Minutes:  info.minutes,
				Projects: info.projects,
				HasLeave: info.hasLeave,
				IsToday:  sameDay(currentDay, today),
			}
			// 节假日信息
			if h := getHolidayInfo(currentDay); h != nil {
				cell.Holiday = h.Name
				if h.IsOffDay {
					cell.OffDay = "yes"
				} else {
					cell.OffDay = "no"
				}
			} else if currentDay.Weekday() == time.Saturday || currentDay.Weekday() == time.Sunday {
				// 周末默认休息（除非是调休上班）
				cell.OffDay = "yes"
				cell.Holiday = "休息日"
			}
			// 色阶
			mins := cell.Minutes
			switch {
			case cell.HasLeave:
				cell.Level = "level-absent"
			case mins >= 690:
				cell.Level = "level-extreme"
			case mins >= 630:
				cell.Level = "level-high"
			case mins >= 570:
				cell.Level = "level-mid"
			case mins >= 510:
				cell.Level = "level-low"
			default:
				cell.Level = "level-none"
			}
			if cell.OffDay == "yes" && mins > 0 && (cell.Level == "level-none" || cell.Level == "level-low") {
				cell.Level = "level-mid"
			}
			week = append(week, cell)
			currentDay = currentDay.AddDate(0, 0, 1)
		}
		hasAny := false
		for _, c := range week {
			if c != nil {
				hasAny = true
				break
			}
		}
		if hasAny {
			weeks = append(weeks, week)
		}
		if currentDay.After(lastDay) {
			break
		}
	}

	page := monthPageData{
		Base:        baseData(r, "month"),
		FirstDay:    firstDay.Format("2006-01-02"),
		LastDay:     lastDay.Format("2006-01-02"),
		PrevMonth:   shiftMonth(firstDay, -1).Format("2006-01-02"),
		NextMonth:   shiftMonth(firstDay, 1).Format("2006-01-02"),
		Weeks:       weeks,
		MonthTotal:  monthTotal,
		HasChart:    !projSeries.isEmpty(),
		DayURLPrefix: "/views/day?date=",
	}
	page.ChartProjLabel, page.ChartProjData = projSeries.json()
	page.ChartTaskLabel, page.ChartTaskData = taskSeries.json()
	render(w, "views_month.html", page)
}

func mustJSONJS(v any) template.JS {
	b, _ := json.Marshal(v)
	return template.JS(b)
}
