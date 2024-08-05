package internal

// Nil 常量表示空指针
// Nil constant represents a null pointer
const Nil = 0

type (
	// Pointer 类型表示一个无符号 32 位整数，用于指向元素的位置
	// Pointer type represents an unsigned 32-bit integer used to point to the position of an element
	Pointer uint32

	// Element 结构体表示双端队列中的一个元素
	// Element struct represents an element in the deque
	Element[T any] struct {
		// prev 指向前一个元素的位置
		// prev points to the position of the previous element
		prev Pointer

		// addr 指向当前元素的位置
		// addr points to the position of the current element
		addr Pointer

		// next 指向下一个元素的位置
		// next points to the position of the next element
		next Pointer

		// value 存储元素的值
		// value stores the value of the element
		value T
	}

	// Deque 结构体表示一个双端队列
	// Deque struct represents a double-ended queue
	Deque[T any] struct {
		// head 指向队列头部元素的位置
		// head points to the position of the head element in the queue
		head Pointer

		// tail 指向队列尾部元素的位置
		// tail points to the position of the tail element in the queue
		tail Pointer

		// length 表示队列的长度
		// length represents the length of the queue
		length int

		// stack 用于存储空闲位置的栈
		// stack is used to store the stack of free positions
		stack Stack[Pointer]

		// elements 存储队列中的所有元素
		// elements stores all the elements in the queue
		elements []Element[T]

		// template 用于创建新元素的模板
		// template is used as a template for creating new elements
		template Element[T]
	}
)

// IsNil 检查指针是否为空
// IsNil checks if the pointer is null
func (c Pointer) IsNil() bool {
	return c == Nil
}

// Addr 返回元素的地址
// Addr returns the address of the element
func (c *Element[T]) Addr() Pointer {
	return c.addr
}

// Next 返回下一个元素的地址
// Next returns the address of the next element
func (c *Element[T]) Next() Pointer {
	return c.next
}

// Prev 返回前一个元素的地址
// Prev returns the address of the previous element
func (c *Element[T]) Prev() Pointer {
	return c.prev
}

// Value 返回元素的值
// Value returns the value of the element
func (c *Element[T]) Value() T {
	return c.value
}

// New 创建双端队列
// New creates a double-ended queue
func New[T any](capacity int) *Deque[T] {
	// 初始化 Deque 结构体，elements 切片的容量为 1 + capacity
	// Initialize the Deque struct, with the capacity of the elements slice set to 1 + capacity
	return &Deque[T]{elements: make([]Element[T], 1, 1+capacity)}
}

// Get 根据地址获取元素
// Get retrieves an element based on its address
func (c *Deque[T]) Get(addr Pointer) *Element[T] {
	// 如果地址大于 0，返回对应地址的元素
	// If the address is greater than 0, return the element at that address
	if addr > 0 {
		return &(c.elements[addr])
	}

	// 否则返回 nil
	// Otherwise, return nil
	return nil
}

// getElement 追加元素一定要先调用此方法, 因为追加可能会造成扩容, 地址发生变化!!!
// getElement must be called before appending elements, as appending may cause reallocation and address changes!!!
func (c *Deque[T]) getElement() *Element[T] {
	// 如果 elements 切片为空，追加一个模板元素
	// If the elements slice is empty, append a template element
	if len(c.elements) == 0 {
		c.elements = append(c.elements, c.template)
	}

	// 如果 stack 中有空闲地址，从 stack 中弹出一个地址并返回对应的元素
	// If there are free addresses in the stack, pop an address from the stack and return the corresponding element
	if c.stack.Len() > 0 {
		// 从 stack 中弹出一个空闲地址
		// Pop a free address from the stack
		addr := c.stack.Pop()

		// 获取该地址对应的元素
		// Get the element corresponding to that address
		v := c.Get(addr)

		// 设置元素的地址
		// Set the address of the element
		v.addr = addr

		// 返回该元素
		// Return the element
		return v
	}

	// 否则 stack 中没有空闲地址，计算新元素的地址
	// Otherwise, there are no free addresses in the stack, calculate the address of the new element
	addr := Pointer(len(c.elements))

	// 将模板元素追加到 elements 列表中
	// Append the template element to the elements list
	c.elements = append(c.elements, c.template)

	// 获取新元素
	// Get the new element
	v := c.Get(addr)

	// 设置新元素的地址
	// Set the address of the new element
	v.addr = addr

	// 返回新元素
	// Return the new element
	return v
}

