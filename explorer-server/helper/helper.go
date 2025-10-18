package helper

func Deref(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

func DerefFloat(ptr *float64) float64 {
	if ptr == nil {
		return 0
	}
	return *ptr
}
