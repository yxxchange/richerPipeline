package sort

import (
	"testing"
)

func TestStack(t *testing.T) {
	// 创建一个新的栈
	stack := Stack{}

	// 测试栈是否为空
	if !stack.IsEmpty() {
		t.Errorf("Expected stack to be empty, but it wasn't")
	}

	// 测试推入元素
	stack.Push(1)
	stack.Push(2)
	stack.Push(3)

	// 测试查看顶部元素
	if top := stack.Peek(); top != 3 {
		t.Errorf("Expected top element to be 3, but got %v", top)
	}

	// 测试弹出元素
	if popped := stack.Pop(); popped != 3 {
		t.Errorf("Expected popped element to be 3, but got %v", popped)
	}

	// 测试栈是否为空
	if stack.IsEmpty() {
		t.Errorf("Expected stack not to be empty, but it was")
	}

	// 测试推入和弹出多个元素
	stack.Push(4)
	stack.Push(5)
	for _, value := range []int{5, 4, 2, 1} {
		if popped := stack.Pop(); popped != value {
			t.Errorf("Expected popped element to be %v, but got %v", value, popped)
		}
	}

	// 测试栈是否为空
	if !stack.IsEmpty() {
		t.Errorf("Expected stack to be empty, but it wasn't")
	}

	// 测试在空栈上弹出元素
	if popped := stack.Pop(); popped != nil {
		t.Errorf("Expected popped element to be nil, but got %v", popped)
	}

	// 测试在空栈上查看顶部元素
	if top := stack.Peek(); top != nil {
		t.Errorf("Expected top element to be nil, but got %v", top)
	}
}
