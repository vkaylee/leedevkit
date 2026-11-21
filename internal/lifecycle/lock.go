package lifecycle

import "sync"

type BuildLock struct{ locks sync.Map }
type buildMutex struct{ mutex sync.Mutex }

func NewBuildLock() *BuildLock { return &BuildLock{} }

func (l *BuildLock) With(key string, work func()) {
	value, _ := l.locks.LoadOrStore(key, &buildMutex{})
	guard := value.(*buildMutex)
	guard.mutex.Lock()
	defer guard.mutex.Unlock()
	work()
}
