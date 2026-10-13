// Package register:Windows 注册表写入(Chrome + Edge,HKCU 无需管理员)。
// 通过 PowerShell 操作(纯 stdlib,不引 golang.org/x/sys,保持 exe 小体积)。
package register

import (
	"fmt"
	"os/exec"
	"strings"
)

// HostName 是 Chrome native messaging 的 host 标识。
const HostName = "com.tabboard.tb"

var browserKeys = []struct{ Vendor, Path string }{
	{"Chrome", `Software\Google\Chrome\NativeMessagingHosts`},
	{"Edge", `Software\Microsoft\Edge\NativeMessagingHosts`},
}

// runPowerShell 执行一段 PowerShell;失败返回 (false, stderr)。
func runPowerShell(script string) (bool, string) {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, strings.TrimSpace(string(out))
	}
	return true, strings.TrimSpace(string(out))
}

// RegisterHost 把 hostName → manifestPath 写进 Chrome + Edge 注册表。
// ⚠️ PowerShell 一行多语句必须用 "; " 分隔(空格连接会 parser error)。
func RegisterHost(manifestPath string) (bool, []string) {
	msgs := []string{}
	allOK := true
	// PowerShell 单引号字符串里,单引号自身用两个单引号转义
	psPath := strings.ReplaceAll(manifestPath, "'", "''")

	for _, b := range browserKeys {
		regKey := `HKCU:\` + b.Path + `\` + HostName
		script := strings.Join([]string{
			`$ErrorActionPreference = 'Stop'`,
			`try {`,
			fmt.Sprintf(`  New-Item -Path '%s' -Force | Out-Null`, regKey),
			fmt.Sprintf(`  Set-ItemProperty -LiteralPath '%s' -Name '(default)' -Value '%s'`, regKey, psPath),
			fmt.Sprintf(`  Write-Output 'ok:%s'`, b.Vendor),
			`} catch {`,
			`  Write-Error $_.Exception.Message`,
			`  exit 1`,
			`}`,
		}, "; ")

		ok, out := runPowerShell(script)
		if ok {
			msgs = append(msgs, b.Vendor+": 已注册")
		} else {
			allOK = false
			msgs = append(msgs, b.Vendor+": 失败 — "+out)
		}
	}
	return allOK, msgs
}

// UnregisterHost 删除注册表项。
func UnregisterHost() (bool, []string) {
	msgs := []string{}
	for _, b := range browserKeys {
		regKey := `HKCU:\` + b.Path + `\` + HostName
		script := fmt.Sprintf(`Remove-Item -LiteralPath '%s' -ErrorAction SilentlyContinue; exit 0`, regKey)
		ok, out := runPowerShell(script)
		if ok {
			msgs = append(msgs, b.Vendor+": 已移除")
		} else {
			msgs = append(msgs, b.Vendor+": 失败 — "+out)
		}
	}
	return true, msgs
}

// IsRegistered 查询注册状态(Chrome + Edge)。
func IsRegistered() (bool, []string) {
	present := []string{}
	for _, b := range browserKeys {
		regKey := `HKCU:\` + b.Path + `\` + HostName
		script := fmt.Sprintf(`if (Test-Path -LiteralPath '%s') { exit 0 } else { exit 1 }`, regKey)
		if ok, _ := runPowerShell(script); ok {
			present = append(present, b.Vendor)
		}
	}
	return len(present) > 0, present
}
