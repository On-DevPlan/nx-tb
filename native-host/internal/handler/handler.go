// Package handler:命令注册表。stdio host 和 CLI 两条 transport 共用一个表。
package handler

import (
	"fmt"
	"sort"
	"sync"
)

// Response 是每个命令统一的返回形态(与 JS 版契约一致)。
type Response struct {
	Status  string      `json:"status"`            // "ok" | "error"
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
}

// Handler 处理一条命令;req 是原始 JSON 对象(值为 interface{},按需断言)。
type Handler func(req map[string]interface{}) Response

var (
	mu       sync.RWMutex
	handlers = map[string]Handler{}
	frozen   bool
)

// Register 由各命令文件的 init() 调用。
func Register(name string, h Handler) {
	mu.Lock()
	defer mu.Unlock()
	if frozen {
		panic(fmt.Sprintf("registry 已冻结,无法注册 %q", name))
	}
	if _, dup := handlers[name]; dup {
		panic(fmt.Sprintf("重复注册命令: %q", name))
	}
	handlers[name] = h
}

// List 返回已注册命令名(排序后,便于 help 输出稳定)。
func List() []string {
	mu.Lock()
	frozen = true
	mu.Unlock()
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(handlers))
	for k := range handlers {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// Handle 分发一条命令;未注册 / panic 都转成 error Response。
func Handle(name string, req map[string]interface{}) (resp Response) {
	mu.Lock()
	frozen = true
	mu.Unlock()

	mu.RLock()
	h, ok := handlers[name]
	mu.RUnlock()
	if !ok {
		return Response{Status: "error", Message: fmt.Sprintf("未知命令: %q。运行 'nx-tb tb help' 查看可用命令。", name)}
	}

	defer func() {
		if r := recover(); r != nil {
			resp = Response{Status: "error", Message: fmt.Sprintf("handler panic: %v", r)}
		}
	}()
	return h(req)
}

// ---- req 取值 helpers(JSON 数字是 float64,统一在 Go 侧收敛) ----

// Str 取字符串字段,缺失返回 ""。
func Str(req map[string]interface{}, key string) string {
	if v, ok := req[key].(string); ok {
		return v
	}
	return ""
}

// Num 取数字字段(float64 → int),缺失返回 def。
func Num(req map[string]interface{}, key string, def int) int {
	if v, ok := req[key].(float64); ok {
		return int(v)
	}
	return def
}

// Bool 取布尔字段,缺失返回 def。
func Bool(req map[string]interface{}, key string, def bool) bool {
	if v, ok := req[key].(bool); ok {
		return v
	}
	return def
}

// Slice 取数组字段。
func Slice(req map[string]interface{}, key string) []interface{} {
	if v, ok := req[key].([]interface{}); ok {
		return v
	}
	return nil
}
