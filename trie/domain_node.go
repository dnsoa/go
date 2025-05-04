package trie

type domainNode[T any] struct {
	parent   *domainNode[T]
	wildcard *domainNode[T]
	data     T
	children map[string]*domainNode[T]
	isLeaf   bool
}

func newDomainNode[T any]() *domainNode[T] {
	return &domainNode[T]{
		children: map[string]*domainNode[T]{},
	}
}

func (n *domainNode[T]) IsLeaf() bool {
	return n.isLeaf
}

func (n *domainNode[T]) IsEmpty() bool {
	return len(n.children) == 0
}

func (n *domainNode[T]) Data() T {
	return n.data
}
func (n *domainNode[T]) Parent() *domainNode[T] {
	return n.parent
}

func (n *domainNode[T]) MarkAsLeaf() {
	n.isLeaf = true
}

func (n *domainNode[T]) MarkAsNode() {
	n.isLeaf = false
}

func (n *domainNode[T]) SetParent(parent *domainNode[T]) {
	n.parent = parent
}

func (n *domainNode[T]) SetWildcard(wildcard *domainNode[T]) {
	n.wildcard = wildcard
}

func (n *domainNode[T]) SetData(data T) {
	n.data = data
}

func (n *domainNode[T]) Get(k string) (*domainNode[T], bool) {
	child, ok := n.children[k]
	return child, ok
}

func (n *domainNode[T]) RemoveNode(child *domainNode[T]) {
	for k, v := range n.children {
		if v == child {
			delete(n.children, k)
			if k == wildcard {
				n.wildcard = nil
			}
			break
		}
	}
}

func (n *domainNode[T]) Set(k string, new *domainNode[T]) {
	n.children[k] = new
}
