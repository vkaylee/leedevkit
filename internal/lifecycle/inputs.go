package lifecycle

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

func ReadBuildInputs(composeFile, dockerfile, contextRoot string) ([]byte, []byte, []byte, error) {
	compose, err := os.ReadFile(composeFile)
	if err != nil {
		return nil, nil, nil, err
	}
	docker, err := os.ReadFile(dockerfile)
	if err != nil {
		return nil, nil, nil, err
	}
	context, err := readContext(contextRoot)
	if err != nil {
		return nil, nil, nil, err
	}
	return compose, docker, context, nil
}

func readContext(root string) ([]byte, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return os.ReadFile(root)
	}
	var output bytes.Buffer
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if entry.IsDir() {
			if rel == ".git" || rel == ".leedevkit" || rel == ".test_logs" {
				return fs.SkipDir
			}
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(&output, "%s\x00", rel); err != nil {
			return err
		}
		_, err = output.Write(data)
		return err
	})
	return output.Bytes(), err
}
