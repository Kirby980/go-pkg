package pkg

func ToOtherStruct[T any, U any](src []T, m func(idx int, src T) U) []U {
	dst := make([]U, len(src))
	for i, s := range src {
		dst[i] = m(i, s)
	}
	return dst
}

func SliceToPtrSlice[T any](src []T) []*T {
	dst := make([]*T, len(src))
	for i, s := range src {
		dst[i] = &s
	}
	return dst
}
