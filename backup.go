package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// 数据库备份模块
// - 启动时备份
// - 运行中每周六 03:00 自动备份
// - 最多保留 9 个备份版本
// - 备份文件存放在数据库同级目录

const backupPrefix = "worktime.db.backup-"
const maxBackups = 9

func doBackup() (string, error) {
	src := dbPath()
	f, err := os.Open(src)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	defer f.Close()

	dest := filepath.Join(filepath.Dir(src), backupPrefix+time.Now().Format("20060102-150405"))
	out, err := os.Create(dest)
	if err != nil {
		return "", err
	}
	defer out.Close()
	if _, err := io.Copy(out, f); err != nil {
		os.Remove(dest)
		return "", err
	}
	cleanupBackups()
	return dest, nil
}

func cleanupBackups() {
	dir := filepath.Dir(dbPath())
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), backupPrefix) {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	sort.Slice(files, func(i, j int) bool {
		ai, _ := os.Stat(files[i])
		aj, _ := os.Stat(files[j])
		return ai.ModTime().After(aj.ModTime())
	})
	for _, old := range files[maxBackups:] {
		os.Remove(old)
	}
}

// startWeeklyBackup 后台协程：每小时检查一次，周六 03:00 执行备份
func startWeeklyBackup() {
	for {
		now := time.Now()
		if now.Weekday() == time.Saturday && now.Hour() == 3 {
			if _, err := doBackup(); err != nil {
				log.Printf("定时备份失败: %v", err)
			}
			time.Sleep(61 * time.Second) // 避免同一分钟重复触发
		} else {
			time.Sleep(3600 * time.Second)
		}
	}
}
