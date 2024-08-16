package internal

const Nil = 0

type (
	Pointer uint32

	Element[T any] struct {
		prev, addr, next Pointer
		value            T
	}

	// Deque 结构体表示一个双端队列
	// Deque struct represents a double-ended queue
	Deque[T any] struct {
		// head 指向队列头部元素的位置
		// points to the position of the head element in the queue
		head Pointer

		// tail 指向队列尾部元素的位置
		// points to the position of the tail element in the queue
		tail Pointer

		// length 队列长度
		// length of the queue
		length int

		// stack 存储空闲位置的栈
		// store the stack of free positions
		stack Stack[Pointer]

		// elements 存储队列中的所有元素
		// stores all the elements in the queue
		elements []Element[T]

		// template 创建新元素的模板
		// template for creating new elements
		template Element[T]
	}
)

// IsNil 检查指针是否为空
// checks if the pointer is null
func (c Pointer) IsNil() bool {
	return c == Nil
}

// Addr 返回元素的地址
// Addr returns the address of the element
func (c *Element[T]) Addr() Pointer {
	return c.addr
}

// Next 返回下一个元素的地址
// returns the address of the next element
func (c *Element[T]) Next() Pointer {
	return c.next
}

// Prev 返回前一个元素的地址
// returns the address of the previous element
func (c *Element[T]) Prev() Pointer {
	return c.prev
}

// Value 返回元素的值
// returns the value of the element
func (c *Element[T]) Value() T {
	return c.value
}

// New 创建双端队列
// creates a double-ended queue
func New[T any](capacity int) *Deque[T] {
	return &Deque[T]{elements: make([]Element[T], 1, 1+capacity)}
}

// Get 根据地址获取元素
// retrieves an element based on its address
func (c *Deque[T]) Get(addr Pointer) *Element[T] {
	if addr > 0 {
		return &(c.elements[addr])
	}
	return nil
}

// getElement 追加元素一定要先调用此方法, 因为追加可能会造成扩容, 地址发生变化!!!
// must be called before appending elements, as appending may cause reallocation and address changes!!!
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
// resets the deque
func (c *Deque[T]) Reset() {
	c.autoReset()
}

// autoReset 重置双端队列的状态
// resets the state of the deque
func (c *Deque[T]) autoReset() {
	c.head, c.tail, c.length = Nil, Nil, 0
	c.stack = c.stack[:0]
	c.elements = c.elements[:1]
}

// Len 返回双端队列的长度
// returns the length of the deque
func (c *Deque[T]) Len() int {
	return c.length
}

// Front 返回队列头部的元素
// returns the element at the front of the queue
func (c *Deque[T]) Front() *Element[T] {
	return c.Get(c.head)
}

// Back 返回队列尾部的元素
// returns the element at the back of the queue
func (c *Deque[T]) Back() *Element[T] {
	return c.Get(c.tail)
}

// PushFront 将一个元素添加到队列的头部
// adds an element to the front of the deque
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
// adds an element to the back of the deque
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
// pops an element from the front of the deque and returns its value
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
// pops an element from the back of the deque and returns its value
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
// inserts a new element after the specified element
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
// inserts a new element before the specified element
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
// moves the element at the specified address to the back of the deque
func (c *Deque[T]) MoveToBack(addr Pointer) {
	if ele := c.Get(addr); ele != nil {
		c.doRemove(ele)
		ele.prev, ele.next = Nil, Nil
		c.doPushBack(ele)
	}
}

// MoveToFront 将指定地址的元素移动到队列头部
// moves the element at the specified address to the front of the deque
func (c *Deque[T]) MoveToFront(addr Pointer) {
	if ele := c.Get(addr); ele != nil {
		c.doRemove(ele)
		ele.prev, ele.next = Nil, Nil
		c.doPushFront(ele)
	}
}

// Update 更新指定地址的元素的值
// updates the value of the element at the specified address
func (c *Deque[T]) Update(addr Pointer, value T) {
	if ele := c.Get(addr); ele != nil {
		ele.value = value
	}
}

// Remove 从队列中移除指定地址的元素
// removes the element at the specified address from the deque
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

// Range 遍历队列中的每个元素，并对每个元素执行给定的函数
// iterates over each element in the deque and executes the given function on each element
func (c *Deque[T]) Range(f func(ele *Element[T]) bool) {
	for i := c.Get(c.head); i != nil; i = c.Get(i.next) {
		if !f(i) {
			break
		}
	}
}

// Clone 深拷贝
// deep copy
func (c *Deque[T]) Clone() *Deque[T] {
	var v = *c
	v.elements = make([]Element[T], len(c.elements))
	v.stack = make([]Pointer, len(c.stack))
	copy(v.elements, c.elements)
	copy(v.stack, c.stack)
	return &v
}

// Stack 泛型栈
// generic stack
type Stack[T any] []T

// Len 获取栈中元素的数量
// returns the number of elements in the stack
func (c *Stack[T]) Len() int {
	return len(*c)
}

// Push 将元素追加到栈顶
// appends an element to the top of the stack
func (c *Stack[T]) Push(v T) {
	*c = append(*c, v)
}

// Pop 从栈顶弹出元素并返回其值
// removes the top element from the stack and returns its value
func (c *Stack[T]) Pop() T {
	n := c.Len()
	value := (*c)[n-1]
	*c = (*c)[:n-1]
	return value
}
