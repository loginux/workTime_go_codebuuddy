package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
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
	log.Printf("workTime 启动: http://%s", addr)

	mux := buildMux()
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
