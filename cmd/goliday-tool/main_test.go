// 白盒测试（package main）：覆盖 dispatch（main 的可测形态）的子命令
// 分派与用法错误退出码。
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDispatchUsageErrors(t *testing.T) {
	// 用法错误统一退出码 2（usage 输出到 stderr，不在此断言内容）。
	for _, args := range [][]string{
		nil,          // 无子命令
		{"bogus"},    // 未知子命令
		{"validate"}, // validate 缺文件参数
		{"gen"},      // gen 缺 -year/-out
		{"-h"},       // 无此标志（不是子命令）
	} {
		if rc := dispatch(args); rc != 2 {
			t.Errorf("dispatch(%q) = %d, want 2", args, rc)
		}
	}
}

func TestDispatchValidateRoutes(t *testing.T) {
	// 复用仓库 invalid 样例（文件名年份不匹配会被 LoadYear 拒绝），
	// 复制为临时 2026.toml 使内容违规成为失败原因；分派语义：走
	// validate 路径，非法文件退出码 1，合法文件退出码 0。
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "invalid", "invalid_date.toml"))
	if err != nil {
		t.Fatalf("读取 invalid 样例失败: %v", err)
	}
	bad := filepath.Join(t.TempDir(), "2026.toml")
	if err := os.WriteFile(bad, data, 0o600); err != nil {
		t.Fatalf("写临时配置失败: %v", err)
	}
	if rc := dispatch([]string{"validate", bad}); rc != 1 {
		t.Errorf("dispatch(validate 非法文件) = %d, want 1", rc)
	}
	if rc := dispatch([]string{"validate", "../../testdata/2026.toml"}); rc != 0 {
		t.Errorf("dispatch(validate 合法文件) = %d, want 0", rc)
	}
}

func TestDispatchGenRoutes(t *testing.T) {
	// gen 路径：合法参数流经 runGen（输出到 stdout），退出码由生成结果决定。
	if rc := dispatch([]string{"gen", "-year", "2026", "-file", "-", "-out", "-"}); rc == 2 {
		t.Error("dispatch(gen …) 不应返回用法错误 2（stdin 为空公告，期望生成成功或自检失败）")
	}
}
