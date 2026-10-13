// Package lake:JSONL 数据湖读写 + 路径解析。
// append-only,无 compaction(单条 ~200B,10 年 ~5MB/文件)。
package lake

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// 环境变量可覆盖(测试隔离用):
//   TABBOARD_DIR      整个根目录(默认 ~/.tabboard)
//   TABBOARD_LAKE_DIR 只覆盖 lake 子目录
func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	if h := os.Getenv("USERPROFILE"); h != "" {
		return h
	}
	return "."
}

// Dir 返回 ~/.tabboard(TABBOARD_DIR 可覆盖)。
func Dir() string {
	if d := os.Getenv("TABBOARD_DIR"); d != "" {
		return d
	}
	return filepath.Join(homeDir(), ".tabboard")
}

// LakeDir 返回数据湖目录。
func LakeDir() string {
	if d := os.Getenv("TABBOARD_LAKE_DIR"); d != "" {
		return d
	}
	return filepath.Join(Dir(), "lake")
}

// StateDir 返回状态目录。
func StateDir() string {
	if d := os.Getenv("TABBOARD_STATE_DIR"); d != "" {
		return d
	}
	return filepath.Join(Dir(), "state")
}

// validKind 防路径穿越:kind 只允许字母/数字/下划线。
func validKind(kind string) bool {
	if kind == "" {
		return false
	}
	for i, r := range kind {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' || (i > 0 && r >= '0' && r <= '9')
		if !ok {
			return false
		}
	}
	return true
}

func pathForKind(kind string) (string, error) {
	if !validKind(kind) {
		return "", fmt.Errorf("非法 kind: %q(只允许字母/数字/下划线)", kind)
	}
	return filepath.Join(LakeDir(), kind+".jsonl"), nil
}

// AppendRecords 把 records 按 kind 分桶追加写;返回写入条数。
// records 元素形如 {"kind":"groups","data":{...}}。
func AppendRecords(records []map[string]interface{}) (int, error) {
	if len(records) == 0 {
		return 0, nil
	}
	if err := os.MkdirAll(LakeDir(), 0755); err != nil {
		return 0, err
	}

	now := time.Now().UnixMilli()
	buckets := map[string][]string{}
	order := []string{} // 保持首次出现顺序,便于测试稳定
	for _, r := range records {
		kind, _ := r["kind"].(string)
		data, hasData := r["data"]
		if kind == "" || !hasData {
			return 0, fmt.Errorf("每条 record 必须有 kind:string 和 data")
		}
		if !validKind(kind) {
			return 0, fmt.Errorf("非法 kind: %q", kind)
		}
		obj, ok := data.(map[string]interface{})
		if !ok {
			// 非对象(字符串/数组)包一层,保持每行 JSON object
			obj = map[string]interface{}{"value": data}
		}
		obj["_ts"] = now
		line, err := json.Marshal(obj)
		if err != nil {
			return 0, err
		}
		if _, seen := buckets[kind]; !seen {
			order = append(order, kind)
		}
		buckets[kind] = append(buckets[kind], string(line))
	}

	wrote := 0
	for _, kind := range order {
		path, err := pathForKind(kind)
		if err != nil {
			return wrote, err
		}
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return wrote, err
		}
		payload := strings.Join(buckets[kind], "\n") + "\n"
		if _, err := f.WriteString(payload); err != nil {
			f.Close()
			return wrote, err
		}
		f.Close()
		wrote += len(buckets[kind])
	}
	return wrote, nil
}

// ReadRecords 读一个 kind 的全部记录(带 limit / 时间过滤)。
func ReadRecords(kind string, limit int, sinceMs int64) ([]map[string]interface{}, error) {
	path, err := pathForKind(kind)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []map[string]interface{}{}, nil // 空数据湖不是错误
		}
		return nil, err
	}
	defer f.Close()

	out := []map[string]interface{}{}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 16*1024*1024) // 单行上限 16MB
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			continue // 跳过坏行
		}
		if sinceMs > 0 {
			if ts, ok := obj["_ts"].(float64); !ok || int64(ts) < sinceMs {
				continue
			}
		}
		out = append(out, obj)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, scanner.Err()
}

// KindStat 描述一个 kind 的文件信息。
type KindStat struct {
	Kind    string `json:"kind"`
	Bytes   int64  `json:"bytes"`
	ModTime int64  `json:"mtime"`
}

// ListKinds 扫 lake 目录下的 .jsonl 文件。
func ListKinds() ([]KindStat, error) {
	entries, err := os.ReadDir(LakeDir())
	if err != nil {
		if os.IsNotExist(err) {
			return []KindStat{}, nil
		}
		return nil, err
	}
	out := []KindStat{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, KindStat{
			Kind:    strings.TrimSuffix(e.Name(), ".jsonl"),
			Bytes:   info.Size(),
			ModTime: info.ModTime().UnixMilli(),
		})
	}
	return out, nil
}

// FindRecord 按 id 查单条。
func FindRecord(kind, id string) (map[string]interface{}, error) {
	records, err := ReadRecords(kind, 0, 0)
	if err != nil {
		return nil, err
	}
	for _, r := range records {
		if rid, _ := r["id"].(string); rid == id {
			return r, nil
		}
	}
	return nil, nil
}
