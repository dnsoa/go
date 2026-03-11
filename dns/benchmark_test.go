package dns

import (
	"encoding/hex"
	"net"
	"testing"
)

// Benchmarks for Header operations

func BenchmarkHeaderPack(b *testing.B) {
	h := &Header{
		ID:     0x1234,
		Bits:   0x0100,
		Qdcount: 1,
	}
	for b.Loop() {
		_ = h.Pack()
	}
}

func BenchmarkHeaderUnpack(b *testing.B) {
	h := &Header{}
	buf := []byte{0x12, 0x34, 0x01, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	for b.Loop() {
		_ = h.Unpack(buf)
	}
}

func BenchmarkHeaderSetResponse(b *testing.B) {
	h := &Header{}
	for b.Loop() {
		h.SetResponse()
	}
}

func BenchmarkHeaderSetRcode(b *testing.B) {
	h := &Header{}
	for b.Loop() {
		h.SetRcode(RcodeSuccess)
	}
}

// Benchmarks for domain operations with various inputs

func BenchmarkEncodeDomainShort(b *testing.B) {
	for b.Loop() {
		_ = EncodeDomain(nil, "a.b.c")
	}
}

func BenchmarkEncodeDomainLong(b *testing.B) {
	for b.Loop() {
		_ = EncodeDomain(nil, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.example.com")
	}
}

func BenchmarkEncodeDomainDeep(b *testing.B) {
	domain := "a.b.c.d.e.f.g.h.i.j.k.l.m.n.o.p.q.r.s.t.u.v.w.x.y.z.example.com"
	for b.Loop() {
		_ = EncodeDomain(nil, domain)
	}
}

func BenchmarkDecodeDomainShort(b *testing.B) {
	encoded := s2b("\x01a\x01b\x01c\x00")
	for b.Loop() {
		_ = DecodeDomain(encoded)
	}
}

func BenchmarkDecodeDomainLong(b *testing.B) {
	// Create a valid long domain encoding
	domain := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.example.com"
	encoded := EncodeDomain(nil, domain)
	for b.Loop() {
		_ = DecodeDomain(encoded)
	}
}

func BenchmarkDecodeDomainWithCompression(b *testing.B) {
	// Create a message with compression
	msg := make([]byte, 0, 256)
	msg = append(msg, 0x07) // example
	msg = append(msg, "example"...)
	msg = append(msg, 0x03) // com
	msg = append(msg, "com"...)
	msg = append(msg, 0x00) // root
	// Add compression pointer (0xC000 points to beginning)
	msg = append(msg, 0xC0, 0x00)

	for b.Loop() {
		_, _, _ = UnpackDomainName(msg, 13) // Start from compression pointer
	}
}

// Benchmarks for RR operations

func BenchmarkPackCNAME(b *testing.B) {
	rr := &CNAME{
		Hdr: RR_Header{
			Name:   "www.example.com.",
			Rrtype: TypeCNAME,
			Class:  ClassINET,
			Ttl:    300,
		},
		CNAME: "example.com.",
	}
	msg := make([]byte, 128)

	for b.Loop() {
		_, _ = rr.pack(msg, 0)
	}
}

func BenchmarkUnpackCNAME(b *testing.B) {
	rr := &CNAME{
		Hdr: RR_Header{
			Name:   "www.example.com.",
			Rrtype: TypeCNAME,
			Class:  ClassINET,
			Ttl:    300,
		},
		CNAME: "example.com.",
	}
	msg := make([]byte, 128)
	off, _ := rr.pack(msg, 0)
	msg = msg[:off]

	for b.Loop() {
		rr2 := &CNAME{Hdr: RR_Header{Name: "www.example.com.", Rrtype: TypeCNAME, Class: ClassINET}}
		_, _ = rr2.unpack(msg, 0)
	}
}

func BenchmarkPackMX(b *testing.B) {
	rr := &MX{
		Hdr: RR_Header{
			Name:   "mail.example.com.",
			Rrtype: TypeMX,
			Class:  ClassINET,
			Ttl:    3600,
		},
		Preference: 10,
		MX:         "mail.example.com.",
	}
	msg := make([]byte, 128)

	for b.Loop() {
		_, _ = rr.pack(msg, 0)
	}
}

func BenchmarkPackTXT(b *testing.B) {
	rr := &TXT{
		Hdr: RR_Header{
			Name:   "example.com.",
			Rrtype: TypeTXT,
			Class:  ClassINET,
			Ttl:    3600,
		},
		TXT: []string{"v=spf1 include:_spf.example.com ~all"},
	}
	msg := make([]byte, 128)

	for b.Loop() {
		_, _ = rr.pack(msg, 0)
	}
}

func BenchmarkPackTXTMulti(b *testing.B) {
	rr := &TXT{
		Hdr: RR_Header{
			Name:   "example.com.",
			Rrtype: TypeTXT,
			Class:  ClassINET,
			Ttl:    3600,
		},
		TXT: []string{"v=spf1", "include:_spf.example.com", "~all"},
	}
	msg := make([]byte, 256)

	for b.Loop() {
		_, _ = rr.pack(msg, 0)
	}
}

func BenchmarkUnpackTXT(b *testing.B) {
	rr := &TXT{
		Hdr: RR_Header{
			Name:   "example.com.",
			Rrtype: TypeTXT,
			Class:  ClassINET,
			Ttl:    3600,
		},
		TXT: []string{"v=spf1 include:_spf.example.com ~all"},
	}
	msg := make([]byte, 128)
	off, _ := rr.pack(msg, 0)
	msg = msg[:off]

	for b.Loop() {
		rr2 := &TXT{Hdr: RR_Header{Name: "example.com.", Rrtype: TypeTXT, Class: ClassINET}}
		_, _ = rr2.unpack(msg, 0)
	}
}

// Benchmark for Response unpacking (real-world scenario)

func BenchmarkResponseUnpack(b *testing.B) {
	// Real DNS response with A records
	payload, _ := hex.DecodeString("4ffd8500000100020000000105617874717303636f6d0000010001c00c0001000100000258000401010101c00c000100010000025800040303030300002904d0000000000000")

	for b.Loop() {
		resp := AcquireResponse()
		_ = resp.Unpack(payload)
		ReleaseResponse(resp)
	}
}

// Benchmark comparing EncodeDomain implementations

func BenchmarkEncodeDomainOptimized(b *testing.B) {
	// Test if pre-allocating helps
	domain := "www.example.com"
	for b.Loop() {
		dst := make([]byte, 0, 64)
		_ = EncodeDomain(dst, domain)
	}
}

// Benchmark for Request creation (hot path)

func BenchmarkAcquireRequest(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		req := AcquireRequest()
		ReleaseRequest(req)
	}
}

func BenchmarkAcquireResponse(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		resp := AcquireResponse()
		ReleaseResponse(resp)
	}
}

