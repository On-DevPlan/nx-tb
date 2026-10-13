// TabBoard native host —— 单 exe 双 transport:
//   - stdio host 模式:Chrome 拉起(无参或仅带 origin 参数 + stdin 是管道),
//     消息循环直到 stdin EOF;sendNativeMessage 单发 / connectNative 常驻都支持
//   - CLI 模式:用户在终端跑(有命令参数,或 stdin 是终端),
//     argv 解析 → 同一张命令表 → 文本/JSON 输出
//
// 编译(bun 96MB → Go 预计 3-4MB):
//   go build -ldflags="-s -w" -trimpath -o tabboard-host.exe .
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"tabboard_host/internal/commands"
	"tabboard_host/internal/handler"
	"tabboard_host/internal/protocol"
)

// decideMode 判定 transport。规则:
//  1. stdin 不是管道(终端)→ CLI
//  2. stdin 是管道且首参数是 chrome-extension:// → host(Chrome 会附 origin)
//  3. stdin 是管道且无参数 → host(Chrome 无附加参数的老/简场景)
//  4. 其他(stdin 是管道但有普通命令参数,如 `echo x | nx-tb tb backup`)→ CLI
func decideMode(args []string) string {
	fi, err := os.Stdin.Stat()
	stdinIsPipe := err == nil && (fi.Mode()&os.ModeCharDevice) == 0
	if !stdinIsPipe {
		return "cli"
	}
	if len(args) > 1 {
		if strings.HasPrefix(args[1], "chrome-extension://") {
			return "host"
		}
		return "cli"
	}
	return "host"
}

func main() {
	args := os.Args
	if decideMode(args) == "host" {
		runAsHost()
		return
	}
	runAsCLI(args[1:])
}

// ---- host transport ----

func runAsHost() {
	reader := protocol.NewReader(os.Stdin)
	for {
		req, err := protocol.ReadMessage(reader)
		if err != nil { // io.EOF / 管道关闭 → 正常退出
			return
		}
		cmd, _ := req["command"].(string)
		resp := handler.Handle(cmd, req)
		if err := protocol.WriteMessage(os.Stdout, respMap(resp)); err != nil {
			return // stdout 断了(Chrome 已走),无需继续
		}
	}
}

// respMap 把 Response 转回 map(协议层只认 map)。
func respMap(r handler.Response) map[string]interface{} {
	out := map[string]interface{}{"status": r.Status}
	if r.Data != nil {
		out["data"] = r.Data
	}
	if r.Message != "" {
		out["message"] = r.Message
	}
	return out
}

// ---- CLI transport ----

func runAsCLI(argv []string) {
	// 全局 flag 先摘
	jsonOut := false
	rest := []string{}
	for _, a := range argv {
		switch a {
		case "--json":
			jsonOut = true
		case "--help", "-h":
			printHelp()
			return
		case "--version", "-V":
			fmt.Println(commands.Version)
			return
		default:
			rest = append(rest, a)
		}
	}

	if len(rest) == 0 {
		printHelp()
		return
	}

	cmd := rest[0]
	req := parseFlags(rest[1:])

	resp := handler.Handle(cmd, req)
	if resp.Status == "error" {
		if jsonOut {
			b, _ := json.MarshalIndent(respMap(resp), "", "  ")
			fmt.Println(string(b))
		} else {
			fmt.Fprintln(os.Stderr, "错误: "+resp.Message)
		}
		os.Exit(1)
	}
	printResult(resp, jsonOut)
}

func printResult(resp handler.Response, jsonOut bool) {
	if jsonOut {
		b, _ := json.MarshalIndent(respMap(resp), "", "  ")
		fmt.Println(string(b))
		return
	}
	if resp.Data == nil {
		if resp.Message != "" {
			fmt.Println(resp.Message)
		}
		return
	}
	switch v := resp.Data.(type) {
	case string:
		fmt.Println(v)
	default:
		b, _ := json.MarshalIndent(resp.Data, "", "  ")
		fmt.Println(string(b))
	}
}

// parseFlags 极简 argv 解析:--key value / --key=value / --flag / 位置参数。
func parseFlags(args []string) map[string]interface{} {
	req := map[string]interface{}{}
	positionals := []string{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "--") {
			positionals = append(positionals, a)
			continue
		}
		body := a[2:]
		var key string
		var val interface{} = true
		if eq := strings.Index(body, "="); eq >= 0 {
			key = body[:eq]
			val = body[eq+1:]
		} else {
			key = body
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				val = args[i+1]
				i++
			}
		}
		// 数字自动转
		if s, ok := val.(string); ok {
			if n, err := strconv.Atoi(s); err == nil {
				val = float64(n) // 与 JSON 解码的数字类型对齐(handler.Num 读 float64)
			}
		}
		req[key] = val
	}
	if len(positionals) > 0 {
		// show <kind> <id> 这类位置参数
		if len(positionals) >= 1 {
			req["kind"] = positionals[0]
		}
		if len(positionals) >= 2 {
			req["id"] = positionals[1]
		}
		req["positionals"] = positionals
	}
	return req
}

func printHelp() {
	var b strings.Builder
	b.WriteString("tabboard-host · TabBoard native host + 数据湖\n\n")
	b.WriteString("用法:\n")
	b.WriteString("  tabboard-host <command> [--flags]\n")
	b.WriteString("  tabboard-host --json <command>    机器可读输出\n\n")
	b.WriteString("可用命令:\n")
	names := handler.List()
	writer := bufio.NewWriter(os.Stdout)
	for _, n := range names {
		b.WriteString("  " + n + "\n")
	}
	b.WriteString("\n数据湖: " + os.Getenv("TABBOARD_LAKE_DIR") + "\n")
	writer.WriteString(b.String())
	writer.Flush()
}
