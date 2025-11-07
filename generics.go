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

// SliceDiffSet 计算两个切片的差集 只支持 comparable 类型
func SliceDiffSet[T comparable](src []T, dst []T) []T {
	srcMap := ToMap(src)
	for _, val := range dst {
		delete(srcMap, val)
	}
	var ret = make([]T, 0, len(srcMap))
	for key := range srcMap {
		ret = append(ret, key)
	}

	return ret
}

// SliceIntersectSet 计算两个切片的交集 只支持 comparable 类型
func SliceIntersect[T comparable](src []T, dst []T) []T {
	dstMap := ToMap(dst)
	var ret = make([]T, 0, len(src))
	for _, v := range src {
		if _, exists := dstMap[v]; exists {
			ret = append(ret, v)
		}
	}
	return ret
}

// toMap 将切片转换为 map
func ToMap[T comparable](src []T) map[T]struct{} {
	var dataMap = make(map[T]struct{}, len(src))
	for _, v := range src {
		dataMap[v] = struct{}{}
	}
	return dataMap
}
