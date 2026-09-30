package middlewares

import "math"

// SessionUserID rejects negative, fractional and overflowing session values.
func SessionUserID(raw any) (uint, bool) {
	var id uint
	switch v := raw.(type) {
	case uint:
		id = v
	case int:
		if v <= 0 {
			return 0, false
		}
		id = uint(v)
	case int64:
		if v <= 0 || uint64(uint(v)) != uint64(v) {
			return 0, false
		}
		id = uint(v)
	case float64:
		if v <= 0 || math.IsNaN(v) || math.IsInf(v, 0) || math.Trunc(v) != v || v > 9007199254740991 || v >= float64(^uint(0)) {
			return 0, false
		}
		id = uint(v)
	default:
		return 0, false
	}
	return id, id != 0
}
