package interpreter

type Stack struct {
	parent        *Stack
	children      []*Stack
	inheritedData interface{}
	data          []interface{}
	position      int
	isDone        bool
}

func (self *Stack) Pop() {
	if len(self.children) > 0 {
		v := self.Value()
		for _, c := range self.children {
			c.inheritedData = v
		}
	}
	self.data = self.data[:self.position]
	self.position--
}

func (self *Stack) Push(v interface{}) {
	self.data = append(self.data, v)
	self.position++
}

func (self *Stack) Value() interface{} {
	if self.position == -1 {
		if self.parent != nil {
			if v := self.parent.Value(); v != nil {
				return v
			}
		}
		return self.inheritedData
	}
	return self.data[self.position]
}
