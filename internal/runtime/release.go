package runtime

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	defaultReleaseBase = "https://github.com/vkaylee/leedevkit/releases"
	defaultReleaseAPI  = "https://api.github.com/repos/vkaylee/leedevkit/releases"
	maxReleaseBytes    = 512 << 20
	maxReleaseEntries  = 100000
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
	source := localSource
	if source == "" {
		var err error
		source, err = releaseRootFromExecutable()
		if err != nil {
			return err
		}
	}
	if !filepath.IsAbs(source) {
		source = filepath.Join(root, source)
	}
	return Init(root, source, force)
}

// CurrentExecutablePath returns the symlink-resolved absolute path of the running binary.
func CurrentExecutablePath() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate leedevkit executable: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return "", fmt.Errorf("resolve leedevkit executable: %w", err)
	}
	absolute, err := filepath.Abs(resolved)
	if err != nil {
		return "", fmt.Errorf("resolve leedevkit executable path: %w", err)
	}
	return absolute, nil
}

// AssetRootFromExecutable derives the DevKit root directory from an executable path
// (handling both <root>/bin/leedevkit and <root>/bin/go/<binary>).
func AssetRootFromExecutable(executable string) string {
	dir := filepath.Dir(executable)
	if filepath.Base(dir) == "go" && filepath.Base(filepath.Dir(dir)) == "bin" {
		dir = filepath.Dir(dir)
	}
	return filepath.Dir(dir)
}

func releaseRootFromExecutable() (string, error) {
	executable, err := CurrentExecutablePath()
	if err != nil {
		return "", err
	}
	for candidate := filepath.Dir(executable); ; candidate = filepath.Dir(candidate) {
		if exists(filepath.Join(candidate, "VERSION")) && directoryExists(filepath.Join(candidate, ".agent", "rules")) {
			return candidate, nil
		}
		parent := filepath.Dir(candidate)
		if parent == candidate {
			break
		}
	}
	return "", errors.New("cannot locate global devkit assets from executable")
}

func directoryExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func UpdateProject(root, target, localSource string) error {
	root = filepath.Clean(root)
	if target == "" {
		target = "latest"
	}
	release, err := acquireRelease(root, localSource, target, true)
	if err != nil {
		return err
	}
	defer release.Close()
	return updateGlobal(release.Root)
}

func globalHome() string {
	if home := os.Getenv("DEVKIT_HOME"); home != "" {
		return filepath.Clean(home)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), ".leedevkit")
	}
	return filepath.Join(home, ".leedevkit")
}

func globalCurrentDir() string {
	return filepath.Join(globalHome(), "current")
}

func copyExecutable(source, target string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("source is not executable regular file: %s", source)
	}
	output, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		return err
	}
	if err := output.Sync(); err != nil {
		_ = output.Close()
		return err
	}
	return output.Close()
}

func validReleaseVersion(version string) bool {
	if version == "" || version == "." || version == ".." || strings.ContainsAny(version, `/\\`) {
		return false
	}
	for _, character := range version {
		if !(character >= 'a' && character <= 'z') && !(character >= 'A' && character <= 'Z') && !(character >= '0' && character <= '9') && character != '.' && character != '-' && character != '+' {
			return false
		}
	}
	return true
}

