package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func runSkillsList(root string, stdout, stderr io.Writer) int {
	fmt.Fprintln(stderr, "Skills")
	fmt.Fprintln(stderr, "")

	builtin := filepath.Join(root, ".leedevkit", ".agent", "skills")
	community := filepath.Join(root, ".leedevkit", "skills.d")
	builtinCount := listSkillNames(builtin, stderr, "Built-in")
	communityCount := listSkillNames(community, stderr, "Community")
	if builtinCount == 0 && communityCount == 0 {
		fmt.Fprintln(stderr, "No community skills. Add one:")
		fmt.Fprintln(stderr, "  leedevkit skills add <git-url>")
	}
	return 0
}

func listSkillNames(root string, output io.Writer, label string) int {
	entries, err := os.ReadDir(root)
	if err != nil {
		return 0
	}
	count := 0
	fmt.Fprintf(output, "%s:\n", label)
	for _, entry := range entries {
		if entry.IsDir() {
			fmt.Fprintf(output, "  ▸ %s\n", entry.Name())
			count++
		}
	}
	return count
}
