package main

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

var db *sql.DB

const schemaSQL = `
CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS projects (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    description TEXT DEFAULT '',
    is_default INTEGER DEFAULT 0,
    is_deleted INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    description TEXT DEFAULT '',
    is_default INTEGER DEFAULT 0,
    is_deleted INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (project_id) REFERENCES projects(id)
);

CREATE TABLE IF NOT EXISTS time_entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    project_id INTEGER NOT NULL,
    task_id INTEGER NOT NULL,
    entry_date DATE NOT NULL,
    minutes INTEGER NOT NULL CHECK(minutes > 0),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    content TEXT DEFAULT '',
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (project_id) REFERENCES projects(id),
    FOREIGN KEY (task_id) REFERENCES tasks(id)
);
`

func initDB(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	// 兼容旧版 Python 数据库：worktime.db 不存在而 woktime.db 存在时自动复制沿用
	if _, err := os.Stat(path); os.IsNotExist(err) {
		old := filepath.Join(filepath.Dir(path), "woktime.db")
		if _, err2 := os.Stat(old); err2 == nil {
			if src, err3 := os.Open(old); err3 == nil {
				dst, err4 := os.Create(path)
				if err4 == nil {
					_, err5 := io.Copy(dst, src)
					src.Close()
					dst.Close()
					if err5 == nil {
						log.Printf("已自动沿用旧版数据库: %s -> %s", old, path)
					}
				} else {
					src.Close()
				}
			}
		}
	}
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)", filepath.ToSlash(path))
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return err
	}
	// SQLite 单写者：限制为单连接，避免并发写锁冲突
	conn.SetMaxOpenConns(1)
	if _, err := conn.Exec(schemaSQL); err != nil {
		conn.Close()
		return err
	}
	// 兼容旧数据库：新增 content 字段（已存在则忽略错误）
	conn.Exec("ALTER TABLE time_entries ADD COLUMN content TEXT DEFAULT ''")
	db = conn
	log.Printf("数据库已就绪: %s", path)
	return nil
}
