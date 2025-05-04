package trie

import (
	"fmt"
	"iter"
	"sync"
)

const (
	separator = "."
	wildcard  = "*"
)

type DomainTree[T any] struct {
	root *domainNode[T]
	rw   sync.RWMutex
}

func NewDomainTree[T any]() *DomainTree[T] {
	return &DomainTree[T]{root: newDomainNode[T]()}
}

func (t *DomainTree[T]) Add(k string, value T) {
	t.rw.Lock()
	defer t.rw.Unlock()
	node := t.root
	for part := range splitDomainReverseIterator(k) {
		child, ok := node.Get(part)
		if !ok {
			child = newDomainNode[T]()
			child.SetParent(node)
			node.Set(part, child)
		}
		if part == wildcard {
			node.SetWildcard(child)
		}
		node = child
	}
	node.MarkAsLeaf()
	node.SetData(value)
}

func (t *DomainTree[T]) Lookup(k string) (T, bool) {
	t.rw.RLock()
	defer t.rw.RUnlock()
	node := t.findNode(k)
	if node == nil {
		var empty T
		return empty, false
	}
	return node.Data(), true
}

func (t *DomainTree[T]) findNode(k string) *domainNode[T] {
	node := t.root
	var wildcardNode *domainNode[T]
	match := 0
	total := 0
	for part := range splitDomainReverseIterator(k) {
		total++
		if node.wildcard != nil {
			wildcardNode = node.wildcard
		}
		child, found := node.Get(part)
		if found {
			match++
			node = child
		} else {
			return wildcardNode
		}
	}
	if match == total && node.IsLeaf() {
		return node
	}
	return wildcardNode
}

func (t *DomainTree[T]) Remove(k string) bool {
	t.rw.Lock()
	defer t.rw.Unlock()
	node := t.findNode(k)
	if node == nil {
		return false
	}
	var empty T
	if node.IsLeaf() {
		node.MarkAsNode()
		node.SetData(empty)
		for node.Parent() != nil && node.IsEmpty() {
			parent := node.Parent()
			parent.RemoveNode(node)
			node = parent
		}
		return true
	}
	return false
}

func (tree *DomainTree[T]) Database() iter.Seq[struct {
	Value  T
	Domain string
}] {
	return func(yield func(struct {
		Value  T
		Domain string
	}) bool) {
		var walk func(node *domainNode[T], labels []string) bool
		walk = func(node *domainNode[T], labels []string) bool {
			if node.isLeaf {
				// 域名应从 labels 反转拼接
				domain := ""
				for i := len(labels) - 1; i >= 0; i-- {
					if i != len(labels)-1 {
						domain += separator
					}
					domain += labels[i]
				}
				if !yield(struct {
					Value  T
					Domain string
				}{Domain: domain, Value: node.data}) {
					return false
				}
			}
			for label, child := range node.children {
				if !walk(child, append(labels, label)) {
					return false
				}
			}
			return true
		}
		node := tree.root
		walk(node, nil)
	}
}

func (t *DomainTree[T]) Reset() {
	t.rw.Lock()
	defer t.rw.Unlock()
	t.root = newDomainNode[T]()
}

func (t *DomainTree[T]) Print() {
	var printNode func(node *domainNode[T], label, prefix string)
	printNode = func(node *domainNode[T], label, prefix string) {
		if label != "" {
			branch := "├── "
			leafMark := ""
			if node.IsLeaf() {
				leafMark = separator
				branch = "└── "
			}
			fmt.Printf("%s%s%s%s\n", prefix, branch, label, leafMark)
			if node.IsLeaf() {
				prefix += "    "
			} else {
				prefix += "│   "
			}
		}
		for l, child := range node.children {
			printNode(child, l, prefix)
		}
	}
	node := t.root
	printNode(node, "", "")
}

func splitDomainReverseIterator(domain string) iter.Seq[string] {
	return func(yield func(string) bool) {
		start := len(domain)
		for i := len(domain) - 1; i >= 0; i-- {
			if domain[i] == '.' {
				part := domain[i+1 : start]
				if part != "" {
					if !yield(part) {
						return
					}
				}
				start = i
			}
		}
		if start > 0 {
			part := domain[0:start]
			if !yield(part) {
				return
			}
		}
	}
}
