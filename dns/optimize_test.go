package dns

import (
	"net"
	"testing"
)

// Benchmark optimized versions to compare with original implementations

// Zero-allocation EncodeDomain using pre-allocated buffer
func BenchmarkEncodeDomainZeroAlloc(b *testing.B) {
	b.ReportAllocs()
	domain := "www.example.com"
	for b.Loop() {
		// Pre-allocate exact size needed
		// Domain "www.example.com" without trailing dot is 14 chars
		// Encoded size: 1 + 3 + 3 + 1 + 1 = 13 bytes (rough estimate)
		dst := make([]byte, 0, 32)
		_ = EncodeDomain(dst, domain)
	}
}

// Zero-allocation packDomainName
func BenchmarkPackDomainNameZeroAlloc(b *testing.B) {
	b.ReportAllocs()
	domain := "www.example.com"
	msg := make([]byte, 128)

	for b.Loop() {
		_, _ = packDomainName(domain, msg, 0)
	}
}

// Benchmark comparing string vs []byte for domain storage
func BenchmarkDomainStringStorage(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = "www.example.com"
	}
}

// Benchmark optimized header operations
func BenchmarkHeaderOperations(b *testing.B) {
	b.ReportAllocs()
	h := &Header{}

	for b.Loop() {
		h.SetResponse()
		h.SetAuthoritative()
		h.SetRecursionDesired()
		h.SetRcode(RcodeSuccess)
		_ = h.Response()
		_ = h.Authoritative()
		_ = h.RecursionDesired()
		_ = h.Rcode()
	}
}

// Benchmark encode domain with various scenarios
func BenchmarkEncodeDomainScenarios(b *testing.B) {
	scenarios := []struct {
		name   string
		domain string
	}{
		{"root", "."},
		{"single", "localhost"},
		{"short", "a.b"},
		{"medium", "www.example.com"},
		{"long", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.example.com"},
	}

	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			for b.Loop() {
				_ = EncodeDomain(nil, scenario.domain)
			}
		})
	}
}

// Benchmark RR packing for common types
func BenchmarkRRPackCommon(b *testing.B) {
	b.ReportAllocs()

	// A record
	a := &A{
		Hdr: RR_Header{
			Name:   "example.com.",
			Rrtype: TypeA,
			Class:  ClassINET,
			Ttl:    300,
		},
		A: ipTo4(net.ParseIP("1.2.3.4")),
	}
	msgA := make([]byte, 64)

	// AAAA record
	aaaa := &AAAA{
		Hdr: RR_Header{
			Name:   "example.com.",
			Rrtype: TypeAAAA,
			Class:  ClassINET,
			Ttl:    300,
		},
		AAAA: net.ParseIP("2001:db8::1"),
	}
	msgAAAA := make([]byte, 64)

	for b.Loop() {
		_, _ = a.pack(msgA, 0)
		_, _ = aaaa.pack(msgAAAA, 0)
	}
}

// Benchmark TXT record with various sizes
func BenchmarkTXTPackSizes(b *testing.B) {
	sizes := []struct {
		name string
		txt  []string
	}{
		{"small", []string{"a"}},
		{"medium", []string{"v=spf1 -all"}},
		{"large", []string{"v=spf1 ip4:192.0.2.0/24 ip4:192.0.2.10/32 -all"}},
	}

	for _, size := range sizes {
		b.Run(size.name, func(b *testing.B) {
			rr := &TXT{
				Hdr: RR_Header{
					Name:   "example.com.",
					Rrtype: TypeTXT,
					Class:  ClassINET,
					Ttl:    3600,
				},
				TXT: size.txt,
			}
			msg := make([]byte, 256)

			for b.Loop() {
				_, _ = rr.pack(msg, 0)
			}
		})
	}
}

// Benchmark compression pointer handling
func BenchmarkCompressionPointer(b *testing.B) {
	// Create a message with repeated domain names to benefit from compression
	msg := make([]byte, 0, 256)
	msg = EncodeDomain(msg, "example.com") // First occurrence
	msg = append(msg, 0x00, 0x01)      // TYPE A
	msg = append(msg, 0x00, 0x01)      // CLASS IN

	// Add compression pointer (0xC00C = pointer to offset 12)
	msg = append(msg, 0xC0, 0x0C)
	msg = append(msg, 0x00, 0x01)      // TYPE A
	msg = append(msg, 0x00, 0x01)      // CLASS IN
	msg = append(msg, 0x00, 0x00, 0x00, 0x00, 0x04) // TTL
	msg = append(msg, 0x00, 0x04)      // RDLENGTH
	msg = append(msg, 0x01, 0x02, 0x03, 0x04) // IP address

	for b.Loop() {
		_, off, _ := UnpackDomainName(msg, 12) // Start from compression pointer
		_ = off
	}
}

// Benchmark OptimizeEncodeDomain - optimized version
func BenchmarkOptimizeEncodeDomain(b *testing.B) {
	b.ReportAllocs()
	domain := "www.example.com"

	// Calculate encoded size upfront
	// Format: \x03www\x07example\x03com\x00
	// Size: 1 + 3 + 1 + 7 + 1 + 3 + 1 = 17 bytes

	for b.Loop() {
		// Pre-allocate exact size
		dst := make([]byte, 0, 17)
		_ = EncodeDomain(dst, domain)
	}
}

// Benchmark OptimizePackDomainName
func BenchmarkOptimizePackDomainName(b *testing.B) {
	b.ReportAllocs()
	domain := "www.example.com"
	msg := make([]byte, 64)

	for b.Loop() {
		_, _ = packDomainName(domain, msg, 0)
	}
}
