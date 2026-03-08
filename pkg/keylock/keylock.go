package keylock

import (
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultCleanInterval = 24 * time.Hour
)

// KeyLock provides per-key mutual exclusion.
type KeyLock struct {
	locks         map[string]*innerLock
	cleanInterval time.Duration
	stopChan      chan struct{}
	mutex         sync.Mutex
}

// NewKeyLock creates a new KeyLock.
func NewKeyLock() *KeyLock {
	return &KeyLock{
		locks:         make(map[string]*innerLock),
		cleanInterval: defaultCleanInterval,
		stopChan:      make(chan struct{}),
	}
}

// Lock acquires the lock for the given key.
func (l *KeyLock) Lock(key string) {
	l.mutex.Lock()
	keyLock, ok := l.locks[key]
	if !ok {
		keyLock = newInnerLock()
		l.locks[key] = keyLock
	}
	keyLock.add()
	l.mutex.Unlock()
	keyLock.Lock()
}

// Unlock releases the lock for the given key.
func (l *KeyLock) Unlock(key string) {
	l.mutex.Lock()
	keyLock, ok := l.locks[key]
	l.mutex.Unlock()
	if ok {
		keyLock.done()
		keyLock.Unlock()
	}
}

// Clean removes idle locks (count == 0).
func (l *KeyLock) Clean() {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	for k, v := range l.locks {
		if atomic.LoadInt64(&v.count) == 0 {
			delete(l.locks, k)
		}
	}
}

// StartCleanLoop starts a background goroutine that periodically cleans idle locks.
func (l *KeyLock) StartCleanLoop() {
	go l.cleanLoop()
}

// StopCleanLoop stops the background clean goroutine.
func (l *KeyLock) StopCleanLoop() {
	close(l.stopChan)
}

func (l *KeyLock) cleanLoop() {
	ticker := time.NewTicker(l.cleanInterval)
	for {
		select {
		case <-ticker.C:
			l.Clean()
		case <-l.stopChan:
			ticker.Stop()
			return
		}
	}
}

type innerLock struct {
	count int64
	sync.Mutex
}

func newInnerLock() *innerLock {
	return &innerLock{}
}

func (il *innerLock) add() {
	atomic.AddInt64(&il.count, 1)
}

func (il *innerLock) done() {
	atomic.AddInt64(&il.count, -1)
}