// putElement 将元素放回空闲栈中，并重置元素内容
// putElement puts the element back into the free stack and resets the element's content
func (c *Deque[T]) putElement(ele *Element[T]) {
	// 将元素的地址压入空闲栈中
	// Push the element's address into the free stack
	c.stack.Push(ele.addr)

	// 将元素重置为模板元素
	// Reset the element to the template element
	*ele = c.template
}

// Reset 重置双端队列
// Reset resets the deque
func (c *Deque[T]) Reset() {
	// 调用内部方法 autoReset 进行重置
	// Call the internal method autoReset to reset
	c.autoReset()
}

// autoReset 内部方法，重置双端队列的状态
// autoReset is an internal method that resets the state of the deque
func (c *Deque[T]) autoReset() {
	// 重置头部、尾部指针和长度
	// Reset the head, tail pointers, and length
	c.head, c.tail, c.length = Nil, Nil, 0

	// 清空空闲栈
	// Clear the free stack
	c.stack = c.stack[:0]

	// 保留 elements 列表中的第一个元素，清空其他元素
	// Keep the first element in the elements list, clear other elements
	c.elements = c.elements[:1]
}

// Len 返回双端队列的长度
// Len returns the length of the deque
func (c *Deque[T]) Len() int {
	return c.length
}

// Front 返回队列头部的元素
// Front returns the element at the front of the queue
func (c *Deque[T]) Front() *Element[T] {
	return c.Get(c.head)
}

// Back 返回队列尾部的元素
// Back returns the element at the back of the queue
func (c *Deque[T]) Back() *Element[T] {
	return c.Get(c.tail)
}

// PushFront 将一个元素添加到队列的头部
// PushFront adds an element to the front of the deque
func (c *Deque[T]) PushFront(value T) *Element[T] {
	// 获取一个空闲的元素
	// Get a free element
	ele := c.getElement()

	// 设置元素的值
	// Set the value of the element
	ele.value = value

	// 执行将元素推到队列头部的操作
	// Perform the operation to push the element to the front of the deque
	c.doPushFront(ele)

	// 返回该元素
	// Return the element
	return ele
}

// doPushFront 执行将元素推到队列头部的操作
// doPushFront performs the operation to push the element to the front of the deque
func (c *Deque[T]) doPushFront(ele *Element[T]) {
	// 增加队列长度
	// Increase the length of the deque
	c.length++

	// 如果队列为空，设置头部和尾部指针为新元素的地址
	// If the deque is empty, set the head and tail pointers to the new element's address
	if c.head.IsNil() {
		c.head, c.tail = ele.addr, ele.addr
		return
	}

	// 获取当前头部元素
	// Get the current head element
	head := c.Get(c.head)

	// 设置当前头部元素的前一个元素为新元素
	// Set the previous element of the current head element to the new element
	head.prev = ele.addr

	// 设置新元素的下一个元素为当前头部元素
	// Set the next element of the new element to the current head element
	ele.next = head.addr

	// 更新头部指针为新元素的地址
	// Update the head pointer to the new element's address
	c.head = ele.addr
}

// PushBack 将一个元素添加到队列的尾部
// PushBack adds an element to the back of the deque
func (c *Deque[T]) PushBack(value T) *Element[T] {
	// 获取一个空闲的元素
	// Get a free element
	ele := c.getElement()

	// 设置元素的值
	// Set the value of the element
	ele.value = value

	// 执行将元素推到队列尾部的操作
	// Perform the operation to push the element to the back of the deque
	c.doPushBack(ele)

	// 返回该元素
	// Return the element
	return ele
}

