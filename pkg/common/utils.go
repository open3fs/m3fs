package common

// Pointer returns pointer of val
func Pointer[T any](val T) *T {
	return &val
}
