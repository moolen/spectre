package integrationadmin

type Error struct {
	Code    string
	Message string
	Err     error
}

func (e *Error) Error() string {
	return e.Message
}

func newError(code, message string, err error) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Err:     err,
	}
}
