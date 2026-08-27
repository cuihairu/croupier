package configexplorer

import (
	"fmt"
	"strconv"
)

func parseID(raw string) (uint, error) {
	v, err := strconv.ParseUint(raw, 10, strconv.IntSize)
	if err != nil || v == 0 {
		return 0, fmt.Errorf("invalid id")
	}
	return uint(v), nil
}
