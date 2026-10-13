// Package commands:所有 host 命令。每个文件的 init() 自注册 ——
// stdio host 与 CLI 两条 transport 共用同一张表。
package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"tabboard_host/internal/handler"
	"tabboard_host/internal/lake"
	"tabboard_host/internal/register"
)

const (
	// ExtensionID 来自扩展 manifest 的 key 字段(稳定 ID)
	ExtensionID = "6orv4jipupwr3nk75hnq2bosj63rwlus"
	Version     = "0.2.1"
)

// ---- 通用 helpers ----

// stableExeDir 是 host exe 的稳定安装目录(npm 包路径不稳定,注册前复制过来)。
func stableExeDir() string {
	return filepath.Join(lake.Dir(), "bin")
}

// StableExePath 返回稳定位置的 exe 路径。
func StableExePath() string {
	return filepath.Join(stableExeDir(), "tabboard-host.exe")
}

// installSelf 把当前 exe 复制到稳定目录(已存在且同尺寸则跳过),返回稳定路径。
func installSelf() (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	dst := StableExePath()
	if err := os.MkdirAll(stableExeDir(), 0755); err != nil {
		return "", err
	}
	srcInfo, err := os.Stat(self)
	if err != nil {
		return "", err
	}
	if dstInfo, err := os.Stat(dst); err == nil && dstInfo.Size() == srcInfo.Size() {
		return dst, nil // 已是最新
	}
	data, err := os.ReadFile(self)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(dst, data, 0755); err != nil {
		return "", err
	}
	return dst, nil
}

// writeManifest 写 host.manifest.json(合并已有 allowed_origins)。
func writeManifest(exePath string) (string, error) {
	manifestPath := filepath.Join(lake.Dir(), "host.manifest.json")

	origins := map[string]bool{"chrome-extension://" + ExtensionID + "/": true}
	// 保留已有白名单(dev 扩展 ID 并存)
	if raw, err := os.ReadFile(manifestPath); err == nil {
		var prev struct {
			AllowedOrigins []string `json:"allowed_origins"`
		}
		if json.Unmarshal(raw, &prev) == nil {
			for _, o := range prev.AllowedOrigins {
				origins[o] = true
			}
		}
	}
	originList := make([]string, 0, len(origins))
	for o := range origins {
		originList = append(originList, o)
	}

	manifest := map[string]interface{}{
		"name":            register.HostName,
		"description":     "TabBoard Native Host",
		"path":            exePath,
		"arguments":       []string{},
		"type":            "stdio",
		"allowed_origins": originList,
	}
	if err := os.MkdirAll(lake.Dir(), 0755); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(manifestPath, data, 0644); err != nil {
		return "", err
	}
	return manifestPath, nil
}

// ---- init ----

func init() {
	handler.Register("init", func(req map[string]interface{}) handler.Response {
		skipRegistry := handler.Bool(req, "skip-registry", false) || handler.Bool(req, "skipRegistry", false)

		steps := []map[string]interface{}{}

		// 1) 目录
		for _, d := range []string{lake.LakeDir(), lake.StateDir(), stableExeDir()} {
			if err := os.MkdirAll(d, 0755); err != nil {
				return handler.Response{Status: "error", Message: "创建目录失败: " + err.Error()}
			}
		}
		steps = append(steps, map[string]interface{}{"step": "mkdir", "ok": true})

		// 2) 复制 exe 到稳定目录
		exePath, err := installSelf()
		if err != nil {
			return handler.Response{Status: "error", Message: "安装 exe 失败: " + err.Error()}
		}
		steps = append(steps, map[string]interface{}{"step": "install-exe", "ok": true, "path": exePath})

		// 3) host.manifest.json
		manifestPath, err := writeManifest(exePath)
		if err != nil {
			return handler.Response{Status: "error", Message: "写 manifest 失败: " + err.Error()}
		}
		steps = append(steps, map[string]interface{}{"step": "manifest", "ok": true, "path": manifestPath})

		// 4) 注册表
		regOK := true
		regMsgs := []string{"已跳过(--skip-registry)"}
		if !skipRegistry {
			regOK, regMsgs = register.RegisterHost(manifestPath)
			steps = append(steps, map[string]interface{}{"step": "registry", "ok": regOK, "messages": regMsgs})
		} else {
			steps = append(steps, map[string]interface{}{"step": "registry", "ok": true, "skipped": true})
		}

		return handler.Response{Status: "ok", Data: map[string]interface{}{
			"tabboardDir":  lake.Dir(),
			"lakeDir":      lake.LakeDir(),
			"exePath":      exePath,
			"manifestPath": manifestPath,
			"extensionId":  ExtensionID,
			"hostName":     register.HostName,
			"registryOk":   regOK,
			"registry":     regMsgs,
			"steps":        steps,
			"next":         "初始化完成。在扩展里点「同步到本地」,然后运行 nx-tb tb list 查看。",
		}}
	})
}

