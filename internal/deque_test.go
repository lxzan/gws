package internal

import (
	"container/list"
	"math/rand"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
)

type Ordered interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr |
		~float32 | ~float64 |
		~string
}

func validate[T Ordered](q *Deque[T]) bool {
	var sum = 0
	for i := q.Get(q.head); i != nil; i = q.Get(i.next) {
		sum++
		next := q.Get(i.next)
		if next == nil {
			continue
		}
		if i.next != next.addr {
			return false
		}
		if next.prev != i.addr {
			return false
		}
	}

	if q.Len() != sum {
		return false
	}

	if head := q.Front(); head != nil {
		if head.prev != 0 {
			return false
		}
	}

	if tail := q.Back(); tail != nil {
		if tail.next != 0 {
			return false
		}
	}

	if q.Len() == 1 && q.Front().Value() != q.Back().Value() {
		return false
	}

	return true
}

func TestDeque_Reset(t *testing.T) {
	var q = New[int](8)
	q.PushBack(1)
	q.PushBack(2)
	q.PushBack(3)
	q.Reset()
	assert.True(t, validate(q))
	assert.Equal(t, q.Len(), 0)
}

func TestDeque_PopBack(t *testing.T) {
	var q = New[int](8)
	assert.Equal(t, q.PopBack(), 0)

	q.PushBack(1)
	assert.Equal(t, q.PopBack(), 1)
}

func TestQueue_Range(t *testing.T) {
	const count = 1000

	t.Run("", func(t *testing.T) {
		var q = New[int](0)
		var a []int
		for i := 0; i < count; i++ {
			v := rand.Intn(count)
			q.PushBack(v)
			a = append(a, v)
		}

		assert.Equal(t, q.Len(), count)

		var b []int
		q.Range(func(ele *Element[int]) bool {
			b = append(b, ele.Value())
			return len(b) < 100
		})
		assert.Equal(t, len(b), 100)

		var i = 0
		for q.Len() > 0 {
			v := q.PopFront()
			assert.Equal(t, a[i], v)
			i++
		}
	})

	t.Run("", func(t *testing.T) {
		var q = New[int](0)
		for i := 0; i < count; i++ {
			v := rand.Intn(count)
			q.PushBack(v)
		}

		var a1 []int
		var a2 []int
		for i := q.Front(); i != nil; i = q.Get(i.Next()) {
			a1 = append(a1, i.Value())
		}
		for i := q.Back(); i != nil; i = q.Get(i.Prev()) {
			a2 = append(a2, i.Value())
		}

		assert.ElementsMatch(t, a1, a2)
	})
}

func TestQueue_Addr(t *testing.T) {
	const count = 1000
	var q = New[int](0)
	for i := 0; i < count; i++ {
		v := rand.Intn(count)
		if v&7 == 0 {
			q.PopFront()
		} else {
			q.PushBack(v)
		}
	}

	var sum = 0
	for i := q.Get(q.head); i != nil; i = q.Get(i.next) {
		sum++

		prev := q.Get(i.prev)
		next := q.Get(i.next)
		if prev != nil {
			assert.Equal(t, prev.next, i.addr)
		}
		if next != nil {
			assert.Equal(t, i.addr, next.prev)
		}
	}

	assert.Equal(t, q.Len(), sum)
	if head := q.Get(q.head); head != nil {
		assert.Zero(t, head.prev)
	}
	if tail := q.Get(q.tail); tail != nil {
		assert.Zero(t, tail.next)
	}
}

func TestQueue_Pop(t *testing.T) {
	var q = New[int](0)
	assert.Zero(t, q.Front())
	assert.Zero(t, q.PopFront())

	q.PushBack(1)
	q.PushBack(2)
	q.PushBack(3)
	q.PopFront()
	q.PushBack(4)
	q.PushBack(5)
	q.PopFront()

	var arr []int
	q.Range(func(ele *Element[int]) bool {
		arr = append(arr, ele.Value())
		return true
	})
	assert.Equal(t, q.Front().Value(), 3)
	assert.True(t, IsSameSlice(arr, []int{3, 4, 5}))
	assert.Equal(t, len(q.elements), 5)
	assert.Equal(t, q.stack.Len(), 1)
}

