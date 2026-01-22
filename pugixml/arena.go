package pugixml

import (
	"unsafe"
)

const (
	pageSize  = 32 * 1024 // 32KB, 类似 pugixml
	alignment = 8
	alignMask = ^(alignment - 1)
)

type ByteArena struct {
	pages [][]byte
	cur   int // 当前 page 已使用的偏移量
	page  int // 当前 page 索引
}

func NewArena() *ByteArena {
	return &ByteArena{
		pages: [][]byte{make([]byte, pageSize)},
		cur:   0,
		page:  0,
	}
}

// Alloc 分配 n 字节并确保 8 字节对齐
func (a *ByteArena) Alloc(size int) unsafe.Pointer {
	// 对齐处理
	alignedSize := (size + alignment - 1) & alignMask

	if a.cur+alignedSize > pageSize {
		// 分配新 Page
		newPage := make([]byte, pageSize)
		a.pages = append(a.pages, newPage)
		a.page = len(a.pages) - 1
		a.cur = 0
	}

	ptr := unsafe.Pointer(&a.pages[a.page][a.cur])
	a.cur += alignedSize
	return ptr
}

// AllocNode 在 Arena 中分配一个 Node 结构体
func AllocNode(a *ByteArena) *Node {
	p := a.Alloc(int(unsafe.Sizeof(Node{})))
	return (*Node)(p)
}

// AllocAttr 在 Arena 中分配一个 Attribute 结构体
func AllocAttr(a *ByteArena) *Attribute {
	p := a.Alloc(int(unsafe.Sizeof(Attribute{})))
	return (*Attribute)(p)
}

// InternBytes 将处理后的字节持久化到 Arena
func (a *ByteArena) InternBytes(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	ptr := a.Alloc(len(b))
	// 使用 unsafe.Slice 而不是大型数组类型转换
	dest := unsafe.Slice((*byte)(ptr), len(b))
	copy(dest, b)
	return dest
}
