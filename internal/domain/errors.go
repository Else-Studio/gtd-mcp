package domain

import "errors"

var (
	ErrNotFound   = errors.New("entity not found")
	ErrValidation = errors.New("validation failed")
)
