package internal

const Nil = 0

type (
	Pointer uint32

	Element[T any] struct {
		prev, addr, next Pointer
		value            T
	}

	// Deque 双端队列
	Deque[T any] struct {
		// head 指向队列头部元素的位置
		head Pointer

		// tail 指向队列尾部元素的位置
		tail Pointer

		// length 队列长度
		length int

		// stack 存储空闲位置的栈
		stack Stack[Pointer]

		// elements 存储队列中的所有元素
		elements []Element[T]

		// template 创建新元素的模板
		template Element[T]
	}
)

// IsNil 检查指针是否为空
func (c Pointer) IsNil() bool {
	return c == Nil
}

// Addr 返回元素的地址
func (c *Element[T]) Addr() Pointer {
	return c.addr
}

// Next 返回下一个元素的地址
func (c *Element[T]) Next() Pointer {
	return c.next
}

// Prev 返回前一个元素的地址
func (c *Element[T]) Prev() Pointer {
	return c.prev
}

// Value 返回元素的值
func (c *Element[T]) Value() T {
	return c.value
}

// New 创建双端队列
func New[T any](capacity int) *Deque[T] {
	return &Deque[T]{elements: make([]Element[T], 1, 1+capacity)}
}

// Get 根据地址获取元素
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

// Reset 重置双端队列
func (c *Deque[T]) Reset() {
	c.autoReset()
}

// autoReset 重置双端队列的状态
func (c *Deque[T]) autoReset() {
	c.head, c.tail, c.length = Nil, Nil, 0
	c.stack = c.stack[:0]
	c.elements = c.elements[:1]
}

// Len 返回双端队列的长度
func (c *Deque[T]) Len() int {
	return c.length
}

// Front 返回队列头部的元素
func (c *Deque[T]) Front() *Element[T] {
	return c.Get(c.head)
}

// Back 返回队列尾部的元素
func (c *Deque[T]) Back() *Element[T] {
	return c.Get(c.tail)
}

// PushFront 将一个元素添加到队列的头部
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

// PushBack 将一个元素添加到队列的尾部
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

// PopFront 从队列头部弹出一个元素并返回其值
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

// PopBack 从队列尾部弹出一个元素并返回其值
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

// InsertAfter 在指定元素之后插入一个新元素
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

// InsertBefore 在指定元素之前插入一个新元素
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

// MoveToBack 将指定地址的元素移动到队列尾部
func (c *Deque[T]) MoveToBack(addr Pointer) {
	if ele := c.Get(addr); ele != nil {
		c.doRemove(ele)
		ele.prev, ele.next = Nil, Nil
		c.doPushBack(ele)
	}
}

// MoveToFront 将指定地址的元素移动到队列头部
func (c *Deque[T]) MoveToFront(addr Pointer) {
	if ele := c.Get(addr); ele != nil {
		c.doRemove(ele)
		ele.prev, ele.next = Nil, Nil
		c.doPushFront(ele)
	}
}

// Update 更新指定地址的元素的值
func (c *Deque[T]) Update(addr Pointer, value T) {
	if ele := c.Get(addr); ele != nil {
		ele.value = value
	}
}

// Remove 从队列中移除指定地址的元素
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
	const removeHasPrev = 1
	const removeHasNext = 2
	var state = 0
	if !ele.prev.IsNil() {
		prev = c.Get(ele.prev)
		state += removeHasPrev
	}
	if !ele.next.IsNil() {
		next = c.Get(ele.next)
		state += removeHasNext
	}

	c.length--
	switch state {
	case removeHasPrev | removeHasNext:
		prev.next = next.addr
		next.prev = prev.addr
	case removeHasNext:
		next.prev = Nil
		c.head = next.addr
	case removeHasPrev:
		prev.next = Nil
		c.tail = prev.addr
	default:
		c.head = Nil
		c.tail = Nil
	}
}

// Range 遍历队列中的每个元素，并对每个元素执行给定的函数
func (c *Deque[T]) Range(f func(ele *Element[T]) bool) {
	for i := c.Get(c.head); i != nil; i = c.Get(i.next) {
		if !f(i) {
			break
		}
	}
}

// Clone 深拷贝
func (c *Deque[T]) Clone() *Deque[T] {
	var v = *c
	v.elements = make([]Element[T], len(c.elements))
	v.stack = make([]Pointer, len(c.stack))
	copy(v.elements, c.elements)
	copy(v.stack, c.stack)
	return &v
}

// Stack 泛型栈
type Stack[T any] []T

// Len 获取栈中元素的数量
func (c *Stack[T]) Len() int {
	return len(*c)
}

// Push 将元素追加到栈顶
func (c *Stack[T]) Push(v T) {
	*c = append(*c, v)
}

// Pop 从栈顶弹出元素并返回其值
func (c *Stack[T]) Pop() T {
	n := c.Len()
	value := (*c)[n-1]
	*c = (*c)[:n-1]
	return value
}
