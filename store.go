package goliday

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strconv"
)

// Store 持有多个年份的稀疏配置。由 LoadDir 构建后不可变（无任何导出
// 或未导出的修改途径），并发读取天然安全且无锁开销。
type Store struct {
	years map[int]*YearConfig
}

// Has 报告指定年份的配置是否存在。
func (s *Store) Has(year int) bool {
	_, ok := s.years[year]
	return ok
}

// Get 返回指定年份的配置，不存在时返回 nil。
func (s *Store) Get(year int) *YearConfig {
	return s.years[year]
}

// Years 返回已加载的年份列表，升序排列。
func (s *Store) Years() []int {
	years := make([]int, 0, len(s.years))
	for y := range s.years {
		years = append(years, y)
	}
	slices.Sort(years)
	return years
}

// LoadDir 从目录加载所有形如 NNNN.toml 的年份配置文件。
//
// 忽略子目录、非 toml 文件与非四位数字年份命名的文件；目录中无任何年份文件时
// 返回空 Store 与 nil 错误（允许空目录）。返回的 Store 不可变，可被多个
// goroutine 并发读取。
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
