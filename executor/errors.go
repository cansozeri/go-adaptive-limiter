package executor

// Error represents an error from the executor.
type Error string

func (e Error) Error() string {
	return string(e)
}

// ErrRejectedExecution is returned when execution is rejected due to capacity limits.
var ErrRejectedExecution = Error("execution has been rejected")
