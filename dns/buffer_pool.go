package dns

import (
	"sync"
)

// Buffer pool for domain name operations
var domainNamePool = sync.Pool{
	New: func() any {
		b := make([]byte, maxDomainNamePresentationLength)
		return &b
	},
}

// AcquireBuffer gets a buffer from the pool
func AcquireBuffer() *[]byte {
	return domainNamePool.Get().(*[]byte)
}

// ReleaseBuffer returns a buffer to the pool
func ReleaseBuffer(b *[]byte) {
	*b = (*b)[:0]
	domainNamePool.Put(b)
}
