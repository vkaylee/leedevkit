package runtime

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var payloadDirs = []string{"scripts", "templates", "bin", ".agent", "container", "cmd", "internal"}
var payloadFiles = []string{"VERSION", "go.mod", "go.sum", "devkit.manifest.json", "sbom.cdx.json"}

func Init(root, source string, _ bool) error {
	root, source = filepath.Clean(root), filepath.Clean(source)
	rulesSource := filepath.Join(source, ".agent", "rules")
	if info, err := os.Stat(rulesSource); err != nil || !info.IsDir() {
		return fmt.Errorf("release rules asset unavailable: %s", rulesSource)
	}
	configPath := filepath.Join(root, "leedevkit.toml")
	if _, err := os.Lstat(configPath); errors.Is(err, os.ErrNotExist) {
		if err := createProjectConfig(configPath, filepath.Join(source, "templates"), root); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return syncRules(root, rulesSource)
}

func createProjectConfig(path, templates, projectRoot string) error {
	name := "leedevkit.default.toml"
	if _, err := os.Stat(filepath.Join(projectRoot, "go.mod")); err == nil {
		name = "leedevkit.go.toml"
	} else if _, err := os.Stat(filepath.Join(projectRoot, "Cargo.toml")); err == nil {
		name = "leedevkit.rust.toml"
	}
	data, err := os.ReadFile(filepath.Join(templates, name))
	if err != nil {
		return fmt.Errorf("read init template: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("create project config: %w", err)
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return err
	}
	return nil
}

func readVersion(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("source VERSION missing: %w", err)
	}
	return strings.TrimPrefix(strings.TrimSpace(string(data)), "v"), nil
}

func copyPayload(source, target string) error {
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	for _, name := range payloadDirs {
		if err := copyPath(filepath.Join(source, name), filepath.Join(target, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	for _, name := range payloadFiles {
		if err := copyPath(filepath.Join(source, name), filepath.Join(target, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func copyPath(source, target string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing symlink in runtime payload: %s", source)
	}
	if info.IsDir() {
		if err := os.MkdirAll(target, info.Mode().Perm()); err != nil {
			return err
		}
		entries, err := os.ReadDir(source)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := copyPath(filepath.Join(source, entry.Name()), filepath.Join(target, entry.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		return err
	}
	return output.Close()
}

func syncRules(root, source string) error {
	project := filepath.Join(root, ".agent", "rules")
	if info, err := os.Lstat(project); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to modify symlinked rules directory: %s", project)
	}
	if err := os.MkdirAll(project, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(source)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		target := filepath.Join(project, entry.Name())
		if _, err := os.Lstat(target); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := copyPath(filepath.Join(source, entry.Name()), target); err != nil {
			return err
		}
	}
	return nil
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func isWithin(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}
