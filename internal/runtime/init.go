package runtime

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const (
	ClaudeMarkerStart       = "<!-- leedevkit:start -->"
	ClaudeMarkerLegacyBegin = "<!-- leedevkit:begin -->"
	ClaudeMarkerEnd         = "<!-- leedevkit:end -->"
	claudeHeading           = "## LeeDevKit base context"
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
	if err := syncRules(root, rulesSource); err != nil {
		return err
	}
	return syncClaudeContextIfEnabled(root)
}

func syncClaudeContextIfEnabled(root string) error {
	claudePath := filepath.Join(root, "CLAUDE.md")
	if !claudeHarnessEnabled(root) && !exists(claudePath) {
		return nil
	}
	return SyncClaudeMD(root)
}

func claudeHarnessEnabled(root string) bool {
	data, err := os.ReadFile(filepath.Join(root, "leedevkit.toml"))
	if err != nil {
		return true
	}
	var raw struct {
		AI struct {
			Harnesses []string `toml:"harnesses"`
		} `toml:"ai"`
	}
	if err := toml.Unmarshal(data, &raw); err != nil || len(raw.AI.Harnesses) == 0 {
		return true
	}
	for _, h := range raw.AI.Harnesses {
		if strings.EqualFold(strings.TrimSpace(h), "claude") {
			return true
		}
	}
	return false
}

func IsDevkitSourceRepo(root string) bool {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil || !strings.HasPrefix(string(data), "module github.com/vkaylee/leedevkit\n") {
		return false
	}
	return exists(filepath.Join(root, "templates", "CLAUDE.base.md"))
}

func managedClaudeBlock(isDevkitRepo bool) string {
	if isDevkitRepo {
		return strings.Join([]string{
			ClaudeMarkerStart,
			claudeHeading,
			"",
			"This repository uses LeeDevKit. Before making changes, read and apply:",
			"`templates/CLAUDE.base.md`.",
			"",
			"Project policy lives in `.agent/`; devkit sources live at:",
			"- Built-in skills: `.agent/skills/*/SKILL.md`",
			"- Community skills: `skills.d/*/SKILL.md`",
			"- Built-in agents: `.agent/agents/*.md`",
			"- Project agents: `.agent/agents/*.md`",
			"",
			"Use globally installed `leedevkit --help` for available project commands. This repository's `./leedevkit` launcher requires a local Go binary and is not the default entry point; use it only when that binary exists and is executable.",
			ClaudeMarkerEnd,
		}, "\n")
	}
	return strings.Join([]string{
		ClaudeMarkerStart,
		claudeHeading,
		"",
		"This repository uses LeeDevKit. Before making changes, read and apply:",
		"`~/.leedevkit/current/templates/CLAUDE.base.md`.",
		"",
		"Project policy lives in `.agent/`; devkit sources live at:",
		"- Built-in skills: `~/.leedevkit/current/.agent/skills/*/SKILL.md`",
		"- Community skills: `.agent/skills.d/*/SKILL.md`",
		"- Built-in agents: `~/.leedevkit/current/.agent/agents/*.md`",
		"- Project agents: `.agent/agents/*.md`",
		"",
		"Use the globally installed `leedevkit --help` for available project commands.",
		ClaudeMarkerEnd,
	}, "\n")
}

// HasClaudeMDIssue reports whether CLAUDE.md contains legacy markers, unclosed markers, or duplicate LeeDevKit base blocks.
func HasClaudeMDIssue(content string) bool {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	if strings.Contains(normalized, ClaudeMarkerLegacyBegin) {
		return true
	}
	startCount := strings.Count(normalized, ClaudeMarkerStart)
	endCount := strings.Count(normalized, ClaudeMarkerEnd)
	if startCount > 1 || endCount > 1 || startCount != endCount {
		return true
	}
	headingCount := 0
	for _, line := range strings.Split(normalized, "\n") {
		if strings.TrimSpace(line) == claudeHeading {
			headingCount++
		}
	}
	if headingCount > 1 || (headingCount == 1 && startCount == 0) {
		return true
	}
	return false
}

// RenderClaudeMD idempotently injects or replaces the managed <!-- leedevkit:start -->...<!-- leedevkit:end --> block
// and strips duplicate legacy blocks while preserving user content.
func RenderClaudeMD(existing string, isDevkitRepo bool) string {
	block := managedClaudeBlock(isDevkitRepo)
	normalized := strings.ReplaceAll(existing, "\r\n", "\n")
	if strings.TrimSpace(normalized) == "" {
		return block + "\n"
	}
	lines := strings.Split(normalized, "\n")
	var out []string
	inserted := false
	for i := 0; i < len(lines); {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == ClaudeMarkerStart || trimmed == ClaudeMarkerLegacyBegin {
			end := i + 1
			for end < len(lines) && strings.TrimSpace(lines[end]) != ClaudeMarkerEnd {
				if strings.HasPrefix(strings.TrimSpace(lines[end]), "## ") && strings.TrimSpace(lines[end]) != claudeHeading {
					break
				}
				end++
			}
			if end < len(lines) && strings.TrimSpace(lines[end]) == ClaudeMarkerEnd {
				end++
			}
			if !inserted {
				out = append(out, block)
				inserted = true
			}
			i = end
			continue
		}
		if trimmed == claudeHeading {
			end := i + 1
			for end < len(lines) && isLegacyClaudeBoilerplateLine(lines[end]) {
				end++
			}
			if !inserted {
				out = append(out, block)
				inserted = true
			}
			i = end
			continue
		}
		out = append(out, lines[i])
		i++
	}
	joined := collapseExtraBlankLines(strings.Join(out, "\n"))
	joined = strings.TrimSpace(joined)
	if !inserted {
		if joined == "" {
			return block + "\n"
		}
		return block + "\n\n" + joined + "\n"
	}
	return joined + "\n"
}

func isLegacyClaudeBoilerplateLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return true
	}
	if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "<!-- leedevkit:") {
		return false
	}
	for _, prefix := range []string{
		"This repository uses LeeDevKit.",
		"`templates/CLAUDE.base.md`",
		"`~/.leedevkit/",
		"`.leedevkit/",
		"Project policy lives in `.agent/`",
		"- Built-in skills:",
		"- Community skills:",
		"- Built-in agents:",
		"- Project agents:",
		"Use globally installed `leedevkit",
		"Use the globally installed `leedevkit",
		"Use `./leedevkit",
	} {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	return false
}

func collapseExtraBlankLines(text string) string {
	lines := strings.Split(text, "\n")
	var out []string
	blankRun := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			blankRun++
			if blankRun > 1 {
				continue
			}
			out = append(out, "")
			continue
		}
		blankRun = 0
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// SyncClaudeMD updates CLAUDE.md in root idempotently using atomic write.
func SyncClaudeMD(root string) error {
	root = filepath.Clean(root)
	target := filepath.Join(root, "CLAUDE.md")
	var existing string
	if info, err := os.Lstat(target); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to modify symlinked CLAUDE.md: %s", target)
		}
		data, readErr := os.ReadFile(target)
		if readErr != nil {
			return readErr
		}
		existing = string(data)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	rendered := RenderClaudeMD(existing, IsDevkitSourceRepo(root))
	if existing == rendered {
		return nil
	}
	tmp, err := os.CreateTemp(root, ".claude-md-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := io.WriteString(tmp, rendered); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, target); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
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