// ---- backup ----

func init() {
	handler.Register("backup", func(req map[string]interface{}) handler.Response {
		var rawRecords []interface{}

		if recs := handler.Slice(req, "records"); recs != nil {
			rawRecords = recs
		} else if file := handler.Str(req, "file"); file != "" {
			data, err := os.ReadFile(file)
			if err != nil {
				return handler.Response{Status: "error", Message: "读文件失败: " + err.Error()}
			}
			var parsed []interface{}
			if err := json.Unmarshal(data, &parsed); err != nil {
				return handler.Response{Status: "error", Message: "--file 内容必须是 records 数组: " + err.Error()}
			}
			rawRecords = parsed
		} else {
			return handler.Response{Status: "error", Message: "缺少 records 或 --file 参数"}
		}

		records := make([]map[string]interface{}, 0, len(rawRecords))
		for _, r := range rawRecords {
			if m, ok := r.(map[string]interface{}); ok {
				records = append(records, m)
			}
		}

		wrote, err := lake.AppendRecords(records)
		if err != nil {
			return handler.Response{Status: "error", Message: err.Error()}
		}

		// 元信息
		kinds := []string{}
		seen := map[string]bool{}
		for _, r := range records {
			if k, ok := r["kind"].(string); ok && !seen[k] {
				seen[k] = true
				kinds = append(kinds, k)
			}
		}
		ts := time.Now().UTC().Format(time.RFC3339Nano)
		if err := os.MkdirAll(lake.StateDir(), 0755); err == nil {
			state, _ := json.MarshalIndent(map[string]interface{}{
				"ts": ts, "wrote": wrote, "kinds": kinds,
			}, "", "  ")
			os.WriteFile(filepath.Join(lake.StateDir(), "last_backup.json"), state, 0644)
		}

		return handler.Response{Status: "ok", Data: map[string]interface{}{
			"wrote": wrote, "kinds": kinds, "ts": ts, "lakeDir": lake.Dir(),
		}}
	})
}

// ---- list / show ----

func init() {
	handler.Register("list", func(req map[string]interface{}) handler.Response {
		kind := handler.Str(req, "kind")
		if kind == "" {
			kinds, err := lake.ListKinds()
			if err != nil {
				return handler.Response{Status: "error", Message: err.Error()}
			}
			return handler.Response{Status: "ok", Data: map[string]interface{}{
				"lakeDir": lake.LakeDir(), "kinds": kinds,
			}}
		}
		limit := handler.Num(req, "limit", 50)
		sinceMs := int64(handler.Num(req, "since", 0))
		records, err := lake.ReadRecords(kind, limit, sinceMs)
		if err != nil {
			return handler.Response{Status: "error", Message: err.Error()}
		}
		return handler.Response{Status: "ok", Data: map[string]interface{}{
			"lakeDir": lake.LakeDir(), "kind": kind, "count": len(records), "records": records,
		}}
	})

	handler.Register("show", func(req map[string]interface{}) handler.Response {
		kind := handler.Str(req, "kind")
		id := handler.Str(req, "id")
		if kind == "" || id == "" {
			return handler.Response{Status: "error", Message: "用法: show <kind> <id>"}
		}
		rec, err := lake.FindRecord(kind, id)
		if err != nil {
			return handler.Response{Status: "error", Message: err.Error()}
		}
		if rec == nil {
			return handler.Response{Status: "error", Message: fmt.Sprintf("%s 中找不到 id=%s", kind, id)}
		}
		return handler.Response{Status: "ok", Data: map[string]interface{}{"kind": kind, "record": rec}}
	})
}

