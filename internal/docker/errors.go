package docker

import "errors"

// ErrNotFound is returned when a container does not exist.
var ErrNotFound = errors.New("sandbox not found")
