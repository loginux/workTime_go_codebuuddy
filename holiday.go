package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// 法定节假日模块：仅从外部目录加载年度节假日配置（holiday-cn 标准格式）。
// 目录优先级：exe 同级 holiday/ → 工作目录 holiday/。
// 文件命名：{年份}.json，如 2026.json。

var (
	holidayMu   sync.RWMutex
	holidayData map[string]HolidayInfo
)

type HolidayInfo struct {
	Name    string `json:"name"`
	IsOffDay bool  `json:"is_off_day"`
}

type holidayFile struct {
	Year int `json:"year"`
	Days []struct {
		Name     string `json:"name"`
		Date     string `json:"date"`
		IsOffDay *bool  `json:"isOffDay"`
	} `json:"days"`
}

func holidayDirs() []string {
	var dirs []string
	if exe, err := os.Executable(); err == nil {
		dirs = append(dirs, filepath.Join(filepath.Dir(exe), "holiday"))
	}
	wd, _ := os.Getwd()
	dirs = append(dirs, filepath.Join(wd, "holiday"))
	return dirs
}

func loadHolidays() {
	result := map[string]HolidayInfo{}
	for _, dir := range holidayDirs() {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				continue
			}
			var hf holidayFile
			if err := json.Unmarshal(data, &hf); err != nil {
				log.Printf("节假日文件解析失败 %s: %v", e.Name(), err)
				continue
			}
			for _, d := range hf.Days {
				if d.Date == "" || d.Name == "" {
					continue
				}
				off := true
				if d.IsOffDay != nil {
					off = *d.IsOffDay
				}
				result[d.Date] = HolidayInfo{Name: d.Name, IsOffDay: off}
			}
		}
	}
	holidayMu.Lock()
	holidayData = result
	holidayMu.Unlock()
	if len(result) > 0 {
		log.Printf("已加载节假日配置 %d 天", len(result))
	}
}

// getHolidayInfo 获取某天的节假日信息，非节假日返回 nil
func getHolidayInfo(d time.Time) *HolidayInfo {
	holidayMu.RLock()
	defer holidayMu.RUnlock()
	if info, ok := holidayData[d.Format("2006-01-02")]; ok {
		return &info
	}
	return nil
}

// isOffDay 判断某天是否为休息日（法定节假日或周末，排除调休上班）
func isOffDay(d time.Time) bool {
	if info := getHolidayInfo(d); info != nil {
		return info.IsOffDay
	}
	wd := d.Weekday()
	return wd == time.Saturday || wd == time.Sunday
}
