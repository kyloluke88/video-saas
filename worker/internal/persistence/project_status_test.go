package persistence

import "testing"

func TestIsCancellationRequestedStatus(t *testing.T) {
	if !IsCancellationRequestedStatus(ProjectStatusCancelling) {
		t.Fatal("cancelling should be treated as cancellation requested")
	}
	if !IsCancellationRequestedStatus(ProjectStatusCancelled) {
		t.Fatal("cancelled should be treated as cancellation requested")
	}
	if IsCancellationRequestedStatus(ProjectStatusRunning) {
		t.Fatal("running should not be treated as cancellation requested")
	}
}

func TestIsTerminalProjectStatusExcludesCancelling(t *testing.T) {
	if IsTerminalProjectStatus(ProjectStatusCancelling) {
		t.Fatal("cancelling should not be terminal")
	}
	if !IsTerminalProjectStatus(ProjectStatusCancelled) {
		t.Fatal("cancelled should be terminal")
	}
}
