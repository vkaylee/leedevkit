package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunSkillsListShowsDirectoriesOnly(t *testing.T) {
	root := t.TempDir()
	builtin := filepath.Join(root, "global", ".agent", "skills")
	community := filepath.Join(root, ".agent", "skills.d")
	for _, path := range []string{
		filepath.Join(builtin, "testing", "SKILL.md"),
		filepath.Join(community, "demo", "SKILL.md"),
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("skill"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(builtin, "README.md"), []byte("not a skill directory"), 0o600); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	if code := runSkillsListWithBuiltinRoot(root, builtin, &stderr); code != 0 {
		t.Fatalf("skills list exit code = %d", code)
	}
	output := stderr.String()
	for _, want := range []string{"Built-in:", "Community:", "testing", "demo"} {
		if !strings.Contains(output, want) {
			t.Fatalf("skills output missing %q: %q", want, output)
		}
	}
	if strings.Contains(output, "README.md") {
		t.Fatalf("regular file listed as skill: %q", output)
	}
}

func TestGlobalSkillsRootUsesExecutableLocation(t *testing.T) {
	executable := filepath.Join("", "opt", "leedevkit", "current", "bin", "leedevkit")
	if got, want := globalSkillsRootFromExecutable(executable), filepath.Join("", "opt", "leedevkit", "current", ".agent", "skills"); got != want {
		t.Fatalf("globalSkillsRootFromExecutable() = %q, want %q", got, want)
	}
}

func TestRunSkillsListShowsLegacySkillsWithoutChangingThem(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, ".leedevkit", "skills.d", "demo")
	community := filepath.Join(root, ".agent", "skills.d", "demo")
	for path, body := range map[string]string{
		filepath.Join(legacy, "SKILL.md"):    "legacy",
		filepath.Join(community, "SKILL.md"): "project",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	before := snapshotTree(t, root)
	var stderr bytes.Buffer
	if code := runSkillsListWithBuiltinRoot(root, "", &stderr); code != 0 {
		t.Fatalf("skills list exit code = %d", code)
	}
	for _, want := range []string{"Legacy community (not migrated):", "demo", "remain in .leedevkit/skills.d"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("legacy output missing %q: %q", want, stderr.String())
		}
	}
	if after := snapshotTree(t, root); strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Fatalf("skills list mutated project: before=%v after=%v", before, after)
	}
}

func TestRunSkillsListLegacyOnlyDoesNotReportEmpty(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".leedevkit", "skills.d", "demo"), 0o700); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	runSkillsListWithBuiltinRoot(root, "", &stderr)
	if strings.Contains(stderr.String(), "No community skills") {
		t.Fatalf("legacy skill reported as absent: %s", stderr.String())
	}
}

func TestRunSkillsListIsReadOnlyAndHandlesMissingRoots(t *testing.T) {
	root := t.TempDir()
	before := snapshotTree(t, root)
	var stderr bytes.Buffer
	if code := runSkillsList(root, &bytes.Buffer{}, &stderr); code != 0 {
		t.Fatalf("skills list exit code = %d", code)
	}
	if !strings.Contains(stderr.String(), "No community skills. Add one:") {
		t.Fatalf("missing guidance: %q", stderr.String())
	}
	if after := snapshotTree(t, root); strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Fatalf("skills list mutated project: before=%v after=%v", before, after)
	}
}

func TestRunSkillsListCommandUsesProjectOwnedCommunityPath(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".agent", "skills.d", "testing"), 0o700); err != nil {
		t.Fatal(err)
	}
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })

	var stdout, stderr bytes.Buffer
	if code := run([]string{"skills", "list"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("skills list exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "testing") {
		t.Fatalf("Go skills output = %q", stderr.String())
	}
}

func snapshotTree(t *testing.T, root string) []string {
	t.Helper()
	var paths []string
	if err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		paths = append(paths, relative)
		if info.IsDir() {
			return nil
		}
		paths[len(paths)-1] += ":" + info.Mode().String()
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return paths
}
