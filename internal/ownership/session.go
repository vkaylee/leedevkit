package ownership

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

var projectNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]*$`)

type Session struct {
	project  string
	owned    bool
	lock     *os.File
	lockPath string
	once     sync.Once
	err      error
}

func lockFilePath(project string) string {
	return filepath.Join(os.TempDir(), project+".lock")
}

// IsLocked reports whether another owner holds the lock for project.
func IsLocked(project string) bool {
	if validateProjectName(project) != nil {
		return false
	}
	file, err := os.OpenFile(lockFilePath(project), os.O_RDWR, 0o600)
	if err != nil {
		return false
	}
	defer file.Close()
	if err := lockFile(file); err != nil {
		return true
	}
	_ = unlockFile(file)
	return false
}

func removeLockFile(project string) {
	if validateProjectName(project) != nil || IsLocked(project) {
		return
	}
	_ = os.Remove(lockFilePath(project))
}

func NewProjectName(prefix string) (string, error) {
	if err := validateProjectName(prefix); err != nil {
		return "", err
	}
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", fmt.Errorf("generate project identity: %w", err)
	}
	return fmt.Sprintf("%s-%x", prefix, suffix), nil
}

func Open(project string, owned bool) (*Session, error) {
	if err := validateProjectName(project); err != nil {
		return nil, err
	}
	session := &Session{project: project, owned: owned}
	if !owned {
		return session, nil
	}
	path := lockFilePath(project)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open project lock: %w", err)
	}
	if err := lockFile(file); err != nil {
		file.Close()
		return nil, fmt.Errorf("project %q is already owned", project)
	}
	session.lock = file
	session.lockPath = path
	return session, nil
}

func (s *Session) Project() string { return s.project }
func (s *Session) Owned() bool     { return s.owned }

func (s *Session) Close(teardown func() error) error {
	s.once.Do(func() {
		if s.owned && teardown != nil {
			s.err = teardown()
		}
		if s.lock != nil {
			if s.lockPath != "" {
				_ = os.Remove(s.lockPath)
			}
			_ = unlockFile(s.lock)
			_ = s.lock.Close()
			s.lock = nil
		}
	})
	return s.err
}

func validateProjectName(project string) error {
	if project == "" || strings.HasPrefix(project, "pod_") || !projectNamePattern.MatchString(project) {
		return errors.New("invalid project name")
	}
	return nil
}
