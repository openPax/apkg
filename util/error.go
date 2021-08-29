package util

type ErrorString struct {
	S string
}

func (e *ErrorString) Error() string {
	return e.S
}
