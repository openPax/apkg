package cmd

type errorString struct {
	s string
}
	
func (e *errorString) Error() string {
	return e.s
}