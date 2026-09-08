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

func validateLinks[T Ordered](q *Deque[T]) bool {
	for i := q.Get(q.head); i != nil; i = q.Get(i.next) {
		next := q.Get(i.next)
		if next == nil {
			continue
		}
		if i.next != next.addr || next.prev != i.addr {
			return false
		}
	}
	return true
}

func validateEnds[T Ordered](q *Deque[T]) bool {
	if head := q.Front(); head != nil && head.prev != 0 {
		return false
	}
	if tail := q.Back(); tail != nil && tail.next != 0 {
		return false
	}
	return true
}

func validate[T Ordered](q *Deque[T]) bool {
	var sum = 0
	for i := q.Get(q.head); i != nil; i = q.Get(i.next) {
		sum++
	}
	if q.Len() != sum {
		return false
	}
	if !validateLinks(q) {
		return false
	}
	if !validateEnds(q) {
		return false
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
		for range count {
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
		for range count {
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
	for range count {
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

func randomPushBack(q *Deque[int], ll *list.List, val int) {
	q.PushBack(val)
	ll.PushBack(val)
}

func randomPushFront(q *Deque[int], ll *list.List, val int) {
	q.PushFront(val)
	ll.PushFront(val)
}

func randomPopFront(q *Deque[int], ll *list.List) {
	if q.Len() > 0 {
		q.PopFront()
		ll.Remove(ll.Front())
	}
}

func randomPopBack(q *Deque[int], ll *list.List) {
	if q.Len() > 0 {
		q.PopBack()
		ll.Remove(ll.Back())
	}
}

func randomMoveToBack(q *Deque[int], ll *list.List) {
	if node := q.Front(); node != nil {
		q.MoveToBack(node.Addr())
		ll.MoveToBack(ll.Front())
	}
}

func randomMoveToFront(q *Deque[int], ll *list.List) {
	if node := q.Back(); node != nil {
		q.MoveToFront(node.Addr())
		ll.MoveToFront(ll.Back())
	}
}

func randomInsertAfter(q *Deque[int], ll *list.List, val int) {
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
	for iter := ll.Front(); iter != nil; iter = iter.Next() {
		index++
		if index >= n {
			ll.InsertAfter(val, iter)
			break
		}
	}
}

func randomInsertBefore(q *Deque[int], ll *list.List, val int) {
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
	for iter := ll.Front(); iter != nil; iter = iter.Next() {
		index++
		if index >= n {
			ll.InsertBefore(val, iter)
			break
		}
	}
}

func randomRemove(q *Deque[int], ll *list.List) {
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
	for iter := ll.Front(); iter != nil; iter = iter.Next() {
		index++
		if index >= n {
			ll.Remove(iter)
			break
		}
	}
}

func randomOpGroupA(q *Deque[int], ll *list.List, flag, val int) {
	switch flag {
	case 0, 1:
		randomPushBack(q, ll, val)
	case 2, 3:
		randomPushFront(q, ll, val)
	case 4:
		randomPopFront(q, ll)
	case 5:
		randomPopBack(q, ll)
	case 6:
		randomMoveToBack(q, ll)
	}
}

func randomOpGroupB(q *Deque[int], ll *list.List, flag, val int) {
	switch flag {
	case 0:
		randomMoveToBack(q, ll)
	case 1:
		randomMoveToFront(q, ll)
	case 2:
		randomInsertAfter(q, ll, val)
	case 3:
		randomInsertBefore(q, ll, val)
	case 4, 5:
		randomRemove(q, ll)
	}
}

func randomDispatch(q *Deque[int], ll *list.List, flag, val int) {
	if flag <= 6 {
		randomOpGroupA(q, ll, flag, val)
	} else {
		randomOpGroupB(q, ll, flag-7, val)
	}
}

func TestQueue_Random(t *testing.T) {
	var count = 10000
	var q = Deque[int]{}
	var linkedlist = list.New()
	for range count {
		var flag = rand.Intn(13)
		var val = rand.Int()
		randomDispatch(&q, linkedlist, flag, val)
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
	for range b.N {
		for j := range count / 4 {
			q.PushBack(j)
		}
		for range count / 4 {
			q.PopFront()
		}
		for j := range count / 4 {
			q.PushBack(j)
		}
		for range count / 4 {
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
