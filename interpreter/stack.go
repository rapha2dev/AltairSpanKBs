package interpreter

type Stack struct {
	parent                  *Stack
	children                []*Stack
	inheritedData           interface{}
	data, params            []interface{}
	position, paramPosition int
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

func (self *Stack) PushParam(v interface{}) {
	self.params = append(self.params, v)
	self.paramPosition++
}

func (self *Stack) PopParam() {
	self.Push(self.params[self.paramPosition])
	self.params = self.params[:self.paramPosition]
	self.paramPosition--
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
		if self.inheritedData != nil {
			return self.inheritedData
		}
		if self.paramPosition >= 0 {
			return self.params[self.paramPosition]
		}
		return nil
	}
	return self.data[self.position]
}
