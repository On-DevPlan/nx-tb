// Package protocol:Chrome Native Messaging 标准协议。
// 4 字节 Little-Endian 长度前缀 + UTF-8 JSON body,走 stdin/stdout。
// 纯 stdlib,无外部依赖(靠这一点 + -ldflags="-s -w" 把 exe 压到 ~4MB)。
package protocol

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

const maxMessageSize = 100 * 1024 * 1024 // 100MB 防御上限

// readExact 攒齐 n 字节;stdin EOF 时返回 io.EOF(调用方据此退出消息循环)。
func readExact(r *bufio.Reader, n int) ([]byte, error) {
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// ReadMessage 读一条消息;无法解析时返回 error。
func ReadMessage(reader *bufio.Reader) (map[string]interface{}, error) {
	lenBuf, err := readExact(reader, 4)
	if err != nil {
		return nil, err // io.EOF 表示管道关闭 → 上层正常退出
	}
	length := binary.LittleEndian.Uint32(lenBuf)
	if length > maxMessageSize {
		return nil, fmt.Errorf("message too large: %d", length)
	}
	if length == 0 {
		return nil, fmt.Errorf("empty message")
	}
	body, err := readExact(reader, int(length))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	var msg map[string]interface{}
	if err := json.Unmarshal(body, &msg); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	return msg, nil
}

// WriteMessage 写一条响应(长度前缀 + JSON)。
func WriteMessage(w io.Writer, resp map[string]interface{}) error {
	body, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	header := make([]byte, 4)
	binary.LittleEndian.PutUint32(header, uint32(len(body)))
	if _, err := w.Write(header); err != nil {
		return err
	}
	_, err = w.Write(body)
	return err
}

// NewReader 包装 stdin(带缓冲,减少小包 syscall)。
func NewReader(r io.Reader) *bufio.Reader {
	return bufio.NewReaderSize(r, 64*1024)
}
