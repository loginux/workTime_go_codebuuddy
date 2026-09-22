package main

import "database/sql"

// ─── User ───────────────────────────────────────────────────────────────────

type User struct {
	ID        int64
	Username  string
	CreatedAt string
}

func getUserByID(id int64) *User {
	row := db.QueryRow("SELECT id, username, created_at FROM users WHERE id = ?", id)
	var u User
	if err := row.Scan(&u.ID, &u.Username, &u.CreatedAt); err != nil {
		return nil
	}
	return &u
}

func getUserByUsername(username string) *User {
	row := db.QueryRow("SELECT id, username, created_at FROM users WHERE username = ?", username)
	var u User
	if err := row.Scan(&u.ID, &u.Username, &u.CreatedAt); err != nil {
		return nil
	}
	return &u
}

func getAllUsers() []User {
	rows, err := db.Query("SELECT id, username, created_at FROM users ORDER BY created_at ASC")
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.CreatedAt); err == nil {
			out = append(out, u)
		}
	}
	return out
}

func createUser(username string) (int64, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	res, err := tx.Exec("INSERT INTO users (username) VALUES (?)", username)
	if err != nil {
		return 0, err
	}
	userID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	res, err = tx.Exec(
		"INSERT INTO projects (user_id, name, description, is_default) VALUES (?, '默认项目', '', 1)",
		userID,
	)
	if err != nil {
		return 0, err
	}
	projID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if _, err := tx.Exec(
		"INSERT INTO tasks (project_id, name, description, is_default) VALUES (?, '默认任务', '', 1)",
		projID,
	); err != nil {
		return 0, err
	}
	return userID, tx.Commit()
}

// ─── Project ────────────────────────────────────────────────────────────────

type Project struct {
	ID          int64
	UserID      int64
	Name        string
	Description string
	IsDefault   bool
	IsDeleted   bool
	CreatedAt   string
}

const projectCols = "id, user_id, name, description, is_default, is_deleted, created_at"

func scanProject(scan func(dest ...any) error) *Project {
	var p Project
	var isDefault, isDeleted int
	if err := scan(&p.ID, &p.UserID, &p.Name, &p.Description, &isDefault, &isDeleted, &p.CreatedAt); err != nil {
		return nil
	}
	p.IsDefault = isDefault == 1
	p.IsDeleted = isDeleted == 1
	return &p
}

