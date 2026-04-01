package pipeline

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestProjectLockerSerializesSameProject(t *testing.T) {
	locker := NewProjectLocker()

	var active int32
	var maxActive int32
	var wg sync.WaitGroup

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := locker.WithProject("same-project", func() error {
				current := atomic.AddInt32(&active, 1)
				updateMax(&maxActive, current)
				time.Sleep(40 * time.Millisecond)
				atomic.AddInt32(&active, -1)
				return nil
			}); err != nil {
				t.Fatalf("unexpected lock error: %v", err)
			}
		}()
	}

	wg.Wait()

	if got := atomic.LoadInt32(&maxActive); got != 1 {
		t.Fatalf("expected same project to run serially, max concurrency=%d", got)
	}
	if got := len(locker.locks); got != 0 {
		t.Fatalf("expected internal lock map to be cleaned up, got=%d", got)
	}
}

func TestProjectLockerAllowsDifferentProjectsInParallel(t *testing.T) {
	locker := NewProjectLocker()

	entered := make(chan string, 2)
	release := make(chan struct{})
	var wg sync.WaitGroup

	for _, projectID := range []string{"project-a", "project-b"} {
		wg.Add(1)
		go func(projectID string) {
			defer wg.Done()
			if err := locker.WithProject(projectID, func() error {
				entered <- projectID
				<-release
				return nil
			}); err != nil {
				t.Fatalf("unexpected lock error for %s: %v", projectID, err)
			}
		}(projectID)
	}

	for i := 0; i < 2; i++ {
		select {
		case <-entered:
		case <-time.After(200 * time.Millisecond):
			t.Fatal("expected different projects to enter concurrently")
		}
	}

	close(release)
	wg.Wait()

	if got := len(locker.locks); got != 0 {
		t.Fatalf("expected internal lock map to be cleaned up, got=%d", got)
	}
}

func updateMax(maxValue *int32, current int32) {
	for {
		previous := atomic.LoadInt32(maxValue)
		if current <= previous {
			return
		}
		if atomic.CompareAndSwapInt32(maxValue, previous, current) {
			return
		}
	}
}
