package ownership

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenLogRejectsInvalidPathsWithoutWriting(t *testing.T) {
	root := t.TempDir()
	for _, input := range []struct{ root, project, name string }{
		{"", "project", "log"}, {root, "", "log"}, {root, "project", ""},
		{root, "../outside", "log"}, {root, "project", "../outside"}, {root, ".", "log"},
	} {
		if log, err := OpenLog(input.root, input.project, input.name); err == nil {
			log.Close()
			t.Errorf("accepted %+v", input)
		}
	}
	if _, err := os.Lstat(filepath.Join(root, ".test_logs")); !os.IsNotExist(err) {
		t.Fatalf("invalid input created state: %v", err)
	}
}
