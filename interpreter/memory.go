package interpreter

import "fmt"

type Memory struct {
	parent   *Memory
	children []*Memory
	stacks   []*Stack
	indexes  map[string]int
	seq      int
}

func (self *Memory) OnUnreferenced() {
	for _, stack := range self.stacks {
		stack.inheritedData = nil
	}
}

func (self *Memory) Fork() *Memory {
	child := NewMemory()
	child.parent = self
	for k, v := range self.indexes {
		s := child.MakeStack(k)
		p := self.stacks[v]
		s.parent = p
		p.children = append(p.children, s)
	}
	self.children = append(self.children, child)
	return child
}

func (self *Memory) MakeStack(name string) *Stack {
	if index, has := self.indexes[name]; has {
		return self.stacks[index]
	} else {
		stack := &Stack{data: []interface{}{}, position: -1, paramPosition: -1, children: []*Stack{}}
		self.stacks = append(self.stacks, stack)
		self.indexes[name] = self.seq
		self.seq++
		for _, child := range self.children {
			s := child.MakeStack(name)
			s.parent = stack
			stack.children = append(stack.children, s)
		}
		return stack
	}
}

func (self *Memory) dump(indent string) {
	objs := 0
	inherited := 0
	for _, s := range self.stacks {
		if s.inheritedData != nil {
			inherited++
		}
		objs += len(s.data)

		//fmt.Println(self.indexes, s.data)
	}
	fmt.Printf("-> %sstacks: %d, objs: %d, inherited: %d\n", indent, len(self.stacks), objs, inherited)
}
func (self *Memory) Dump() {
	fmt.Println("\n---------------- Memory Dump")
	self.dump("")
	for _, child := range self.children {
		child.dump("	")
	}
}

func NewMemory() *Memory {
	return &Memory{indexes: map[string]int{}, stacks: []*Stack{}, children: []*Memory{}}
}