func getProjectsByUser(userID int64) []Project {
	rows, err := db.Query(
		"SELECT "+projectCols+" FROM projects WHERE user_id = ? AND is_deleted = 0 ORDER BY is_default DESC, created_at ASC",
		userID,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []Project
	for rows.Next() {
		if p := scanProject(rows.Scan); p != nil {
			out = append(out, *p)
		}
	}
	return out
}

func getProjectByID(id int64) *Project {
	row := db.QueryRow("SELECT "+projectCols+" FROM projects WHERE id = ? AND is_deleted = 0", id)
	return scanProject(row.Scan)
}

func getDefaultProject(userID int64) *Project {
	row := db.QueryRow(
		"SELECT "+projectCols+" FROM projects WHERE user_id = ? AND is_default = 1 AND is_deleted = 0",
		userID,
	)
	return scanProject(row.Scan)
}

func createProject(userID int64, name, description string) (int64, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	res, err := tx.Exec(
		"INSERT INTO projects (user_id, name, description) VALUES (?, ?, ?)",
		userID, name, description,
	)
	if err != nil {
		return 0, err
	}
	projID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if _, err := tx.Exec(
		"INSERT INTO tasks (project_id, name, description, is_default) VALUES (?, '默认任务', '', 1)",
		projID,
	); err != nil {
		return 0, err
	}
	return projID, tx.Commit()
}

func updateProject(id int64, name, description string) error {
	_, err := db.Exec("UPDATE projects SET name = ?, description = ? WHERE id = ?", name, description, id)
	return err
}

func softDeleteProject(id int64) error {
	_, err := db.Exec("UPDATE projects SET is_deleted = 1 WHERE id = ?", id)
	return err
}

func softDeleteTasksByProject(projectID int64) error {
	_, err := db.Exec("UPDATE tasks SET is_deleted = 1 WHERE project_id = ?", projectID)
	return err
}

// ─── Task ───────────────────────────────────────────────────────────────────

type Task struct {
	ID          int64
	ProjectID   int64
	Name        string
	Description string
	IsDefault   bool
	IsDeleted   bool
	CreatedAt   string
}

const taskCols = "id, project_id, name, description, is_default, is_deleted, created_at"

func scanTask(scan func(dest ...any) error) *Task {
	var t Task
	var isDefault, isDeleted int
	if err := scan(&t.ID, &t.ProjectID, &t.Name, &t.Description, &isDefault, &isDeleted, &t.CreatedAt); err != nil {
		return nil
	}
	t.IsDefault = isDefault == 1
	t.IsDeleted = isDeleted == 1
	return &t
}

func getTasksByProject(projectID int64) []Task {
	rows, err := db.Query(
		"SELECT "+taskCols+" FROM tasks WHERE project_id = ? AND is_deleted = 0 ORDER BY is_default DESC, created_at ASC",
		projectID,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []Task
	for rows.Next() {
		if t := scanTask(rows.Scan); t != nil {
			out = append(out, *t)
		}
	}
	return out
}

func getTaskByID(id int64) *Task {
	row := db.QueryRow("SELECT "+taskCols+" FROM tasks WHERE id = ? AND is_deleted = 0", id)
	return scanTask(row.Scan)
}

func getDefaultTask(projectID int64) *Task {
	row := db.QueryRow(
		"SELECT "+taskCols+" FROM tasks WHERE project_id = ? AND is_default = 1 AND is_deleted = 0",
		projectID,
	)
	return scanTask(row.Scan)
}

// getDefaultTaskGlobal 获取全局默认项目下的默认任务
func getDefaultTaskGlobal(userID int64) *Task {
	proj := getDefaultProject(userID)
	if proj == nil {
		return nil
	}
	return getDefaultTask(proj.ID)
}

func createTask(projectID int64, name, description string) (int64, error) {
	res, err := db.Exec(
		"INSERT INTO tasks (project_id, name, description) VALUES (?, ?, ?)",
		projectID, name, description,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func updateTask(id int64, name, description string) error {
	_, err := db.Exec("UPDATE tasks SET name = ?, description = ? WHERE id = ?", name, description, id)
	return err
}

func softDeleteTask(id int64) error {
	_, err := db.Exec("UPDATE tasks SET is_deleted = 1 WHERE id = ?", id)
	return err
}

// ─── Time Entry ─────────────────────────────────────────────────────────────

type TimeEntry struct {
	ID          int64
	UserID      int64
	ProjectID   int64
	TaskID      int64
	EntryDate   string
	Minutes     int
	CreatedAt   string
	Content     string
	UpdatedAt   string
	ProjectName string
	TaskName    string
}

const entryCols = `te.id, te.user_id, te.project_id, te.task_id, te.entry_date, te.minutes,
 te.created_at, te.content, te.updated_at, p.name AS project_name, t.name AS task_name`

const entryJoin = `FROM time_entries te
 JOIN projects p ON te.project_id = p.id
 JOIN tasks t ON te.task_id = t.id`

func scanEntry(scan func(dest ...any) error) *TimeEntry {
	var e TimeEntry
	if err := scan(&e.ID, &e.UserID, &e.ProjectID, &e.TaskID, &e.EntryDate, &e.Minutes,
		&e.CreatedAt, &e.Content, &e.UpdatedAt, &e.ProjectName, &e.TaskName); err != nil {
		return nil
	}
	return &e
}

func getEntriesByDate(userID int64, date string) []TimeEntry {
	rows, err := db.Query(
		"SELECT "+entryCols+" "+entryJoin+" WHERE te.user_id = ? AND te.entry_date = ? ORDER BY te.created_at ASC",
		userID, date,
	)
	if err != nil {
		return nil
	}
	return collectEntries(rows)
}

func getEntriesByDateRange(userID int64, startDate, endDate string) []TimeEntry {
	rows, err := db.Query(
		"SELECT "+entryCols+" "+entryJoin+" WHERE te.user_id = ? AND te.entry_date BETWEEN ? AND ? ORDER BY te.entry_date ASC, te.created_at ASC",
		userID, startDate, endDate,
	)
	if err != nil {
		return nil
	}
	return collectEntries(rows)
}

func collectEntries(rows *sql.Rows) []TimeEntry {
	defer rows.Close()
	var out []TimeEntry
	for rows.Next() {
		if e := scanEntry(rows.Scan); e != nil {
			out = append(out, *e)
		}
	}
	return out
}

func getTimeEntryByID(id int64) *TimeEntry {
	row := db.QueryRow(
		"SELECT "+entryCols+" "+entryJoin+" WHERE te.id = ?",
		id,
	)
	return scanEntry(row.Scan)
}

func createTimeEntry(userID, projectID, taskID int64, entryDate string, minutes int, content string) error {
	_, err := db.Exec(
		"INSERT INTO time_entries (user_id, project_id, task_id, entry_date, minutes, content) VALUES (?, ?, ?, ?, ?, ?)",
		userID, projectID, taskID, entryDate, minutes, content,
	)
	return err
}

func updateTimeEntry(id, projectID, taskID int64, entryDate string, minutes int, content string) error {
	_, err := db.Exec(
		`UPDATE time_entries SET project_id = ?, task_id = ?, entry_date = ?, minutes = ?, content = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		projectID, taskID, entryDate, minutes, content, id,
	)
	return err
}

func deleteTimeEntry(id int64) error {
	_, err := db.Exec("DELETE FROM time_entries WHERE id = ?", id)
	return err
}

func countEntriesByTask(taskID int64) int {
	var cnt int
	row := db.QueryRow("SELECT COUNT(*) FROM time_entries WHERE task_id = ?", taskID)
	if err := row.Scan(&cnt); err != nil {
		return 0
	}
	return cnt
}

func transferEntriesToTask(fromTaskID, toTaskID int64) error {
	_, err := db.Exec(
		"UPDATE time_entries SET task_id = ?, updated_at = CURRENT_TIMESTAMP WHERE task_id = ?",
		toTaskID, fromTaskID,
	)
	return err
}

func transferEntriesToDefault(projectID, defaultProjectID, defaultTaskID int64) error {
	_, err := db.Exec(
		`UPDATE time_entries SET project_id = ?, task_id = ?, updated_at = CURRENT_TIMESTAMP WHERE project_id = ?`,
		defaultProjectID, defaultTaskID, projectID,
	)
	return err
}

// ─── 汇总 ───────────────────────────────────────────────────────────────────

type DailyTotal struct {
	EntryDate    string
	TotalMinutes int
}

func getDailySummary(userID int64, startDate, endDate string) []DailyTotal {
	rows, err := db.Query(
		`SELECT te.entry_date, SUM(te.minutes) FROM time_entries te
		 JOIN projects p ON te.project_id = p.id
		 WHERE te.user_id = ? AND te.entry_date BETWEEN ? AND ? AND p.is_deleted = 0
		 GROUP BY te.entry_date ORDER BY te.entry_date ASC`,
		userID, startDate, endDate,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []DailyTotal
	for rows.Next() {
		var d DailyTotal
		if err := rows.Scan(&d.EntryDate, &d.TotalMinutes); err == nil {
			out = append(out, d)
		}
	}
	return out
}

type PTSummary struct {
	ProjectID    int64
	TaskID       int64
	ProjectName  string
	TaskName     string
	TotalMinutes int
}

func getProjectTaskSummary(userID int64, date string) []PTSummary {
	rows, err := db.Query(
		`SELECT te.project_id, te.task_id, p.name, t.name, SUM(te.minutes)
		 FROM time_entries te
		 JOIN projects p ON te.project_id = p.id
		 JOIN tasks t ON te.task_id = t.id
		 WHERE te.user_id = ? AND te.entry_date = ? AND p.is_deleted = 0 AND t.is_deleted = 0
		 GROUP BY te.project_id, te.task_id
		 ORDER BY p.name, t.name`,
		userID, date,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []PTSummary
	for rows.Next() {
		var s PTSummary
		if err := rows.Scan(&s.ProjectID, &s.TaskID, &s.ProjectName, &s.TaskName, &s.TotalMinutes); err == nil {
			out = append(out, s)
		}
	}
	return out
}
