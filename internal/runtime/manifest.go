package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type manifest struct {
	Version     string            `json:"version"`
	GeneratedAt string            `json:"generated_at"`
	Algorithm   string            `json:"algorithm"`
	FileCount   int               `json:"file_count"`
	Files       map[string]string `json:"files"`
}

type ManifestResult struct {
	FileCount  int
	Modified   []string
	Missing    []string
	Extra      []string
	Invalid    []string
	NoManifest bool
}

func (r ManifestResult) Clean() bool {
	return !r.NoManifest && len(r.Modified) == 0 && len(r.Missing) == 0 && len(r.Extra) == 0 && len(r.Invalid) == 0
}

func WriteManifest(root string) error {
	root = filepath.Clean(root)
	version, err := os.ReadFile(filepath.Join(root, "VERSION"))
	if err != nil {
		return err
	}
	files, err := walkManifestFiles(root)
	if err != nil {
		return err
	}
	payload := manifest{Version: strings.TrimSpace(string(version)), GeneratedAt: "", Algorithm: "sha256", FileCount: len(files), Files: files}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, "devkit.manifest.json"), append(data, '\n'), 0o644)
}

func VerifyManifest(root string) (ManifestResult, error) {
	result := ManifestResult{}
	path := filepath.Join(root, "devkit.manifest.json")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		result.NoManifest = true
		return result, nil
	}
	if err != nil {
		return result, err
	}
	var expected manifest
	if err := json.Unmarshal(data, &expected); err != nil {
		result.Invalid = append(result.Invalid, "manifest JSON invalid")
		return result, nil
	}
	if expected.Algorithm != "sha256" {
		result.Invalid = append(result.Invalid, "algorithm must be sha256")
	}
	version, err := os.ReadFile(filepath.Join(root, "VERSION"))
	if err != nil || strings.TrimSpace(string(version)) != expected.Version {
		result.Invalid = append(result.Invalid, "version does not match VERSION")
	}
	if expected.FileCount != len(expected.Files) {
		result.Invalid = append(result.Invalid, "file_count does not match files")
	}
	actual, err := walkManifestFiles(root)
	if err != nil {
		return result, err
	}
	result.FileCount = len(expected.Files)
	for name, hash := range expected.Files {
		actualHash, ok := actual[name]
		if !ok {
			result.Missing = append(result.Missing, name)
			continue
		}
		if actualHash != hash {
			result.Modified = append(result.Modified, name)
		}
		delete(actual, name)
	}
	for name := range actual {
		result.Extra = append(result.Extra, name)
	}
	sort.Strings(result.Modified)
	sort.Strings(result.Missing)
	sort.Strings(result.Extra)
	return result, nil
}

func walkManifestFiles(root string) (map[string]string, error) {
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path != root && ignoredManifestPath(root, path) {
				return filepath.SkipDir
			}
			return nil
		}
		if ignoredManifestPath(root, path) || entry.Name() == "devkit.manifest.json" || entry.Name() == "dev-state.json" || entry.Name() == ".DS_Store" {
			return nil
		}
		hash, err := fileHash(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = hash
		return nil
	})
	return files, err
}

func ignoredManifestPath(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return true
	}
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		if part == ".venv" || part == "skills.d" || part == "__pycache__" || part == ".git" {
			return true
		}
	}
	return false
}

func fileHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}
