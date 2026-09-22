package main

import (
	"os"
	"path/filepath"
	"strings"
)

// AppVersion 应用版本号，CI 构建时通过 -ldflags 注入实际 Release tag（如 v1.4）；
// 本地 go run / go build 时显示 dev
var AppVersion = "dev"

// getDataDir 返回数据存储目录（数据库、备份、会话密钥）。
// 默认放在 exe 同级目录（单文件分发模式）；可用环境变量 WORKTIME_DATA_DIR 覆盖。
func getDataDir() string {
	if v := os.Getenv("WORKTIME_DATA_DIR"); v != "" {
		return v
	}
	exe, err := os.Executable()
	if err != nil {
		wd, _ := os.Getwd()
		return wd
	}
	if strings.Contains(exe, "go-build") {
		// go run 开发模式：临时目录，改用工作目录
		wd, _ := os.Getwd()
		return wd
	}
	return filepath.Dir(exe)
}

func dbPath() string {
	return filepath.Join(getDataDir(), "instance", "worktime.db")
}