func TestDeque_InsertAfter(t *testing.T) {
	t.Run("", func(t *testing.T) {
		var q = New[int](8)
		assert.Nil(t, q.InsertAfter(1, 0))
	})

	t.Run("", func(t *testing.T) {
		var q = New[int](8)
		q.PushBack(1)
		var node = q.PushBack(2)
		q.PushBack(4)
		q.InsertAfter(3, node.Addr())

		var arr []int
		q.Range(func(ele *Element[int]) bool {
			arr = append(arr, ele.Value())
			return true
		})

		assert.True(t, IsSameSlice(arr, []int{1, 2, 3, 4}))
		assert.True(t, validate(q))
	})

	t.Run("", func(t *testing.T) {
		var q = New[int](8)
		q.PushBack(1)
		q.PushBack(2)
		var node = q.PushBack(4)
		q.InsertAfter(3, node.Addr())

		var arr []int
		q.Range(func(ele *Element[int]) bool {
			arr = append(arr, ele.Value())
			return true
		})
		assert.True(t, IsSameSlice(arr, []int{1, 2, 4, 3}))
		assert.True(t, validate(q))
	})
}

func TestDeque_InsertBefore(t *testing.T) {
	t.Run("", func(t *testing.T) {
		var q = New[int](8)
		assert.Nil(t, q.InsertBefore(1, 0))
	})

	t.Run("", func(t *testing.T) {
		var q = New[int](8)
		q.PushBack(1)
		var node = q.PushBack(2)
		q.PushBack(4)
		q.InsertBefore(3, node.Addr())

		var arr []int
		q.Range(func(ele *Element[int]) bool {
			arr = append(arr, ele.Value())
			return true
		})

		assert.True(t, IsSameSlice(arr, []int{1, 3, 2, 4}))
		assert.True(t, validate(q))
	})

	t.Run("", func(t *testing.T) {
		var q = New[int](8)
		var node = q.PushBack(1)
		q.PushBack(2)
		q.PushBack(4)
		q.InsertBefore(3, node.Addr())

		var arr []int
		q.Range(func(ele *Element[int]) bool {
			arr = append(arr, ele.Value())
			return true
		})
		assert.True(t, IsSameSlice(arr, []int{3, 1, 2, 4}))
		assert.True(t, validate(q))
	})
}

func TestDeque_Update(t *testing.T) {
	var q = New[int](8)
	var node = q.PushBack(1)
	q.Update(node.Addr(), 2)
	assert.Equal(t, q.Get(node.Addr()).Value(), 2)
}

func TestDeque_Delete(t *testing.T) {
	t.Run("", func(t *testing.T) {
		var q = New[int](8)
		var node = q.PushBack(1)
		q.PushBack(2)
		q.PushBack(3)
		q.Remove(node.Addr())

		var arr []int
		q.Range(func(ele *Element[int]) bool {
			arr = append(arr, ele.Value())
			return true
		})
		assert.True(t, IsSameSlice(arr, []int{2, 3}))
		assert.True(t, validate(q))
	})

	t.Run("", func(t *testing.T) {
		var q = New[int](8)
		q.PushBack(1)
		var node = q.PushBack(2)
		q.PushBack(3)
		q.Remove(node.Addr())

		var arr []int
		q.Range(func(ele *Element[int]) bool {
			arr = append(arr, ele.Value())
			return true
		})
		assert.True(t, IsSameSlice(arr, []int{1, 3}))
		assert.True(t, validate(q))
	})

	t.Run("", func(t *testing.T) {
		var q = New[int](8)
		q.PushBack(1)
		q.PushBack(2)
		var node = q.PushBack(3)
		q.Remove(node.Addr())

		var arr []int
		q.Range(func(ele *Element[int]) bool {
			arr = append(arr, ele.Value())
			return true
		})
		assert.True(t, IsSameSlice(arr, []int{1, 2}))
		assert.True(t, validate(q))
	})

	t.Run("", func(t *testing.T) {
		var q = New[int](8)
		var node = q.PushBack(3)
		q.Remove(node.Addr())
		assert.Equal(t, q.Len(), 0)
		assert.True(t, validate(q))
	})
}