// ---- doctor ----

func init() {
	handler.Register("doctor", func(req map[string]interface{}) handler.Response {
		checks := []map[string]interface{}{}

		// 1) exe 存在
		self, _ := os.Executable()
		checks = append(checks, map[string]interface{}{
			"name": "exe", "ok": self != "", "path": self,
		})

		// 2) 数据湖可写
		lakeOK := true
		lakeErr := ""
		if err := os.MkdirAll(lake.LakeDir(), 0755); err != nil {
			lakeOK, lakeErr = false, err.Error()
		} else {
			probe := filepath.Join(lake.LakeDir(), ".doctor-write-test")
			if err := os.WriteFile(probe, []byte("ok"), 0644); err != nil {
				lakeOK, lakeErr = false, err.Error()
			}
		}
		c := map[string]interface{}{"name": "lake-writable", "ok": lakeOK, "path": lake.LakeDir()}
		if lakeErr != "" {
			c["error"] = lakeErr
		}
		checks = append(checks, c)

		// 3) manifest
		manifestPath := filepath.Join(lake.Dir(), "host.manifest.json")
		mOK := false
		mDetail := ""
		if raw, err := os.ReadFile(manifestPath); err == nil {
			var m map[string]interface{}
			if json.Unmarshal(raw, &m) == nil {
				_, hasPath := m["path"]
				_, hasOrigins := m["allowed_origins"]
				mOK = hasPath && hasOrigins
				mDetail = fmt.Sprintf("path=%v", m["path"])
			} else {
				mDetail = "JSON 解析失败"
			}
		} else {
			mDetail = "不存在,运行 nx-tb tb init"
		}
		checks = append(checks, map[string]interface{}{
			"name": "manifest", "ok": mOK, "path": manifestPath, "detail": mDetail,
		})

		// 4) 注册表
		regOK, present := register.IsRegistered()
		checks = append(checks, map[string]interface{}{
			"name": "registry", "ok": regOK, "browsers": present,
		})

		// 5) 上次同步
		lastBackup := map[string]interface{}{"name": "last-backup", "ok": true}
		if raw, err := os.ReadFile(filepath.Join(lake.StateDir(), "last_backup.json")); err == nil {
			var lb map[string]interface{}
			if json.Unmarshal(raw, &lb) == nil {
				lastBackup["detail"] = lb
			}
		} else {
			lastBackup["detail"] = map[string]interface{}{"present": false}
		}
		checks = append(checks, lastBackup)

		allOK := true
		bad := []string{}
		for _, c := range checks {
			if okv, _ := c["ok"].(bool); !okv {
				allOK = false
				bad = append(bad, fmt.Sprintf("%v", c["name"]))
			}
		}
		summary := "所有检查通过。"
		if !allOK {
			summary = strings.Join(bad, ", ") + " 异常"
		}

		return handler.Response{Status: "ok", Data: map[string]interface{}{
			"version": Version, "tabboardDir": lake.Dir(), "allOk": allOK,
			"checks": checks, "summary": summary,
		}}
	})
}

// ---- version ----

func init() {
	handler.Register("version", func(req map[string]interface{}) handler.Response {
		return handler.Response{Status: "ok", Data: map[string]interface{}{
			"version": Version, "runtime": "go", "platform": "windows",
		}}
	})
}
