package pugixml

import (
	"bytes"
	"iter"
)

type NodeType uint8

const (
	NodeDocument NodeType = iota
	NodeElement
	NodePCDATA
	NodeCDATA
	NodeComment
	NodePI
	NodeDeclaration
)

type Attribute struct {
	Name     []byte
	Value    []byte
	NextAttr *Attribute
}

// Attributes 是 Attribute 指针切片的便捷类型，提供常用的查询 helper
type Attributes []*Attribute

// Has reports whether attributes contains an attribute with the given name.
func (as Attributes) Has(name []byte) bool {
	for _, a := range as {
		if bytes.Equal(a.Name, name) {
			return true
		}
	}
	return false
}

// Get returns attribute value and whether it was found.
func (as Attributes) Get(name []byte) ([]byte, bool) {
	for _, a := range as {
		if bytes.Equal(a.Name, name) {
			return a.Value, true
		}
	}
	return nil, false
}

// Find returns the attribute pointer if present or nil.
func (as Attributes) Find(name []byte) *Attribute {
	for _, a := range as {
		if bytes.Equal(a.Name, name) {
			return a
		}
	}
	return nil
}

// Map applies the function f to each attribute in order. It delegates to Seq
// so callers can use an iterator abstraction.
func (as Attributes) Map() iter.Seq[*Attribute] {
	return func(yield func(*Attribute) bool) {
		for _, a := range as {
			if !yield(a) {
				return
			}
		}
	}
}

type Node struct {
	Type        NodeType
	Name        []byte
	Value       []byte
	Parent      *Node
	FirstChild  *Node
	LastChild   *Node
	NextSibling *Node
	FirstAttr   *Attribute
}

func (n *Node) AppendChild(_ *ByteArena, child *Node) {
	child.Parent = n
	if n.FirstChild == nil {
		n.FirstChild = child
	} else {
		n.LastChild.NextSibling = child
	}
	n.LastChild = child
}

func (n *Node) AppendAttr(_ *ByteArena, attr *Attribute) {
	if n.FirstAttr == nil {
		n.FirstAttr = attr
	} else {
		curr := n.FirstAttr
		for curr.NextAttr != nil {
			curr = curr.NextAttr
		}
		curr.NextAttr = attr
	}
}

// ChildNodes 返回所有子节点切片
func (n *Node) ChildNodes() []*Node {
	var nodes []*Node
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		nodes = append(nodes, child)
	}
	return nodes
}

// Attrs 返回所有属性切片（类型为 Attributes，可直接使用 .Has/.Get/.Find 等 helper）
func (n *Node) Attrs() Attributes {
	var attrs Attributes
	for attr := n.FirstAttr; attr != nil; attr = attr.NextAttr {
		attrs = append(attrs, attr)
	}
	return attrs
}

// FindChildByName 根据名称查找子节点
func (n *Node) FindChildByName(name []byte) *Node {
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if bytes.Equal(child.Name, name) {
			return child
		}
	}
	return nil
}

// GetAttr 获取属性值
func (n *Node) GetAttr(name []byte) ([]byte, bool) {
	for attr := n.FirstAttr; attr != nil; attr = attr.NextAttr {
		if bytes.Equal(attr.Name, name) {
			return attr.Value, true
		}
	}
	return nil, false
}

// String 返回节点的字符串表示（用于调试）
func (n *Node) String() string {
	var buf []byte
	n.stringify(&buf, 0)
	return string(buf)
}

func (n *Node) stringify(buf *[]byte, depth int) {
	indent := bytes.Repeat([]byte("  "), depth)

	switch n.Type {
	case NodeDocument:
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			child.stringify(buf, depth)
		}
	case NodeElement:
		*buf = append(*buf, indent...)
		*buf = append(*buf, '<')
		*buf = append(*buf, n.Name...)

		for attr := n.FirstAttr; attr != nil; attr = attr.NextAttr {
			*buf = append(*buf, ' ')
			*buf = append(*buf, attr.Name...)
			*buf = append(*buf, '=')
			*buf = append(*buf, '"')
			*buf = append(*buf, attr.Value...)
			*buf = append(*buf, '"')
		}

		if n.FirstChild == nil {
			*buf = append(*buf, '/', '>')
		} else {
			*buf = append(*buf, '>')
			*buf = append(*buf, '\n')

			for child := n.FirstChild; child != nil; child = child.NextSibling {
				child.stringify(buf, depth+1)
			}

			*buf = append(*buf, indent...)
			*buf = append(*buf, '<', '/')
			*buf = append(*buf, n.Name...)
			*buf = append(*buf, '>')
		}
		*buf = append(*buf, '\n')
	case NodePCDATA, NodeCDATA:
		if len(n.Value) > 0 {
			*buf = append(*buf, indent...)
			*buf = append(*buf, n.Value...)
			*buf = append(*buf, '\n')
		}
	case NodeComment:
		*buf = append(*buf, indent...)
		*buf = append(*buf, '<', '!', '-', '-', ' ')
		*buf = append(*buf, n.Value...)
		*buf = append(*buf, ' ', '-', '-', '>')
		*buf = append(*buf, '\n')
	case NodePI:
		*buf = append(*buf, indent...)
		*buf = append(*buf, '<', '?')
		*buf = append(*buf, n.Value...)
		*buf = append(*buf, '?', '>')
		*buf = append(*buf, '\n')
	}
}
