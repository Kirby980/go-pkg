package pkg

import "sync"

type Queue[T any] struct {
	sync.Mutex
	items []T
}

// Enqueue 入队
func (q *Queue[T]) Enqueue(v T) {
	q.Lock()
	defer q.Unlock()
	q.items = append(q.items, v)
}

// Dequeue 出队
func (q *Queue[T]) Dequeue() T {
	q.Lock()
	defer q.Unlock()
	if len(q.items) == 0 {
		return *new(T)
	}
	v := q.items[0]
	q.items = q.items[1:]
	return v
}

// Peek 查看队首元素
func (q *Queue[T]) Peek() T {
	q.Lock()
	defer q.Unlock()
	if len(q.items) == 0 {
		return *new(T)
	}
	return q.items[0]
}

// Len 返回队列长度
func (q *Queue[T]) Len() int {
	return len(q.items)
}

// Clear 清空队列
func (q *Queue[T]) Clear() {
	q.Lock()
	defer q.Unlock()
	q.items = make([]T, 0)
}
