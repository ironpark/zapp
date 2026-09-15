package zapp

import "fmt"

type StepError struct {
	Step Step
	Err  error
}

func (e *StepError) Error() string { return fmt.Sprintf("%s: %v", e.Step, e.Err) }
func (e *StepError) Unwrap() error { return e.Err }
