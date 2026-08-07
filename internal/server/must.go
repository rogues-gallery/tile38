package server

func Must[T any](a T, err error) T {
	if err != nil {
		panic(err)
	}
	return a
}

func Default[T comparable](a, b T) T {
	var c T
	if a == c {
		return b
	}
	return a
}
