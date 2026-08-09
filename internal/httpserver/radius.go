package httpserver

import (
	"strconv"
)

func parseRadius(raw string) (float64, error) {
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, err
	}
	if v <= 0 {
		return 0, strconv.ErrSyntax
	}
	if v > 50 {
		v = 50
	}
	return v, nil
}