func updateGlobal(source string) error {
	home := globalHome()
	if err := os.MkdirAll(home, 0o755); err != nil {
		return err
	}
	lockPath := filepath.Join(home, ".install.lock")
	if err := os.Mkdir(lockPath, 0o700); errors.Is(err, os.ErrExist) {
		return errors.New("global install or update already in progress")
	} else if err != nil {
		return fmt.Errorf("lock global update: %w", err)
	}
	defer os.Remove(lockPath)
	if _, err := os.Lstat(filepath.Join(source, "devkit.manifest.json")); err != nil {
		return fmt.Errorf("release integrity manifest required: %w", err)
	}
	result, err := VerifyManifest(source)
	if err != nil {
		return fmt.Errorf("verify release integrity: %w", err)
	}
	if !result.Clean() {
		return fmt.Errorf("release integrity check failed: modified=%v missing=%v extra=%v invalid=%v", result.Modified, result.Missing, result.Extra, result.Invalid)
	}
	version, err := readVersion(filepath.Join(source, "VERSION"))
	if err != nil {
		return err
	}
	if !validReleaseVersion(version) {
		return fmt.Errorf("invalid release version %q", version)
	}
	binaryName, err := nativeBinaryName()
	if err != nil {
		return err
	}
	binary := filepath.Join(source, "bin", "go", binaryName)
	if info, err := os.Stat(binary); err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("release missing executable native binary: %s", binary)
	}
	launcher := filepath.Join(source, "bin", "leedevkit")
	if info, err := os.Stat(launcher); err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("release missing launcher: %s", launcher)
	}
	current := filepath.Join(home, "current")
	oldTarget, err := os.Lstat(current)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect global current: %w", err)
	}
	var previousVersion string
	if err == nil {
		if oldTarget.Mode()&os.ModeSymlink == 0 {
			return errors.New("global current is not a symlink; refusing update")
		}
		resolvedCurrent, resolveErr := filepath.EvalSymlinks(current)
		if resolveErr != nil {
			return fmt.Errorf("resolve global current: %w", resolveErr)
		}
		resolvedHome, resolveErr := filepath.EvalSymlinks(home)
		if resolveErr != nil {
			return resolveErr
		}
		relative, relErr := filepath.Rel(resolvedHome, resolvedCurrent)
		if relErr != nil || filepath.IsAbs(relative) || filepath.Dir(relative) != "." || !validReleaseVersion(relative) {
			return errors.New("global current points outside managed releases; refusing update")
		}
		previousVersion = relative
		if previousVersion == "v"+version {
			return nil
		}
	}
	stage, err := os.MkdirTemp(home, ".update-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	staged := filepath.Join(stage, "release")
	if err := copyPayload(source, staged); err != nil {
		return err
	}
	stagedResult, err := VerifyManifest(staged)
	if err != nil || !stagedResult.Clean() {
		return fmt.Errorf("staged release integrity check failed: result=%+v err=%v", stagedResult, err)
	}
	if err := copyExecutable(filepath.Join(staged, "bin", "go", binaryName), filepath.Join(staged, "bin", "leedevkit")); err != nil {
		return fmt.Errorf("prepare global launcher: %w", err)
	}
	versionDir := filepath.Join(home, "v"+version)
	createdVersionDir := false
	if exists(versionDir) {
		info, infoErr := os.Lstat(versionDir)
		existingVersion, versionErr := readVersion(filepath.Join(versionDir, "VERSION"))
		existingManifest, manifestErr := VerifyManifest(versionDir)
		if infoErr != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || versionErr != nil || existingVersion != version || manifestErr != nil || !existingManifest.Clean() {
			return fmt.Errorf("refusing to activate invalid existing global release: %s", versionDir)
		}
		if err := activateGlobalVersion(home, current, version); err != nil {
			return err
		}
		return nil
	} else if err := os.Rename(staged, versionDir); err != nil {
		return fmt.Errorf("stage global release: %w", err)
	} else {
		createdVersionDir = true
	}
	cleanupVersion := func() {
		if createdVersionDir {
			_ = os.RemoveAll(versionDir)
		}
	}
	if err := activateGlobalVersion(home, current, version); err != nil {
		cleanupVersion()
		return err
	}
	return nil
}

func activateGlobalVersion(home, current, version string) error {
	newLink, err := os.CreateTemp(home, ".current-update-*")
	if err != nil {
		return err
	}
	newLinkPath := newLink.Name()
	if err := newLink.Close(); err != nil {
		_ = os.Remove(newLinkPath)
		return err
	}
	if err := os.Remove(newLinkPath); err != nil {
		return err
	}
	if err := os.Symlink("v"+version, newLinkPath); err != nil {
		return err
	}
	if err := os.Rename(newLinkPath, current); err != nil {
		_ = os.Remove(newLinkPath)
		return fmt.Errorf("activate global release: %w", err)
	}
	return nil
}

func acquireRelease(root, localSource, version string, globalOnly bool) (Release, error) {
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
	if !globalOnly {
		if _, err := os.Stat(filepath.Join(root, "VERSION")); err == nil {
			return Release{Root: root}, nil
		}
	}
	return Download(version)
}

// ReleaseClient fetches and verifies DevKit release archives using injectable HTTP client and base URL.
type ReleaseClient struct {
	HTTPClient *http.Client
	BaseURL    string
}

func defaultReleaseClient() ReleaseClient {
	return ReleaseClient{
		HTTPClient: &http.Client{Timeout: releaseTimeout()},
		BaseURL:    os.Getenv("LEEDEVKIT_RELEASE_BASE_URL"),
	}
}

func (c ReleaseClient) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: releaseTimeout()}
}

func Download(version string) (Release, error) {
	return defaultReleaseClient().Download(version)
}

