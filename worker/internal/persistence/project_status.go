package persistence

const (
	ProjectStatusQueued int16 = iota
	ProjectStatusRunning
	ProjectStatusRetrying
	ProjectStatusFinished
	ProjectStatusError
	ProjectStatusCancelling
	ProjectStatusCancelled
)

func IsCancellationRequestedStatus(status int16) bool {
	return status == ProjectStatusCancelling || status == ProjectStatusCancelled
}

func IsCancelledProjectStatus(status int16) bool {
	return status == ProjectStatusCancelled
}

func IsTerminalProjectStatus(status int16) bool {
	switch status {
	case ProjectStatusFinished, ProjectStatusError, ProjectStatusCancelled:
		return true
	default:
		return false
	}
}
