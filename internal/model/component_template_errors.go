package model

import "errors"

var (
	errComponentTemplateKeyRequired   = errors.New("component template key is required")
	errComponentTemplateBuiltinDelete = errors.New("builtin component template cannot be deleted")
)
