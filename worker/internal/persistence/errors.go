package persistence

type FatalError struct {
	Err error
}

func (e FatalError) Error() string {
	if e.Err == nil {
		return "fatal persistence error"
	}
	return e.Err.Error()
}

func (e FatalError) Unwrap() error {
	return e.Err
}

func wrapFatal(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := err.(FatalError); ok {
		return err
	}
	return FatalError{Err: err}
}
