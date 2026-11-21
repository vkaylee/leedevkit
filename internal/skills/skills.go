package skills

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

type Entry struct {
	Name    string
	URL     string
	Version string
}

type Manager struct {
	Root string
}

func (m *Manager) Run(args []string) error {
	if len(args) == 0 {
		return errors.New("skills action required")
	}
	if m.Root == "" {
		return errors.New("skills root required")
	}
	if err := os.MkdirAll(filepath.Join(m.Root, ".leedevkit", "skills.d"), 0o755); err != nil {
		return err
	}
	switch args[0] {
	case "list":
		return nil
	case "remove":
		if len(args) != 2 {
			return errors.New("skills remove requires name")
		}
		return m.remove(args[1])
	case "add":
		if len(args) != 4 || args[2] != "--version" {
			return errors.New("skills add requires URL and --version")
		}
		return m.add(args[1], args[3])
	case "install":
		return m.install()
	case "update":
		return m.update()
	default:
		return fmt.Errorf("unsupported skills action %q", args[0])
	}
}

func (m *Manager) add(rawURL, version string) error {
	if !safeURL(rawURL) {
		return errors.New("skill URL must be HTTP(S) or SSH git URL")
	}
	name := skillName(rawURL)
	return m.cloneAndActivate(Entry{Name: name, URL: rawURL, Version: version})
}

func (m *Manager) install() error {
	entries, err := loadConfigured(m.Root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := m.cloneAndActivate(entry); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) cloneAndActivate(entry Entry) error {
	if !safeName(entry.Name) || !safeURL(entry.URL) {
		return errors.New("invalid skill entry")
	}
	base := filepath.Join(m.Root, ".leedevkit", "skills.d")
	target := filepath.Join(base, entry.Name)
	if exists(target) {
		return fmt.Errorf("skill %q already exists", entry.Name)
	}
	stageRoot, err := os.MkdirTemp(base, ".skills-stage-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stageRoot)
	stage := filepath.Join(stageRoot, entry.Name)
	if err := runGit(stageRoot, "clone", "--depth", "1", "--branch", entry.Version, entry.URL, stage); err != nil {
		return err
	}
	if err := os.Rename(stage, target); err != nil {
		return err
	}
	if err := m.writeLock(); err != nil {
		_ = os.RemoveAll(target)
		return err
	}
	return nil
}

func (m *Manager) remove(name string) error {
	if name == "" || strings.ContainsAny(name, `/\\`) || name == "." || name == ".." {
		return errors.New("invalid skill name")
	}
	target := filepath.Join(m.Root, ".leedevkit", "skills.d", name)
	if !exists(target) {
		return fmt.Errorf("skill %q not found", name)
	}
	backup := filepath.Join(filepath.Dir(target), "."+name+".remove-backup")
	if err := os.Rename(target, backup); err != nil {
		return err
	}
	if err := m.writeLock(); err != nil {
		_ = os.Rename(backup, target)
		return err
	}
	return os.RemoveAll(backup)
}

func (m *Manager) update() error {
	base := filepath.Join(m.Root, ".leedevkit", "skills.d")
	entries, err := os.ReadDir(base)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") || !entry.IsDir() || !exists(filepath.Join(base, entry.Name(), ".git")) {
			continue
		}
		if err := runGit(filepath.Join(base, entry.Name()), "pull", "--ff-only"); err != nil {
			return err
		}
	}
	return m.writeLock()
}

func (m *Manager) writeLock() error {
	base := filepath.Join(m.Root, ".leedevkit", "skills.d")
	lock := map[string]string{}
	entries, err := os.ReadDir(base)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") || !entry.IsDir() || !exists(filepath.Join(base, entry.Name(), ".git")) {
			continue
		}
		output, err := gitOutput(filepath.Join(base, entry.Name()), "rev-parse", "HEAD")
		if err != nil || strings.TrimSpace(output) == "" {
			return fmt.Errorf("read skill %s revision", entry.Name())
		}
		lock[entry.Name()] = strings.TrimSpace(output)
	}
	data, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return err
	}
	target := filepath.Join(m.Root, "leedevkit.lock")
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, target)
}

func loadConfigured(root string) ([]Entry, error) {
	data, err := os.ReadFile(filepath.Join(root, "leedevkit.toml"))
	if err != nil {
		return nil, err
	}
	var raw struct {
		Addons struct {
			Skills []struct {
				URL     string `toml:"url"`
				Version string `toml:"version"`
			} `toml:"skills"`
		} `toml:"addons"`
	}
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	entries := make([]Entry, 0, len(raw.Addons.Skills))
	for _, item := range raw.Addons.Skills {
		version := item.Version
		if version == "" {
			version = "main"
		}
		entries = append(entries, Entry{Name: skillName(item.URL), URL: item.URL, Version: version})
	}
	return entries, nil
}

func runGit(dir string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	command.Stdin = strings.NewReader("")
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return nil
}

func gitOutput(dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	output, err := command.Output()
	return string(output), err
}

func safeURL(value string) bool {
	return strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "git@")
}

func safeName(value string) bool {
	return value != "" && value != "." && value != ".." && !strings.ContainsAny(value, `/\\`) && filepath.Base(value) == value
}
func skillName(value string) string {
	value = strings.TrimSuffix(strings.TrimRight(value, "/"), ".git")
	return filepath.Base(value)
}
func exists(path string) bool { _, err := os.Lstat(path); return err == nil }
