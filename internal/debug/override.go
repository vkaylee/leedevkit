package debug

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func WriteOverride(environment Environment, networks []string, directory, image string) (string, error) {
	if len(networks) == 0 || image == "" {
		return "", fmt.Errorf("approved networks and debug image are required")
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", err
	}
	mounts := []string{}
	manifest := filepath.Clean(environment.SourcePath)
	if !inside(environment.ProjectRoot, manifest) {
		return "", fmt.Errorf("manifest must stay inside project root")
	}
	mounts = append(mounts, manifest+":/workspace/"+filepath.ToSlash(relative(environment.ProjectRoot, manifest))+":ro")
	for _, service := range environment.Services {
		for _, source := range service.LogFiles {
			path := filepath.Clean(filepath.Join(environment.ProjectRoot, source))
			info, err := os.Lstat(path)
			if err != nil || info.Mode()&os.ModeSymlink != 0 || !inside(environment.ProjectRoot, path) {
				return "", fmt.Errorf("approved log source unavailable: %s", source)
			}
			mounts = append(mounts, path+":/workspace/"+filepath.ToSlash(relative(environment.ProjectRoot, path))+":ro")
		}
	}
	networkMap := map[string]any{}
	networkKeys := []string{}
	for index, network := range networks {
		key := fmt.Sprintf("debug_net_%d", index)
		networkKeys = append(networkKeys, key)
		networkMap[key] = map[string]any{"external": true, "name": network}
	}
	document := map[string]any{
		"services": map[string]any{"leedevkit-debug": map[string]any{
			"image":        image,
			"build":        map[string]any{"context": environment.ProjectRoot, "dockerfile": "container/debug/Dockerfile"},
			"read_only":    true,
			"user":         "65532:65532",
			"cap_drop":     []string{"ALL"},
			"security_opt": []string{"no-new-privileges:true"},
			"pids_limit":   64,
			"mem_limit":    "256m",
			"tmpfs":        []string{"/tmp:rw,nosuid,nodev,noexec,size=16m"},
			"volumes":      mounts,
			"networks":     networkKeys,
			"working_dir":  "/workspace",
			"entrypoint":   []string{"/app/leedevkit-debug"},
			"stdin_open":   false,
			"tty":          false,
		}},
		"networks": networkMap,
	}
	path := filepath.Join(directory, "debug-compose.override.json")
	data, err := json.Marshal(document)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func inside(root, path string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	if err != nil {
		return false
	}
	return rel != ".." && !startsParent(rel)
}

func startsParent(path string) bool {
	return len(path) >= 3 && path[:3] == ".."+string(filepath.Separator)
}
func relative(root, path string) string {
	value, _ := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	return value
}
