package dns

import (
	"testing"

	"github.com/dnsoa/go/assert"
)

func TestEncodeDomain(t *testing.T) {
	r := assert.New(t)

	tests := []struct {
		name     string
		domain   string
		expected string
	}{
		{
			name:     "simple domain",
			domain:   "example.com",
			expected: "\x07example\x03com\x00",
		},
		{
			name:     "subdomain",
			domain:   "www.example.com",
			expected: "\x03www\x07example\x03com\x00",
		},
		{
			name:     "root",
			domain:   ".",
			expected: "\x00",
		},
		{
			name:     "single label",
			domain:   "localhost",
			expected: "\x09localhost\x00",
		},
		{
			name:     "long label",
			domain:   "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.example.com",
			expected: string([]byte{0x3e}) + "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" + "\x07example\x03com\x00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EncodeDomain(nil, tt.domain)
			r.Equal(tt.expected, string(result))
		})
	}
}

func TestDecodeDomainAll(t *testing.T) {
	r := assert.New(t)

	tests := []struct {
		name     string
		encoded  string
		expected string
	}{
		{
			name:     "simple domain",
			encoded:  "\x07example\x03com\x00",
			expected: "example.com",
		},
		{
			name:     "subdomain",
			encoded:  "\x03www\x07example\x03com\x00",
			expected: "www.example.com",
		},
		{
			name:     "root",
			encoded:  "\x00",
			expected: ".",
		},
		{
			name:     "single label",
			encoded:  "\x09localhost\x00",
			expected: "localhost",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DecodeDomain(s2b(tt.encoded))
			r.Equal(tt.expected, string(result))
		})
	}
}

func TestPackDomainName(t *testing.T) {
	r := assert.New(t)

	tests := []struct {
		name   string
		domain string
	}{
		{"simple", "example.com"},
		{"subdomain", "www.example.com"},
		{"root", "."},
		{"single", "localhost"},
		{"deep", "a.b.c.d.e.f.g.h.i.j.k.l.m.n.o.p.q.r.s.t.u.v.w.x.y.z.example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := make([]byte, 512)
			off, err := packDomainName(tt.domain, msg, 0)
			r.NoError(err)
			r.True(off > 0)

			// Verify by unpacking
			unpacked, off2, err := UnpackDomainName(msg, 0)
			r.NoError(err)
			r.Equal(off, off2)

			expected := tt.domain
			if expected != "." {
				expected += "."
			}
			r.Equal(expected, string(unpacked))
		})
	}
}

func TestUnpackDomainName(t *testing.T) {
	r := assert.New(t)

	tests := []struct {
		name     string
		encoded  string
		expected string
	}{
		{
			name:     "simple",
			encoded:  "\x07example\x03com\x00",
			expected: "example.com.",
		},
		{
			name:     "subdomain",
			encoded:  "\x03www\x07example\x03com\x00",
			expected: "www.example.com.",
		},
		{
			name:     "root",
			encoded:  "\x00",
			expected: ".",
		},
		{
			name:     "single label",
			encoded:  "\x09localhost\x00",
			expected: "localhost.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use writable buffer for fast path optimization (in-place modification)
			msg := []byte(tt.encoded)
			result, off, err := UnpackDomainName(msg, 0)
			r.NoError(err)
			r.Equal(len(msg), off)
			r.Equal(tt.expected, string(result))
		})
	}
}

func TestUnpackDomainNameWithCompression(t *testing.T) {
	r := assert.New(t)

	// Create a message with compression pointers
	// The second domain name should use a pointer to the first
	msg := make([]byte, 0, 128)
	// First domain: example.com
	msg = append(msg, 0x07)
	msg = append(msg, "example"...)
	msg = append(msg, 0x03)
	msg = append(msg, "com"...)
	msg = append(msg, 0x00)
	// Second domain uses pointer to first (0xC00C points to offset 12)
	// But for this test, let's point to the beginning of example.com (offset 0)
	msg = append(msg, 0xC0)
	msg = append(msg, 0x00) // Pointer to offset 0

	// Unpack first domain
	result1, off1, err := UnpackDomainName(msg, 0)
	r.NoError(err)
	r.Equal("example.com.", string(result1))
	r.Equal(13, off1) // 7 + 1 + 3 + 1 + 1

	// Unpack second domain (with compression)
	result2, off2, err := UnpackDomainName(msg, off1)
	r.NoError(err)
	r.Equal("example.com.", string(result2))
	r.Equal(15, off2) // off1 + 2 (compression pointer is 2 bytes)
}

func TestSplitDomainName(t *testing.T) {
	r := assert.New(t)

	tests := []struct {
		name     string
		domain   string
		expected []string
	}{
		{
			name:     "simple",
			domain:   "example.com",
			expected: []string{"example", "com"},
		},
		{
			name:     "subdomain",
			domain:   "www.example.com",
			expected: []string{"www", "example", "com"},
		},
		{
			name:     "root",
			domain:   ".",
			expected: []string{},
		},
		{
			name:     "empty",
			domain:   "",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitDomainName(tt.domain)
			r.DeepEqual(tt.expected, result)
		})
	}
}

func BenchmarkEncodeDomain(b *testing.B) {
	for b.Loop() {
		_ = EncodeDomain(nil, "www.example.com")
	}
}

func BenchmarkDecodeDomain(b *testing.B) {
	encoded := EncodeDomain(nil, "www.example.com")
	b.ResetTimer()

	for b.Loop() {
		_ = DecodeDomain(encoded)
	}
}

func BenchmarkPackDomainName(b *testing.B) {
	msg := make([]byte, 512)
	for b.Loop() {
		_, _ = packDomainName("www.example.com", msg, 0)
	}
}

func BenchmarkUnpackDomainName(b *testing.B) {
	msg := make([]byte, 512)
	off, _ := packDomainName("www.example.com", msg, 0)
	msg = msg[:off]
	b.ResetTimer()

	for b.Loop() {
		_, _, _ = UnpackDomainName(msg, 0)
	}
}
