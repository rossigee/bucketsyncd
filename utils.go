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
			time.Sleep(time.Duration(1<<uint(i)) * time.Second) // Exponential backoff
		}
	}
	return err
}