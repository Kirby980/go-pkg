package pkg

// ToOtherStruct 函数将一个类型为 T 的切片转换为另一个类型为 U 的切片
func ToOtherStruct[T any, U any](src []T, m func(idx int, src T) U) []U {
	dst := make([]U, len(src))
	for i, s := range src {
		dst[i] = m(i, s)
	}
	return dst
}

// 将切片转换为指针切片
func SliceToPtrSlice[T any](src []T) []*T {
	dst := make([]*T, len(src))
	for i, s := range src {
		dst[i] = &s
	}
	return dst
}
