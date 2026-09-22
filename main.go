package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	_ "time/tzdata" // 内嵌 IANA 时区库，无 zoneinfo 的系统（OpenWrt）也能解析 Asia/Shanghai
)

// initTimezone 修正运行时时区。
// Go 在找不到 zoneinfo 的系统（如 OpenWrt/musl）会回退 UTC，导致日志、备份、
// "今天"判定等全部偏差。优先级：显式 TZ 环境变量 → /etc/TZ（OpenWrt POSIX 格式）→ 系统默认。
func initTimezone() {
	if _, ok := os.LookupEnv("TZ"); ok {
		return // 用户显式指定 TZ，交给标准库（配合内嵌 tzdata 可解析 Asia/Shanghai）
	}
	if time.Local.String() != "UTC" {
		return // 系统时区已正确（/etc/localtime 或 zoneinfo 存在）
	}
	data, err := os.ReadFile("/etc/TZ")
	if err != nil {
		return
	}
	tz := strings.TrimSpace(string(data))
	if tz == "" {
		return
	}
	if loc, err := time.LoadLocation(tz); err == nil {
		time.Local = loc
		log.Printf("时区: %s（来自 /etc/TZ）", tz)
		return
	}
	// 解析 POSIX TZ 形如 CST-8 / CST-8:00（POSIX 符号反直觉：-8 表示东八区）
	if m := regexp.MustCompile(`^([A-Za-z]+)([+-]?\d{1,2})(:\d{2})?$`).FindStringSubmatch(tz); m != nil {
		if n, err := strconv.Atoi(m[2]); err == nil {
			time.Local = time.FixedZone(m[1], -n*3600)
			log.Printf("时区: %s（UTC%+d，来自 /etc/TZ）", m[1], -n)
		}
	}
}

func main() {
	initTimezone()

	host := flag.String("host", "127.0.0.1", "监听地址")
	port := flag.Int("port", 5000, "监听端口")
	flag.Parse()

	dataDir := getDataDir()
	if err := os.MkdirAll(filepath.Join(dataDir, "instance"), 0o755); err != nil {
		log.Fatalf("创建数据目录失败: %v", err)
	}

	if err := initDB(dbPath()); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}

	if err := initSessionSecret(); err != nil {
		log.Fatalf("初始化会话密钥失败: %v", err)
	}

	loadHolidays()

	// 启动时备份数据库
	if p, err := doBackup(); err == nil && p != "" {
		log.Printf("数据库已备份: %s", p)
	}
	go startWeeklyBackup()

	addr := fmt.Sprintf("%s:%d", *host, *port)
	log.Printf("workTime v%s (Go %s) 启动: http://%s", AppVersion, runtime.Version(), addr)

	mux := buildMux()
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
