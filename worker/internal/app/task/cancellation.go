package task

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"worker/internal/persistence"
)

const projectCancelPollInterval = 10 * time.Second

type CancelledError struct {
	ProjectID string
}

func (e CancelledError) Error() string {
	if e.ProjectID == "" {
		return "project cancelled"
	}
	return fmt.Sprintf("project cancelled: %s", e.ProjectID)
}

func (e CancelledError) Unwrap() error {
	return context.Canceled
}

func ensureProjectActive(projectID string) error {
	cancelled, err := projectIsCancelled(projectID)
	if err != nil {
		return err
	}
	if cancelled {
		return CancelledError{ProjectID: projectID}
	}
	return nil
}

func projectIsCancelled(projectID string) (bool, error) {
	if projectID == "" {
		return false, nil
	}

	store, err := persistence.DefaultStore()
	if err != nil {
		return false, err
	}

	project, err := store.FindProjectByProjectID(projectID)
	if err != nil {
		return false, err
	}
	return persistence.IsCancellationRequestedStatus(project.Status), nil
}

func startProjectCancellationWatcher(ctx context.Context, projectID string, cancel context.CancelFunc) func() {
	if projectID == "" || cancel == nil {
		return func() {}
	}

	done := make(chan struct{})
	var once sync.Once
	go watchProjectCancellation(ctx, projectID, cancel, done)

	return func() {
		once.Do(func() {
			close(done)
		})
	}
}

func watchProjectCancellation(ctx context.Context, projectID string, cancel context.CancelFunc, done <-chan struct{}) {
	ticker := time.NewTicker(projectCancelPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-ticker.C:
			cancelled, err := projectIsCancelled(projectID)
			if err != nil {
				log.Printf("⚠️ project cancel poll failed project_id=%s err=%v", projectID, err)
				continue
			}
			if cancelled {
				log.Printf("🛑 project cancel detected by DB poll project_id=%s", projectID)
				cancel()
				return
			}
		}
	}
}
