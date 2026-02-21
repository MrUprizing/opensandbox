package docker

import "errors"

// ErrNotFound is returned when a container does not exist.
var ErrNotFound = errors.New("sandbox not found")

// ErrImageNotFound is returned when an image does not exist locally.
var ErrImageNotFound = errors.New("image not found locally")
