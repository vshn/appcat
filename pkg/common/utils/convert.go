package utils

func ToInt32(v any) int32 {
	switch p := v.(type) {
	case int64:
		return int32(p)
	case float64:
		return int32(p)
	case int:
		return int32(p)
	default:
		return 0
	}
}