// doPushBack 将元素添加到队列的尾部
// doPushBack adds an element to the back of the deque
func (c *Deque[T]) doPushBack(ele *Element[T]) {
	// 增加队列长度
	// Increase the length of the deque
	c.length++

	// 如果队列为空，设置头部和尾部指针为新元素的地址
	// If the deque is empty, set the head and tail pointers to the new element's address
	if c.tail.IsNil() {
		c.head, c.tail = ele.addr, ele.addr
		return
	}

	// 获取当前尾部元素
	// Get the current tail element
	tail := c.Get(c.tail)

	// 设置当前尾部元素的下一个元素为新元素
	// Set the next element of the current tail element to the new element
	tail.next = ele.addr

	// 设置新元素的前一个元素为当前尾部元素
	// Set the previous element of the new element to the current tail element
	ele.prev = tail.addr

	// 更新尾部指针为新元素的地址
	// Update the tail pointer to the new element's address
	c.tail = ele.addr
}

// PopFront 从队列头部弹出一个元素并返回其值
// PopFront pops an element from the front of the deque and returns its value
func (c *Deque[T]) PopFront() (value T) {
	// 获取队列头部的元素
	// Get the element at the front of the deque
	if ele := c.Front(); ele != nil {
		// 获取元素的值
		// Get the value of the element
		value = ele.value

		// 从队列中移除该元素
		// Remove the element from the deque
		c.doRemove(ele)

		// 将元素放回空闲栈中
		// Put the element back into the free stack
		c.putElement(ele)

		// 如果队列为空，重置队列
		// If the deque is empty, reset the deque
		if c.length == 0 {
			c.autoReset()
		}
	}

	// 返回弹出的元素值
	// Return the popped element's value
	return value
}

// PopBack 从队列尾部弹出一个元素并返回其值
// PopBack pops an element from the back of the deque and returns its value
func (c *Deque[T]) PopBack() (value T) {
	// 获取队列尾部的元素
	// Get the element at the back of the deque
	if ele := c.Back(); ele != nil {
		// 获取元素的值
		// Get the value of the element
		value = ele.value

		// 从队列中移除该元素
		// Remove the element from the deque
		c.doRemove(ele)

		// 将元素放回空闲栈中
		// Put the element back into the free stack
		c.putElement(ele)

		// 如果队列为空，重置队列
		// If the deque is empty, reset the deque
		if c.length == 0 {
			c.autoReset()
		}
	}

	// 返回弹出的元素值
	// Return the popped element's value
	return value
}

// InsertAfter 在指定元素之后插入一个新元素
// InsertAfter inserts a new element after the specified element
func (c *Deque[T]) InsertAfter(value T, mark Pointer) *Element[T] {
	// 如果标记指针为空，返回 nil
	// If the mark pointer is null, return nil
	if mark.IsNil() {
		return nil
	}

	// 增加队列长度
	// Increase the length of the deque
	c.length++

	// 获取一个空闲的元素
	// Get a free element
	e1 := c.getElement()

	// 获取标记的元素
	// Get the marked element
	e0 := c.Get(mark)

	// 获取标记元素的下一个元素
	// Get the next element of the marked element
	e2 := c.Get(e0.next)

	// 设置新元素的前一个元素、下一个元素和值
	// Set the previous element, next element, and value of the new element
	e1.prev, e1.next, e1.value = e0.addr, e0.next, value

	// 如果下一个元素不为空，设置其前一个元素为新元素
	// If the next element is not null, set its previous element to the new element
	if e2 != nil {
		e2.prev = e1.addr
	}

	// 设置标记元素的下一个元素为新元素
	// Set the next element of the marked element to the new element
	e0.next = e1.addr

	// 如果新元素是最后一个元素，更新尾部指针
	// If the new element is the last element, update the tail pointer
	if e1.next.IsNil() {
		c.tail = e1.addr
	}

	// 返回新插入的元素
	// Return the newly inserted element
	return e1
}

