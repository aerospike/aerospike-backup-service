package timeutil

import "time"

func MeasureDuration(f func() error) (time.Duration, error) {
	startTime := time.Now()
	err := f()
	return time.Since(startTime), err
}

func MeasureDurationValue[T any](f func() (T, error)) (T, time.Duration, error) {
	startTime := time.Now()
	val, err := f()
	return val, time.Since(startTime), err
}