func (c ReleaseClient) Download(version string) (Release, error) {
	if version == "" || version == "latest" {
		var err error
		version, err = c.LatestVersion()
		if err != nil {
			return Release{}, err
		}
	}
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	if !validReleaseVersion(version) || version == "latest" {
		return Release{}, fmt.Errorf("invalid release version %q", version)
	}
	base := strings.TrimRight(c.BaseURL, "/")
	if base == "" {
		base = defaultReleaseBase
	}
	url := fmt.Sprintf("%s/download/v%s/leedevkit-%s.tar.gz", base, version, version)
	client := c.httpClient()
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
	checksumRequest, err := http.NewRequest(http.MethodGet, url+".sha256", nil)
	if err != nil {
		return Release{}, err
	}
	checksumRequest.Header.Set("User-Agent", "leedevkit-go")
	checksumResponse, err := client.Do(checksumRequest)
	if err != nil {
		return Release{}, fmt.Errorf("download release checksum: %w", err)
	}
	checksumData, checksumErr := io.ReadAll(io.LimitReader(checksumResponse.Body, 1025))
	checksumCloseErr := checksumResponse.Body.Close()
	if checksumErr != nil || checksumCloseErr != nil || checksumResponse.StatusCode != http.StatusOK {
		return Release{}, errors.New("download release checksum failed")
	}
	expectedChecksum := strings.Fields(string(checksumData))
	if len(checksumData) > 1024 || len(expectedChecksum) != 2 || len(expectedChecksum[0]) != sha256.Size*2 {
		return Release{}, errors.New("release checksum format invalid")
	}
	if _, err := hex.DecodeString(expectedChecksum[0]); err != nil {
		return Release{}, errors.New("release checksum format invalid")
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
	hash := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(archive, hash), io.LimitReader(response.Body, maxReleaseBytes+1))
	closeErr := archive.Close()
	if copyErr != nil || closeErr != nil {
		cleanup()
		return Release{}, fmt.Errorf("write release archive: %w", firstError(copyErr, closeErr))
	}
	if info, statErr := os.Stat(archivePath); statErr != nil || info.Size() > maxReleaseBytes {
		cleanup()
		return Release{}, errors.New("release archive exceeds maximum size")
	}
	if !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), expectedChecksum[0]) {
		cleanup()
		return Release{}, errors.New("release archive checksum mismatch")
	}
	extractRoot := filepath.Join(stage, "extracted")
	if err := os.MkdirAll(extractRoot, 0o700); err != nil {
		cleanup()
		return Release{}, err
	}
	root, err := extractArchive(archivePath, extractRoot)
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

func latestReleaseAPI(base string) string {
	if strings.HasPrefix(base, "http://127.0.0.1") || strings.HasPrefix(base, "http://localhost") {
		return strings.TrimRight(base, "/") + "/latest"
	}
	base = strings.TrimRight(base, "/")
	if strings.HasSuffix(base, "/releases") {
		base = strings.TrimSuffix(base, "/releases")
	}
	if strings.Contains(base, "api.github.com/repos/") {
		return base + "/releases/latest"
	}
	return defaultReleaseAPI + "/latest"
}

func latestVersion() (string, error) {
	return defaultReleaseClient().LatestVersion()
}

func (c ReleaseClient) LatestVersion() (string, error) {
	api := latestReleaseAPI(c.BaseURL)
	request, err := http.NewRequest(http.MethodGet, api, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "leedevkit-go")
	response, err := c.httpClient().Do(request)
	if err != nil {
		return "", fmt.Errorf("resolve latest release: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("resolve latest release: HTTP %s", response.Status)
	}
	mediaType := strings.ToLower(response.Header.Get("Content-Type"))
	if !strings.Contains(mediaType, "application/json") {
		return "", fmt.Errorf("resolve latest release: unexpected content type %q", mediaType)
	}
	var payload struct {
		Tag string `json:"tag_name"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil {
		return "", fmt.Errorf("decode latest release: %w", err)
	}
	if payload.Tag == "" {
		return "", errors.New("resolve latest release: tag_name is missing")
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
	var totalExpanded int64
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
		if header.Size < 0 || header.Size > maxReleaseBytes || totalExpanded+header.Size > maxReleaseBytes || len(members) >= maxReleaseEntries {
			return "", errors.New("release archive exceeds extraction limits")
		}
		totalExpanded += header.Size
		members = append(members, member{header.Name, header.Mode, header.Typeflag, header.Size})
	}
	if err := compressed.Close(); err != nil {
		return "", err
	}
	if _, err := input.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
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

func nativeBinaryName() (string, error) {
	var osName, architecture string
	switch runtime.GOOS {
	case "linux":
		osName = "linux"
	case "darwin":
		osName = "darwin"
	case "windows":
		osName = "windows"
	default:
		return "", fmt.Errorf("unsupported platform %q", runtime.GOOS)
	}
	switch runtime.GOARCH {
	case "amd64":
		architecture = "amd64"
	case "arm64":
		architecture = "arm64"
	default:
		return "", fmt.Errorf("unsupported architecture %q", runtime.GOARCH)
	}
	suffix := ""
	if osName == "windows" {
		suffix = ".exe"
	}
	return fmt.Sprintf("leedevkit-go-%s-%s%s", osName, architecture, suffix), nil
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
