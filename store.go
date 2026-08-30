package goliday

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
)

// Store 持有多个年份的稀疏配置。加载完成后只读，读取方法并发安全。
type Store struct {
	mu    sync.RWMutex
	years map[int]*YearConfig
}

// Has 报告指定年份的配置是否存在。
func (s *Store) Has(year int) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.years[year]
	return ok
}

// Get 返回指定年份的配置，不存在时返回 nil。
func (s *Store) Get(year int) *YearConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.years[year]
}

// Years 返回已加载的年份列表，升序排列。
func (s *Store) Years() []int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	years := make([]int, 0, len(s.years))
	for y := range s.years {
		years = append(years, y)
	}
	sort.Ints(years)
	return years
}

// LoadDir 从目录加载所有形如 NNNN.toml 的年份配置文件。
//
// 忽略子目录、非 toml 文件与非四位数字年份命名的文件；目录中无任何年份文件时
// 返回空 Store 与 nil 错误（允许空目录）。加载完成后输出已加载年份列表日志。
func LoadDir(dir string) (*Store, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("读取目录 %s 失败: %w", dir, err)
	}

	s := &Store{years: make(map[int]*YearConfig)}
	for _, e := range entries {
		if !e.Type().IsRegular() || !yearFilePattern.MatchString(e.Name()) {
			continue
		}
		cfg, err := LoadYear(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		s.years[cfg.Year] = cfg
	}

	years := s.Years()
	names := make([]string, len(years))
	for i, y := range years {
		names[i] = strconv.Itoa(y)
	}
	log.Printf("goliday: 已从 %s 加载 %d 个年份配置：%v", dir, len(years), names)

	return s, nil
}
