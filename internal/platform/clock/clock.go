package clock

import "time"

// Clock abstracts time for testing.
type Clock interface {
	Now() time.Time
}

// System uses the real wall clock.
type System struct{}

func (System) Now() time.Time { return time.Now().UTC() }
