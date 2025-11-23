package game

import (
	"errors"
	"strconv"
)

var (
	// ErrInvalidRequest is returned when the request is invalid
	ErrInvalidRequest = errors.New("invalid request")
	// ErrInvalidID is returned when the ID cannot be parsed
	ErrInvalidID = errors.New("invalid ID")
)

// parseID converts a string ID to uint
func parseID(id string) (uint, error) {
	val, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return 0, ErrInvalidID
	}
	return uint(val), nil
}
