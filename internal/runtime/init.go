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
var payloadFiles = []string{"VERSION", "go.mod", "go.sum", "devkit.manifest.json"}

func Init(root, source string, force bool) error {
	root, source = filepath.Clean(root), filepath.Clean(source)
	target := filepath.Join(root, ".leedevkit")
	configPath := filepath.Join(root, "leedevkit.toml")
	originalConfig, configErr := os.ReadFile(configPath)
	configExisted := configErr == nil
	configChanged := false
	succeeded := false
	defer func() {
		if succeeded || !configChanged {
			return
		}
		if configExisted {
			_ = os.WriteFile(configPath, originalConfig, 0o644)
		} else {
			_ = os.Remove(configPath)
		}
	}()
	configured, err := configuredVersion(configPath)
	if errors.Is(err, os.ErrNotExist) {
		actual, versionErr := readVersion(filepath.Join(source, "VERSION"))
		if versionErr != nil {
			return versionErr
		}
		if err := createPinnedConfig(configPath, filepath.Join(source, "templates"), actual); err != nil {
			return err
		}
		configChanged = true
		configured = actual
	} else if err != nil {
		return err
	}
	actual, err := readVersion(filepath.Join(source, "VERSION"))
	if err != nil {
		return err
	}
	if configured == "latest" {
		data, readErr := os.ReadFile(configPath)
		if readErr != nil {
			return readErr
		}
		updated := strings.Replace(string(data), "version = \"latest\"", "version = \""+actual+"\"", 1)
		if err := os.WriteFile(configPath, []byte(updated), 0o644); err != nil {
			return fmt.Errorf("pin project config: %w", err)
		}
		configChanged = true
		configured = actual
	}
	if configured == "" {
		return errors.New("[devkit].version must be pinned before Go runtime init")
	}
	if actual != configured {
		return fmt.Errorf("local devkit version %q does not match requested %q", actual, configured)
	}
	if source == target || isWithin(target, source) {
		return errors.New("source and runtime paths must not overlap")
	}
	if info, err := os.Lstat(target); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to replace symlinked runtime: %s", target)
	}
	_ = force
	stage, err := os.MkdirTemp(root, ".leedevkit-init-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	staged := filepath.Join(stage, "runtime")
	if err := copyPayload(source, staged); err != nil {
		return err
	}
	if err := syncRules(root, filepath.Join(staged, ".agent", "rules")); err != nil {
		return err
	}
	backup := filepath.Join(stage, "previous")
	if exists(target) {
		if err := os.Rename(target, backup); err != nil {
			return fmt.Errorf("stage existing runtime: %w", err)
		}
	}
	activated := false
	rollback := func() {
		if activated {
			_ = os.RemoveAll(target)
		}
		if exists(backup) {
			_ = os.Rename(backup, target)
		}
	}
	if err := os.Rename(staged, target); err != nil {
		rollback()
		return fmt.Errorf("activate runtime: %w", err)
	}
	activated = true
	for _, name := range []string{"skills.d", ".venv"} {
		old := filepath.Join(backup, name)
		if !exists(old) {
			continue
		}
		newPath := filepath.Join(target, name)
		_ = os.RemoveAll(newPath)
		if err := os.Rename(old, newPath); err != nil {
			rollback()
			return fmt.Errorf("preserve runtime state %s: %w", name, err)
		}
	}
	if exists(backup) {
		_ = os.RemoveAll(backup)
	}
	if err := syncRules(root, filepath.Join(target, ".agent", "rules")); err != nil {
		return err
	}
	succeeded = true
	return nil
}

func createPinnedConfig(path, templates, version string) error {
	name := "leedevkit.default.toml"
	if _, err := os.Stat(filepath.Join(templates, "leedevkit.go.toml")); err == nil {
		name = "leedevkit.go.toml"
	} else if _, err := os.Stat(filepath.Join(templates, "leedevkit.rust.toml")); err == nil {
		name = "leedevkit.rust.toml"
	}
	data, err := os.ReadFile(filepath.Join(templates, name))
	if err != nil {
		return fmt.Errorf("read init template: %w", err)
	}
	content := strings.Replace(string(data), "version = \"latest\"", "version = \""+version+"\"", 1)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write project config: %w", err)
	}
	return nil
}

func configuredVersion(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read project config: %w", err)
	}
	lines := strings.Split(string(data), "\n")
	inDevkit := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") {
			inDevkit = line == "[devkit]"
			continue
		}
		if !inDevkit || !strings.HasPrefix(line, "version") {
			continue
		}
		_, value, ok := strings.Cut(line, "=")
		if ok {
			return strings.Trim(strings.TrimSpace(value), "\"'"), nil
		}
	}
	return "", errors.New("project config has no [devkit].version")
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
