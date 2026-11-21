package lifecycle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func ReadMetadata(root, project string) (BuildMetadata, error) {
	path := filepath.Join(root, ".test_logs", project, "build-metadata.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return BuildMetadata{}, nil
		}
		return BuildMetadata{}, err
	}
	var metadata BuildMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return BuildMetadata{}, fmt.Errorf("decode build metadata: %w", err)
	}
	return metadata, nil
}

func WriteMetadata(root, project string, metadata BuildMetadata) error {
	directory := filepath.Join(root, ".test_logs", project)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(directory, "build-metadata.json"), data, 0o600)
}