func TestQueue_Random(t *testing.T) {
	var count = 10000
	var q = Deque[int]{}
	var linkedlist = list.New()
	for i := 0; i < count; i++ {
		var flag = rand.Intn(13)
		var val = rand.Int()
		switch flag {
		case 0, 1:
			q.PushBack(val)
			linkedlist.PushBack(val)
		case 2, 3:
			q.PushFront(val)
			linkedlist.PushFront(val)
		case 4:
			if q.Len() > 0 {
				q.PopFront()
				linkedlist.Remove(linkedlist.Front())
			}
		case 5:
			if q.Len() > 0 {
				q.PopBack()
				linkedlist.Remove(linkedlist.Back())
			}
		case 6, 7:
			if node := q.Front(); node != nil {
				q.MoveToBack(node.Addr())
				linkedlist.MoveToBack(linkedlist.Front())
			}
		case 8:
			if node := q.Back(); node != nil {
				q.MoveToFront(node.Addr())
				linkedlist.MoveToFront(linkedlist.Back())
			}
		case 9:
			var n = rand.Intn(10)
			var index = 0
			for iter := q.Front(); iter != nil; iter = q.Get(iter.Next()) {
				index++
				if index >= n {
					q.InsertAfter(val, iter.Addr())
					break
				}
			}

			index = 0
			for iter := linkedlist.Front(); iter != nil; iter = iter.Next() {
				index++
				if index >= n {
					linkedlist.InsertAfter(val, iter)
					break
				}
			}
		case 10:
			var n = rand.Intn(10)
			var index = 0
			for iter := q.Front(); iter != nil; iter = q.Get(iter.Next()) {
				index++
				if index >= n {
					q.InsertBefore(val, iter.Addr())
					break
				}
			}

			index = 0
			for iter := linkedlist.Front(); iter != nil; iter = iter.Next() {
				index++
				if index >= n {
					linkedlist.InsertBefore(val, iter)
					break
				}
			}
		case 11, 12:
			var n = rand.Intn(10)
			var index = 0
			for iter := q.Front(); iter != nil; iter = q.Get(iter.Next()) {
				index++
				if index >= n {
					q.Remove(iter.Addr())
					break
				}
			}

			index = 0
			for iter := linkedlist.Front(); iter != nil; iter = iter.Next() {
				index++
				if index >= n {
					linkedlist.Remove(iter)
					break
				}
			}
		default:

		}
	}

	assert.True(t, validate(&q))
	for i := linkedlist.Front(); i != nil; i = i.Next() {
		var val = q.PopFront()
		assert.Equal(t, i.Value, val)
	}
}

func BenchmarkQueue_PushAndPop(b *testing.B) {
	const count = 1000
	var q = New[int](count)
	for i := 0; i < b.N; i++ {
		for j := 0; j < count/4; j++ {
			q.PushBack(j)
		}
		for j := 0; j < count/4; j++ {
			q.PopFront()
		}
		for j := 0; j < count/4; j++ {
			q.PushBack(j)
		}
		for j := 0; j < count/4; j++ {
			q.PopFront()
		}
	}
}

func TestDeque_Clone(t *testing.T) {
	var h = New[int](8)
	h.PushBack(1)
	h.PushBack(3)
	h.PushBack(2)
	h.PushBack(4)

	var h1 = h.Clone()
	var h2 = h
	assert.True(t, IsSameSlice(h.elements, h1.elements))
	var addr = (uintptr)(unsafe.Pointer(&h.elements[0]))
	var addr1 = (uintptr)(unsafe.Pointer(&h1.elements[0]))
	var addr2 = (uintptr)(unsafe.Pointer(&h2.elements[0]))
	assert.NotEqual(t, addr, addr1)
	assert.Equal(t, addr, addr2)
}

func TestDeque_PushFront(t *testing.T) {
	var q Deque[int]
	q.PushFront(1)
	q.PushFront(3)
	q.PushFront(5)
	assert.Equal(t, q.PopFront(), 5)
}
