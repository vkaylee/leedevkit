package ownership

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenLogCreatesBoundedPrivateLog(t *testing.T) {
	root := t.TempDir()
	log, err := OpenLog(root, "leedevkit-test-log", "run.log")
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	data := bytes.Repeat([]byte("x"), int(maxLogBytes)+100)
	if _, err := log.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := log.Close(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(root, ".test_logs", "leedevkit-test-log", "run.log"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != maxLogBytes {
		t.Fatalf("size = %d, want %d", info.Size(), maxLogBytes)
	}
}