// InsertBefore 在指定元素之前插入一个新元素
// InsertBefore inserts a new element before the specified element
func (c *Deque[T]) InsertBefore(value T, mark Pointer) *Element[T] {
	// 如果标记指针为空，返回 nil
	// If the mark pointer is null, return nil
	if mark.IsNil() {
		return nil
	}

	// 增加队列长度
	// Increase the length of the deque
	c.length++

	// 获取一个空闲的元素
	// Get a free element
	e1 := c.getElement()

	// 获取标记的元素
	// Get the marked element
	e2 := c.Get(mark)

	// 获取标记元素的前一个元素
	// Get the previous element of the marked element
	e0 := c.Get(e2.prev)

	// 设置新元素的前一个元素、下一个元素和值
	// Set the previous element, next element, and value of the new element
	e1.prev, e1.next, e1.value = e2.prev, e2.addr, value

	// 如果前一个元素不为空，设置其下一个元素为新元素
	// If the previous element is not null, set its next element to the new element
	if e0 != nil {
		e0.next = e1.addr
	}

	// 设置标记元素的前一个元素为新元素
	// Set the previous element of the marked element to the new element
	e2.prev = e1.addr

	// 如果新元素是第一个元素，更新头部指针
	// If the new element is the first element, update the head pointer
	if e1.prev.IsNil() {
		c.head = e1.addr
	}

	// 返回新插入的元素
	// Return the newly inserted element
	return e1
}

// MoveToBack 将指定地址的元素移动到队列尾部
// MoveToBack moves the element at the specified address to the back of the deque
func (c *Deque[T]) MoveToBack(addr Pointer) {
	// 获取指定地址的元素
	// Get the element at the specified address
	if ele := c.Get(addr); ele != nil {
		// 从队列中移除该元素
		// Remove the element from the deque
		c.doRemove(ele)

		// 重置元素的前后指针
		// Reset the previous and next pointers of the element
		ele.prev, ele.next = Nil, Nil

		// 将元素推到队列尾部
		// Push the element to the back of the deque
		c.doPushBack(ele)
	}
}

// MoveToFront 将指定地址的元素移动到队列头部
// MoveToFront moves the element at the specified address to the front of the deque
func (c *Deque[T]) MoveToFront(addr Pointer) {
	// 获取指定地址的元素
	// Get the element at the specified address
	if ele := c.Get(addr); ele != nil {
		// 从队列中移除该元素
		// Remove the element from the deque
		c.doRemove(ele)

		// 重置元素的前后指针
		// Reset the previous and next pointers of the element
		ele.prev, ele.next = Nil, Nil

		// 将元素推到队列头部
		// Push the element to the front of the deque
		c.doPushFront(ele)
	}
}

// Update 更新指定地址的元素的值
// Update updates the value of the element at the specified address
func (c *Deque[T]) Update(addr Pointer, value T) {
	// 获取指定地址的元素
	// Get the element at the specified address
	if ele := c.Get(addr); ele != nil {
		// 更新元素的值
		// Update the value of the element
		ele.value = value
	}
}

// Remove 从队列中移除指定地址的元素
// Remove removes the element at the specified address from the deque
func (c *Deque[T]) Remove(addr Pointer) {
	// 获取指定地址的元素
	// Get the element at the specified address
	if ele := c.Get(addr); ele != nil {
		// 从队列中移除该元素
		// Remove the element from the deque
		c.doRemove(ele)

		// 将元素放回空闲栈中
		// Put the element back into the free stack
		c.putElement(ele)

		// 如果队列为空，重置队列
		// If the deque is empty, reset the deque
		if c.length == 0 {
			c.autoReset()
		}
	}
}

