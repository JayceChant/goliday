// Command goliday-tool 提供年份配置的校验（validate）与生成（gen）子命令。
package main

import (
	"fmt"
	"os"

	"github.com/JayceChant/goliday"
)

const usageText = `用法：
  goliday-tool validate <file...>
      逐个校验年份配置文件，打印 "OK <path>" 或 "FAIL <path>: <原因>"；
      全部通过退出 0，任一失败退出 1。
  goliday-tool gen -year <年> -out <文件> [-file <公告文本>]
      解析中文官方放假公告，生成稀疏 TOML 配置草稿（需人工核对）；
      -file 缺省或为 "-" 时从 stdin 读取公告文本；-out 为 "-" 时输出到 stdout。

退出码：0 成功；1 校验失败或生成失败；2 用法错误。`

func usage() {
	fmt.Fprintln(os.Stderr, usageText)
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "validate":
		os.Exit(runValidate(os.Args[2:]))
	case "gen":
		os.Exit(runGen(os.Args[2:]))
	default:
		usage()
		os.Exit(2)
	}
}

// runValidate 对每个文件调用 goliday.LoadYear 并打印结果，返回进程退出码。
func runValidate(paths []string) int {
	if len(paths) == 0 {
		usage()
		return 2
	}
	rc := 0
	for _, path := range paths {
		if _, err := goliday.LoadYear(path); err != nil {
			// LoadYear 的错误信息以文件路径开头，恰好构成 "FAIL <path>: <原因>"。
			fmt.Printf("FAIL %v\n", err)
			rc = 1
		} else {
			fmt.Printf("OK %s\n", path)
		}
	}
	return rc
}
