package main

import (
	"time"
)

// RetryOperation retries an operation up to maxRetries times with exponential backoff
func RetryOperation(operation func() error, maxRetries int) error {
	var err error
	for i := 0; i < maxRetries; i++ {
		err = operation()
		if err == nil {
			return nil
		}
		if i < maxRetries-1 {
			// Multiply rather than bit-shift to avoid int→uint conversion flagged by G115.
			delay := time.Second
			for j := 0; j < i; j++ {
				delay *= 2
			}
			time.Sleep(delay)
		}
	}
	return err
}
