package ownership

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const maxLogBytes int64 = 4 << 20

func OpenLog(root, project, name string) (io.WriteCloser, error) {
	if root == "" || project == "" || name == "" {
		return nil, fmt.Errorf("log root, project, and name are required")
	}
	if strings.ContainsAny(project+name, `/\\`) || project == "." || name == "." || strings.Contains(project, "..") || strings.Contains(name, "..") {
		return nil, fmt.Errorf("invalid log path")
	}
	directory := filepath.Join(root, ".test_logs", project)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}
	file, err := os.OpenFile(filepath.Join(directory, name), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open log: %w", err)
	}
	return &boundedLog{file: file}, nil
}

type boundedLog struct {
	file  *os.File
	bytes int64
}

func (l *boundedLog) Write(data []byte) (int, error) {
	if l.bytes >= maxLogBytes {
		return len(data), nil
	}
	allowed := int64(len(data))
	if remaining := maxLogBytes - l.bytes; allowed > remaining {
		allowed = remaining
	}
	written, err := l.file.Write(data[:allowed])
	l.bytes += int64(written)
	if int64(written) < int64(len(data)) && err == nil {
		return len(data), nil
	}
	return written, err
}

func (l *boundedLog) Close() error { return l.file.Close() }
