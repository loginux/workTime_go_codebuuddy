package main

import (
	_ "embed"
	"os"
	"path/filepath"
	"strings"
)

// AppVersion 应用版本号。
// 来源：CI 构建时把 Release tag 写入 version.txt，编译时通过 go:embed 嵌入；
// 本地 go run / go build 使用仓库中的 version.txt（内容为 dev）。
var AppVersion = readVersion()

//go:embed version.txt
var versionFile string

func readVersion() string {
	return strings.TrimSpace(versionFile)
}

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
