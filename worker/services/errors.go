package services

type NonRetryableError struct {
	Err error
}

func (e NonRetryableError) Error() string { return e.Err.Error() }
