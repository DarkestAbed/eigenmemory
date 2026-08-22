package types

import "time"

// NowUTC returns the current time in UTC. It is exposed so tests can stub it.
func NowUTC() time.Time {
	return time.Now().UTC()
}
