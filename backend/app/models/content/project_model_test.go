package content

import "testing"

func TestProjectStatusNameIncludesCancelling(t *testing.T) {
	if got := ProjectStatusName(ProjectStatusCancelling); got != "cancelling" {
		t.Fatalf("unexpected status name: %s", got)
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
