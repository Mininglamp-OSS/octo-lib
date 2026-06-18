package log

import (
	"sync"
	"testing"
)

// resetLoggers forces the lazy-init path by clearing the published logger set,
// independent of test ordering or -shuffle. It is white-box (same package) on
// purpose: the globals are unexported.
func resetLoggers() {
	configMu.Lock()
	defer configMu.Unlock()
	loggers.Store(nil)
}

// TestConcurrentLazyInit reproduces issue #77: before the sync fix, the first
// concurrent log calls raced on the unsynchronized lazy init (log.Warn reading a
// global that Configure was concurrently writing). Run with -race to guard
// against regressions.
func TestConcurrentLazyInit(t *testing.T) {
	resetLoggers()

	const goroutines = 50
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start // release all at once to maximize contention on first-use init
			Info("concurrent info")
			Warn("concurrent warn")
			Error("concurrent error")
			Debug("concurrent debug")
		}()
	}
	close(start)
	wg.Wait()

	if loggers.Load() == nil {
		t.Fatal("loggers should be configured after concurrent lazy-init use")
	}
}

// TestConcurrentConfigureAndLog ensures explicit Configure is safe alongside
// concurrent logging (the lazy init and an explicit reconfigure both publish the
// globals; they must not race).
func TestConcurrentConfigureAndLog(t *testing.T) {
	resetLoggers()

	const goroutines = 25
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < goroutines; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			Configure(NewOptions())
		}()
		go func() {
			defer wg.Done()
			<-start
			Warn("racing with configure")
		}()
	}
	close(start)
	wg.Wait()

	if loggers.Load() == nil {
		t.Fatal("loggers should be configured after concurrent Configure/log use")
	}
}
