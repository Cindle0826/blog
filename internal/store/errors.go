package store

import "errors"

var (
	ErrNotFound     = errors.New("not found")
	ErrSlugConflict = errors.New("slug conflict")
	ErrSlugLocked   = errors.New("slug locked")
)
