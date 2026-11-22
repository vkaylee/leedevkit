package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func runSkillsList(root string, stdout, stderr io.Writer) int {
	return runSkillsListWithBuiltinRoot(root, globalSkillsRoot(), stderr)
}

func runSkillsListWithBuiltinRoot(root, builtin string, stderr io.Writer) int {
	fmt.Fprintln(stderr, "Skills")
	fmt.Fprintln(stderr, "")

	if builtin != "" {
		if info, err := os.Stat(builtin); err != nil || !info.IsDir() {
			builtin = ""
		}
	}
	community := filepath.Join(root, ".agent", "skills.d")
	legacy := filepath.Join(root, ".leedevkit", "skills.d")
	builtinCount := listSkillNames(builtin, stderr, "Built-in")
	communityCount := listSkillNames(community, stderr, "Community")
	legacyCount := listSkillNames(legacy, stderr, "Legacy community (not migrated)")
	if legacyCount > 0 {
		fmt.Fprintln(stderr, "Legacy skills remain in .leedevkit/skills.d; move them manually after reviewing conflicts.")
	}
	if builtinCount == 0 && communityCount == 0 && legacyCount == 0 {
		fmt.Fprintln(stderr, "No community skills. Add one:")
		fmt.Fprintln(stderr, "  leedevkit skills add <git-url>")
	}
	return 0
}

func globalSkillsRoot() string {
	executable, err := os.Executable()
	if err != nil {
		return ""
	}
	resolved, err := filepath.EvalSymlinks(executable)
	if err != nil {
		resolved = executable
	}
	return globalSkillsRootFromExecutable(resolved)
}

func globalSkillsRootFromExecutable(executable string) string {
	return filepath.Join(filepath.Dir(filepath.Dir(executable)), ".agent", "skills")
}

func listSkillNames(root string, output io.Writer, label string) int {
	entries, err := os.ReadDir(root)
	if err != nil {
		return 0
	}
	count := 0
	fmt.Fprintf(output, "%s:\n", label)
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			fmt.Fprintf(output, "  ▸ %s\n", entry.Name())
			count++
		}
	}
	return count
}
