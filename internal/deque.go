package internal

const Nil = 0

type (
	Pointer uint32

	Element[T any] struct {
		prev, addr, next Pointer
		value            T
	}

	// Deque 可以不使用New函数, 声明为值类型自动初始化
	Deque[T any] struct {
		head, tail Pointer        // 头尾指针
		length     int            // 长度
		stack      Stack[Pointer] // 回收站
		elements   []Element[T]   // 元素列表
		template   Element[T]     // 空值模板
	}
)

func (c Pointer) IsNil() bool {
	return c == Nil
}

func (c *Element[T]) Addr() Pointer {
	return c.addr
}

func (c *Element[T]) Next() Pointer {
	return c.next
}

func (c *Element[T]) Prev() Pointer {
	return c.prev
}

func (c *Element[T]) Value() T {
	return c.value
}

// New 创建双端队列
func New[T any](capacity int) *Deque[T] {
	return &Deque[T]{elements: make([]Element[T], 1, 1+capacity)}
}

func (c *Deque[T]) Get(addr Pointer) *Element[T] {
	if addr > 0 {
		return &(c.elements[addr])
	}
	return nil
}

// getElement 追加元素一定要先调用此方法, 因为追加可能会造成扩容, 地址发生变化!!!
func (c *Deque[T]) getElement() *Element[T] {
	if len(c.elements) == 0 {
		c.elements = append(c.elements, c.template)
	}

	if c.stack.Len() > 0 {
		addr := c.stack.Pop()
		v := c.Get(addr)
		v.addr = addr
		return v
	}

	addr := Pointer(len(c.elements))
	c.elements = append(c.elements, c.template)
	v := c.Get(addr)
	v.addr = addr
	return v
}

func (c *Deque[T]) putElement(ele *Element[T]) {
	c.stack.Push(ele.addr)
	*ele = c.template
}

// Reset 重置
func (c *Deque[T]) Reset() {
	c.autoReset()
}

func (c *Deque[T]) autoReset() {
	c.head, c.tail, c.length = Nil, Nil, 0
	c.stack = c.stack[:0]
	c.elements = c.elements[:1]
}

func (c *Deque[T]) Len() int {
	return c.length
}

func (c *Deque[T]) Front() *Element[T] {
	return c.Get(c.head)
}

func (c *Deque[T]) Back() *Element[T] {
	return c.Get(c.tail)
}

func (c *Deque[T]) PushFront(value T) *Element[T] {
	ele := c.getElement()
	ele.value = value
	c.doPushFront(ele)
	return ele
}

func (c *Deque[T]) doPushFront(ele *Element[T]) {
	c.length++

	if c.head.IsNil() {
		c.head, c.tail = ele.addr, ele.addr
		return
	}

	head := c.Get(c.head)
	head.prev = ele.addr
	ele.next = head.addr
	c.head = ele.addr
}

func (c *Deque[T]) PushBack(value T) *Element[T] {
	ele := c.getElement()
	ele.value = value
	c.doPushBack(ele)
	return ele
}

func (c *Deque[T]) doPushBack(ele *Element[T]) {
	c.length++

	if c.tail.IsNil() {
		c.head, c.tail = ele.addr, ele.addr
		return
	}

	tail := c.Get(c.tail)
	tail.next = ele.addr
	ele.prev = tail.addr
	c.tail = ele.addr
}

func (c *Deque[T]) PopFront() (value T) {
	if ele := c.Front(); ele != nil {
		value = ele.value
		c.doRemove(ele)
		c.putElement(ele)
		if c.length == 0 {
			c.autoReset()
		}
	}
	return value
}

func (c *Deque[T]) PopBack() (value T) {
	if ele := c.Back(); ele != nil {
		value = ele.value
		c.doRemove(ele)
		c.putElement(ele)
		if c.length == 0 {
			c.autoReset()
		}
	}
	return value
}

func (c *Deque[T]) InsertAfter(value T, mark Pointer) *Element[T] {
	if mark.IsNil() {
		return nil
	}

	c.length++
	e1 := c.getElement()
	e0 := c.Get(mark)
	e2 := c.Get(e0.next)
	e1.prev, e1.next, e1.value = e0.addr, e0.next, value

	if e2 != nil {
		e2.prev = e1.addr
	}

	e0.next = e1.addr
	if e1.next.IsNil() {
		c.tail = e1.addr
	}
	return e1
}

func (c *Deque[T]) InsertBefore(value T, mark Pointer) *Element[T] {
	if mark.IsNil() {
		return nil
	}

	c.length++
	e1 := c.getElement()
	e2 := c.Get(mark)
	e0 := c.Get(e2.prev)
	e1.prev, e1.next, e1.value = e2.prev, e2.addr, value

	if e0 != nil {
		e0.next = e1.addr
	}

	e2.prev = e1.addr

	if e1.prev.IsNil() {
		c.head = e1.addr
	}
	return e1
}

func (c *Deque[T]) MoveToBack(addr Pointer) {
	if ele := c.Get(addr); ele != nil {
		c.doRemove(ele)
		ele.prev, ele.next = Nil, Nil
		c.doPushBack(ele)
	}
}

func (c *Deque[T]) MoveToFront(addr Pointer) {
	if ele := c.Get(addr); ele != nil {
		c.doRemove(ele)
		ele.prev, ele.next = Nil, Nil
		c.doPushFront(ele)
	}
}

func (c *Deque[T]) Update(addr Pointer, value T) {
	if ele := c.Get(addr); ele != nil {
		ele.value = value
	}
}

func (c *Deque[T]) Remove(addr Pointer) {
	if ele := c.Get(addr); ele != nil {
		c.doRemove(ele)
		c.putElement(ele)
		if c.length == 0 {
			c.autoReset()
		}
	}
}

func (c *Deque[T]) doRemove(ele *Element[T]) {
	var prev, next *Element[T] = nil, nil
	var state = 0
	if !ele.prev.IsNil() {
		prev = c.Get(ele.prev)
		state += 1
	}
	if !ele.next.IsNil() {
		next = c.Get(ele.next)
		state += 2
	}

	c.length--
	switch state {
	case 3:
		prev.next = next.addr
		next.prev = prev.addr
	case 2:
		next.prev = Nil
		c.head = next.addr
	case 1:
		prev.next = Nil
		c.tail = prev.addr
	default:
		c.head = Nil
		c.tail = Nil
	}
}

func (c *Deque[T]) Range(f func(ele *Element[T]) bool) {
	for i := c.Get(c.head); i != nil; i = c.Get(i.next) {
		if !f(i) {
			break
		}
	}
}

func (c *Deque[T]) Clone() *Deque[T] {
	var v = *c
	v.elements = make([]Element[T], len(c.elements))
	v.stack = make([]Pointer, len(c.stack))
	copy(v.elements, c.elements)
	copy(v.stack, c.stack)
	return &v
}

type Stack[T any] []T

// Len 获取元素数量
func (c *Stack[T]) Len() int {
	return len(*c)
}

// Push 追加元素
func (c *Stack[T]) Push(v T) {
	*c = append(*c, v)
}

// Pop 弹出元素
func (c *Stack[T]) Pop() (value T) {
	n := c.Len()
	switch n {
	case 0:
		return
	default:
		value = (*c)[n-1]
		*c = (*c)[:n-1]
		return
	}
}