// doRemove 从队列中移除指定的元素
// doRemove removes the specified element from the deque
func (c *Deque[T]) doRemove(ele *Element[T]) {
	// 初始化前后元素指针为 nil
	// Initialize previous and next element pointers to nil
	var prev, next *Element[T] = nil, nil

	// 初始化状态为 0
	// Initialize state to 0
	var state = 0

	// 如果前一个元素不为空，获取前一个元素并更新状态
	// If the previous element is not nil, get the previous element and update the state
	if !ele.prev.IsNil() {
		// 使用 c.Get 方法获取 ele 的前一个元素，并将其赋值给 prev
		// Use the c.Get method to get the previous element of ele and assign it to prev
		prev = c.Get(ele.prev)

		// 将状态值 state 增加 1, 用于标记前一个元素存在
		// Increase the state value by 1, used to mark that the previous element exists
		state += 1
	}

	// 如果下一个元素不为空，获取下一个元素并更新状态
	// If the next element is not nil, get the next element and update the state
	if !ele.next.IsNil() {
		// 使用 c.Get 方法获取 ele 的下一个元素，并将其赋值给 next
		// Use the c.Get method to get the next element of ele and assign it to next
		next = c.Get(ele.next)

		// 将状态值 state 增加 2, 用于标记前后元素都存在
		// Increase the state value by 2, used to mark that both previous and next elements exist
		state += 2
	}

	// 减少队列长度
	// Decrease the length of the deque
	c.length--

	// 根据状态更新前后元素的指针
	// Update the pointers of the previous and next elements based on the state
	switch state {
	case 3:
		// 如果前后元素都存在，更新前一个元素的 next 指针和后一个元素的 prev 指针
		// If both previous and next elements exist, update the next pointer of the previous element and the prev pointer of the next element
		prev.next = next.addr
		next.prev = prev.addr
	case 2:
		// 如果只有后一个元素存在，更新后一个元素的 prev 指针并设置头部指针
		// If only the next element exists, update the prev pointer of the next element and set the head pointer
		next.prev = Nil
		c.head = next.addr
	case 1:
		// 如果只有前一个元素存在，更新前一个元素的 next 指针并设置尾部指针
		// If only the previous element exists, update the next pointer of the previous element and set the tail pointer
		prev.next = Nil
		c.tail = prev.addr
	default:
		// 如果前后元素都不存在，重置头部和尾部指针
		// If neither previous nor next elements exist, reset the head and tail pointers
		c.head = Nil
		c.tail = Nil
	}
}

// Range 遍历队列中的每个元素，并对每个元素执行给定的函数
// Range iterates over each element in the deque and executes the given function on each element
func (c *Deque[T]) Range(f func(ele *Element[T]) bool) {
	// 从队列头部开始遍历
	// Start iterating from the head of the deque
	for i := c.Get(c.head); i != nil; i = c.Get(i.next) {
		// 如果函数返回 false，则停止遍历
		// If the function returns false, stop iterating
		if !f(i) {
			break
		}
	}
}

// Clone 创建并返回队列的一个副本
// Clone creates and returns a copy of the deque
func (c *Deque[T]) Clone() *Deque[T] {
	// 创建队列的副本
	// Create a copy of the deque
	var v = *c

	// 为副本分配新的元素切片
	// Allocate a new slice for the elements of the copy
	v.elements = make([]Element[T], len(c.elements))

	// 为副本分配新的指针栈
	// Allocate a new slice for the stack of the copy
	v.stack = make([]Pointer, len(c.stack))

	// 复制元素到副本
	// Copy the elements to the copy
	copy(v.elements, c.elements)

	// 复制指针栈到副本
	// Copy the stack to the copy
	copy(v.stack, c.stack)

	// 返回副本
	// Return the copy
	return &v
}

// Stack 是一个泛型栈类型
// Stack is a generic stack type
type Stack[T any] []T

// Len 获取栈中元素的数量
// Len returns the number of elements in the stack
func (c *Stack[T]) Len() int {
	// 返回栈的长度
	// Return the length of the stack
	return len(*c)
}

// Push 将元素追加到栈顶
// Push appends an element to the top of the stack
func (c *Stack[T]) Push(v T) {
	// 将元素追加到栈顶
	// Append the element to the top of the stack
	*c = append(*c, v)
}

// Pop 从栈顶弹出元素并返回其值
// Pop removes the top element from the stack and returns its value
func (c *Stack[T]) Pop() T {
	// 获取栈的长度
	// Get the length of the stack
	n := c.Len()

	// 获取栈顶元素的值
	// Get the value of the top element
	value := (*c)[n-1]

	// 移除栈顶元素
	// Remove the top element from the stack
	*c = (*c)[:n-1]

	// 返回栈顶元素的值
	// Return the value of the top element
	return value
}
