package interpreter

type ScopeInstance struct {
	parent *ScopeInstance
	scope  *ScopeBuilder
	data   []interface{}
}

func (self *ScopeInstance) Set(index int, v interface{}) {
	self.data[index] = v
}

func (self *ScopeInstance) Find(name string) interface{} {
	if i, h := self.scope.indexes[name]; h {
		if self.data[i] != nil {
			return self.data[i]
		}
	}
	if self.parent != nil {
		return self.parent.Find(name)
	}
	return nil
}

func (self *ScopeInstance) Value(index int, scope *ScopeBuilder) interface{} {
	if scope == self.scope {
		return self.data[index]
	}
	return nil
}

func (self *ScopeInstance) Child(scope *ScopeBuilder) *ScopeInstance {
	return &ScopeInstance{parent: self, scope: scope, data: make([]interface{}, scope.seq)}
}

// -------------------

type ScopeBuilder struct {
	indexes map[string]int
	seq     int

	// closure
	body         *Baked
	paramIndexes []int
	memoize      *Memoize
}

func (self *ScopeBuilder) Register(name string) int {
	if index, has := self.indexes[name]; has {
		return index
	} else {
		self.indexes[name] = self.seq
		defer func() { self.seq++ }()
		return self.seq
	}
}

func (self *ScopeBuilder) New() *ScopeInstance {
	return &ScopeInstance{scope: self, data: make([]interface{}, self.seq)}
}

func newScopeBuilder() *ScopeBuilder {
	return &ScopeBuilder{indexes: map[string]int{}}
}
