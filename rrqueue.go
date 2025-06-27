package pkg

import "sync"

type RrQueue[T any] struct {
	// 环形队列
	items []T
	// 队首
	head int
	// 队尾
	tail int
	// 队列大小
	size int
	// 互斥锁
	mu sync.Mutex
}

// Enqueue 入队
func (q *RrQueue[T]) Enqueue(v T) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.size == len(q.items) {
		q.resize()
	}
	q.items[q.tail] = v
	q.tail = (q.tail + 1) % q.size
	q.size++
}

// resize 扩容
// size为0改为10，size大小小于256就扩大两倍 否则扩大1.25倍
func (q *RrQueue[T]) resize() {
	var newSize int
	if q.size == 0 {
		newSize = 10
	} else if q.size < 256 {
		newSize = q.size * 2
	} else {
		newSize = q.size * 5 / 4
	}
	newItems := make([]T, newSize)
	for i := 0; i < q.size; i++ {
		newItems[i] = q.items[(q.head+i)%q.size]
	}
	q.items = newItems
	q.head = 0
	q.tail = q.size
	q.size = newSize
}

// Dequeue 出队
func (q *RrQueue[T]) Dequeue() T {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.size == 0 {
		return *new(T)
	}
	v := q.items[q.head]
	q.head = (q.head + 1) % q.size
	q.size--
	return v
}

// Peek 查看队首元素
func (q *RrQueue[T]) Peek() T {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.size == 0 {
		return *new(T)
	}
	return q.items[q.head]
}

// Len 返回队列长度
func (q *RrQueue[T]) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.size
}

// Clear 清空队列
func (q *RrQueue[T]) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = make([]T, 0)
	q.head = 0
	q.tail = 0
	q.size = 0
}
