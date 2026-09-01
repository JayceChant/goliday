// 黑盒测试（package goliday_test）：仅经导出 API（LoadDir/Store）验证
// 目录加载的对外契约：仅加载 NNNN.toml、忽略杂项、空目录与错误场景。
package goliday_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JayceChant/goliday"
)

func TestStoreTempDirScenario(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"2025.toml": minimal2025TOML,
		"2026.toml": minimal2026TOML,
		"README.md": "# 说明文档\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("写入 %s 失败: %v", name, err)
		}
	}
	other := filepath.Join(dir, "other")
	if err := os.Mkdir(other, 0o755); err != nil {
		t.Fatalf("创建子目录失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(other, "2027.toml"), []byte(minimal2027TOML), 0o644); err != nil {
		t.Fatalf("写入子目录 2027.toml 失败: %v", err)
	}

	s := mustLoadDir(t, dir)
	years := s.Years()
	if len(years) != 2 || years[0] != 2025 || years[1] != 2026 {
		t.Fatalf("Years() = %v，期望 [2025 2026]（子目录与非年份文件不应被加载）", years)
	}
	if s.Has(2027) {
		t.Error("Has(2027) 应为 false，子目录中的 2027.toml 不应被加载")
	}
	if c := s.Get(2026); c == nil || c.Year != 2026 {
		t.Errorf("Get(2026) = %+v，期望 Year = 2026 的配置", c)
	}
}

func TestStoreEmptyDir(t *testing.T) {
	s := mustLoadDir(t, t.TempDir())
	if years := s.Years(); len(years) != 0 {
		t.Errorf("空目录 Years() = %v，期望为空", years)
	}
	if s.Has(2025) {
		t.Error("空目录 Has(2025) 应为 false")
	}
	if c := s.Get(2025); c != nil {
		t.Errorf("空目录 Get(2025) = %+v，期望 nil", c)
	}
}

// TestLoadDirInvalid 逐一验证 invalid 样例经 LoadDir 报错。
// 样例原始文件名非 NNNN.toml（会被 LoadDir 忽略），故复制为 2026.toml 后加载。
func TestLoadDirInvalid(t *testing.T) {
	for name, keywords := range invalidFiles {
		name, keywords := name, keywords
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(filepath.Join("testdata", "invalid", name))
			if err != nil {
				t.Fatalf("读取样例失败: %v", err)
			}
			dir := t.TempDir()
			target := filepath.Join(dir, "2026.toml")
			if err := os.WriteFile(target, src, 0o644); err != nil {
				t.Fatalf("写入 %s 失败: %v", target, err)
			}

			if _, err := goliday.LoadDir(dir); err != nil {
				msg := err.Error()
				// 错误信息内嵌的即传入的原始路径，直接断言（兼容平台路径分隔符差异）。
				if !strings.Contains(msg, target) {
					t.Errorf("错误信息 %q 未包含文件名 %q", msg, target)
				}
				for _, kw := range keywords {
					if !strings.Contains(msg, kw) {
						t.Errorf("错误信息 %q 未包含关键词 %q", msg, kw)
					}
				}
			} else {
				t.Fatalf("LoadDir(%q) 未报错", dir)
			}
		})
	}
}

// TestLoadDirInvalidDirIgnored 非年份命名的 invalid 样例在 LoadDir 中应被忽略而非报错。
func TestLoadDirInvalidDirIgnored(t *testing.T) {
	s := mustLoadDir(t, filepath.Join("testdata", "invalid"))
	if years := s.Years(); len(years) != 0 {
		t.Errorf("Years() = %v，期望为空（无 NNNN.toml 文件）", years)
	}
}

func TestLoadDirMissingDir(t *testing.T) {
	if _, err := goliday.LoadDir(filepath.Join(t.TempDir(), "no-such-dir")); err == nil {
		t.Error("不存在的目录 LoadDir 未报错")
	}
}
