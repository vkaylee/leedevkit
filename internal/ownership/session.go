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
	project string
	owned   bool
	lock    *os.File
	once    sync.Once
	err     error
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
	path := filepath.Join(os.TempDir(), project+".lock")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open project lock: %w", err)
	}
	if err := lockFile(file); err != nil {
		file.Close()
		return nil, fmt.Errorf("project %q is already owned", project)
	}
	session.lock = file
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
