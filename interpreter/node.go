package interpreter

type Node struct {
	execs    int
	executor func() interface{}
}

func (self *Node) execute(ch chan interface{}) {
	ch <- self.executor()
}

func (self *Node) Execute() interface{} {
	if self.execs == 1000 {
		self.execs = 0
		ch := make(chan interface{})
		go self.execute(ch)
		return <-ch
	} else {
		self.execs++
		return self.executor()
	}
}

func newNode(executor func() interface{}) *Node {
	return &Node{executor: executor}
}
