package runtime

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultReleaseBase = "https://github.com/vkaylee/leedevkit/releases"
	maxReleaseBytes    = 512 << 20
)

type Release struct {
	Root    string
	cleanup func()
}

func (r Release) Close() {
	if r.cleanup != nil {
		r.cleanup()
	}
}

func InitProject(root, localSource string, force bool) error {
	root = filepath.Clean(root)
	configPath := filepath.Join(root, "leedevkit.toml")
	configured, err := configuredVersion(configPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if configured == "" {
		configured = "latest"
	}

	release, err := acquireRelease(root, localSource, configured)
	if err != nil {
		return err
	}
	defer release.Close()
	return Init(root, release.Root, force)
}

func UpdateProject(root, target, localSource string) error {
	root = filepath.Clean(root)
	configPath := filepath.Join(root, "leedevkit.toml")
	configured, err := configuredVersion(configPath)
	if err != nil {
		return err
	}
	if target == "" {
		target = "latest"
	}
	release, err := acquireRelease(root, localSource, target)
	if err != nil {
		return err
	}
	defer release.Close()
	version, err := readVersion(filepath.Join(release.Root, "VERSION"))
	if err != nil {
		return err
	}
	current, err := readVersion(filepath.Join(root, ".leedevkit", "VERSION"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if current == version && configured == version {
		return nil
	}
	if err := replaceRuntime(filepath.Join(root, ".leedevkit"), release.Root, []string{"skills.d", ".venv"}); err != nil {
		return err
	}
	return updateConfigVersion(configPath, version)
}

func acquireRelease(root, localSource, version string) (Release, error) {
	if localSource != "" {
		path := filepath.Clean(localSource)
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
		if _, err := os.Stat(filepath.Join(path, "VERSION")); err != nil {
			return Release{}, fmt.Errorf("local devkit source invalid: %w", err)
		}
		return Release{Root: path}, nil
	}
	if _, err := os.Stat(filepath.Join(root, "VERSION")); err == nil {
		return Release{Root: root}, nil
	}
	return Download(version)
}

func Download(version string) (Release, error) {
	if version == "" || version == "latest" {
		var err error
		version, err = latestVersion()
		if err != nil {
			return Release{}, err
		}
	}
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	if version == "" {
		return Release{}, errors.New("release version is empty")
	}
	base := strings.TrimRight(os.Getenv("LEEDEVKIT_RELEASE_BASE_URL"), "/")
	if base == "" {
		base = defaultReleaseBase
	}
	url := fmt.Sprintf("%s/download/v%s/leedevkit-%s.tar.gz", base, version, version)
	client := &http.Client{Timeout: releaseTimeout()}
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return Release{}, err
	}
	request.Header.Set("User-Agent", "leedevkit-go")
	response, err := client.Do(request)
	if err != nil {
		return Release{}, fmt.Errorf("download release: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("download release: HTTP %s", response.Status)
	}
	stage, err := os.MkdirTemp("", "leedevkit-release-")
	if err != nil {
		return Release{}, err
	}
	cleanup := func() { _ = os.RemoveAll(stage) }
	archivePath := filepath.Join(stage, "release.tar.gz")
	archive, err := os.OpenFile(archivePath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
	if err != nil {
		cleanup()
		return Release{}, err
	}
	_, copyErr := io.Copy(archive, io.LimitReader(response.Body, maxReleaseBytes+1))
	closeErr := archive.Close()
	if copyErr != nil || closeErr != nil {
		cleanup()
		return Release{}, fmt.Errorf("write release archive: %w", firstError(copyErr, closeErr))
	}
	if info, statErr := os.Stat(archivePath); statErr != nil || info.Size() > maxReleaseBytes {
		cleanup()
		return Release{}, errors.New("release archive exceeds maximum size")
	}
	root, err := extractArchive(archivePath, stage)
	if err != nil {
		cleanup()
		return Release{}, err
	}
	actual, err := readVersion(filepath.Join(root, "VERSION"))
	if err != nil || actual != version {
		cleanup()
		return Release{}, fmt.Errorf("release version mismatch: got %q want %q", actual, version)
	}
	return Release{Root: root, cleanup: cleanup}, nil
}

func latestVersion() (string, error) {
	base := strings.TrimRight(os.Getenv("LEEDEVKIT_RELEASE_BASE_URL"), "/")
	if base == "" {
		base = defaultReleaseBase
	}
	api := strings.Replace(base, "/releases", "", 1) + "/releases/latest"
	request, err := http.NewRequest(http.MethodGet, api, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "leedevkit-go")
	response, err := (&http.Client{Timeout: releaseTimeout()}).Do(request)
	if err != nil {
		return "", fmt.Errorf("resolve latest release: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("resolve latest release: HTTP %s", response.Status)
	}
	var payload struct {
		Tag string `json:"tag_name"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("decode latest release: %w", err)
	}
	return strings.TrimPrefix(payload.Tag, "v"), nil
}

func extractArchive(archivePath, stage string) (string, error) {
	input, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer input.Close()
	compressed, err := gzip.NewReader(input)
	if err != nil {
		return "", fmt.Errorf("open release gzip: %w", err)
	}
	defer compressed.Close()
	tarball := tar.NewReader(compressed)
	type member struct {
		name string
		mode int64
		kind byte
		size int64
	}
	members := []member{}
	for {
		header, readErr := tarball.Next()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return "", fmt.Errorf("read release archive: %w", readErr)
		}
		clean := filepath.Clean(filepath.FromSlash(header.Name))
		if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
			return "", fmt.Errorf("unsafe release member: %s", header.Name)
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeDir {
			return "", fmt.Errorf("unsupported release member: %s", header.Name)
		}
		members = append(members, member{header.Name, header.Mode, header.Typeflag, header.Size})
	}
	input.Seek(0, io.SeekStart)
	compressed.Close()
	compressed, err = gzip.NewReader(input)
	if err != nil {
		return "", err
	}
	tarball = tar.NewReader(compressed)
	for _, item := range members {
		if _, err := tarball.Next(); err != nil {
			return "", err
		}
		destination := filepath.Join(stage, filepath.FromSlash(item.name))
		if item.kind == tar.TypeDir {
			if err := os.MkdirAll(destination, os.FileMode(item.mode)&0o777); err != nil {
				return "", err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return "", err
		}
		output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, os.FileMode(item.mode)&0o777)
		if err != nil {
			return "", err
		}
		_, copyErr := io.CopyN(output, tarball, item.size)
		closeErr := output.Close()
		if copyErr != nil || closeErr != nil {
			return "", firstError(copyErr, closeErr)
		}
	}
	entries, err := os.ReadDir(stage)
	if err != nil {
		return "", err
	}
	if len(entries) == 1 && entries[0].IsDir() {
		return filepath.Join(stage, entries[0].Name()), nil
	}
	return stage, nil
}

func replaceRuntime(target, source string, preserve []string) error {
	parent := filepath.Dir(target)
	stage, err := os.MkdirTemp(parent, ".leedevkit-update-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	staged := filepath.Join(stage, "runtime")
	if err := copyPayload(source, staged); err != nil {
		return err
	}
	backup := filepath.Join(stage, "previous")
	if exists(target) && !exists(backup) {
		if err := os.Rename(target, backup); err != nil {
			return err
		}
	}
	if err := os.Rename(staged, target); err != nil {
		if exists(backup) {
			_ = os.Rename(backup, target)
		}
		return err
	}
	for _, name := range preserve {
		old, next := filepath.Join(backup, name), filepath.Join(target, name)
		if exists(old) {
			_ = os.RemoveAll(next)
			if err := os.Rename(old, next); err != nil {
				_ = os.RemoveAll(target)
				_ = os.Rename(backup, target)
				return err
			}
		}
	}
	return nil
}

func updateConfigVersion(path, version string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	inDevkit := false
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			inDevkit = trimmed == "[devkit]"
			continue
		}
		if inDevkit && strings.HasPrefix(trimmed, "version") {
			prefix, _, found := strings.Cut(line, "=")
			if found {
				lines[index] = prefix + "= \"" + version + "\""
				return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
			}
		}
	}
	return fmt.Errorf("project config has no [devkit].version")
}

func releaseTimeout() time.Duration {
	return 120 * time.Second
}

func firstError(values ...error) error {
	for _, err := range values {
		if err != nil {
			return err
		}
	}
	return nil
}