// Benchmark for complete request building (hot path)

func BenchmarkBuildRequest(b *testing.B) {
	b.ReportAllocs()
	cookie := []byte("f23036f16bfde3df")

	for b.Loop() {
		req := AcquireRequest()
		req.SetEDNS0(4096, true)
		req.SetEDNS0Cookie(cookie)
		req.SetQuestion("example.com", TypeA, ClassINET)
		ReleaseRequest(req)
	}
}

// Benchmark for string/bytes conversion (zero-copy)

func BenchmarkS2B(b *testing.B) {
	s := "example.com."
	b.ReportAllocs()
	for b.Loop() {
		_ = s2b(s)
	}
}

func BenchmarkB2S(b *testing.B) {
	bs := s2b("example.com.")
	b.ReportAllocs()
	for b.Loop() {
		_ = b2s(bs)
	}
}

// Benchmark for common IP operations

func BenchmarkIPv4To4(b *testing.B) {
	ip := net.ParseIP("1.2.3.4")
	for b.Loop() {
		_ = ip.To4()
	}
}

func BenchmarkIPv6To16(b *testing.B) {
	ip := net.ParseIP("2001:db8::1")
	for b.Loop() {
		_ = ip.To16()
	}
}

// Benchmark for TXT record operations (common in SPF)

func BenchmarkTXTLongString(b *testing.B) {
	rr := &TXT{
		Hdr: RR_Header{
			Name:   "example.com.",
			Rrtype: TypeTXT,
			Class:  ClassINET,
			Ttl:    3600,
		},
		TXT: []string{"v=spf1 ip4:192.0.2.0/24 ip4:192.0.2.10/32 ip4:192.0.2.11/32 ip4:192.0.2.12/31 -all"},
	}
	msg := make([]byte, 256)

	for b.Loop() {
		_, _ = rr.pack(msg, 0)
	}
}
