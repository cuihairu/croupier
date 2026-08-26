package schedule

import "github.com/cuihairu/croupier/internal/common/errorx"

func errBadRequest(msg string) error { return errorx.NewBadRequest(msg) }
