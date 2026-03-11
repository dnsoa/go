package dns

import (
	"net"
	"testing"

	"github.com/dnsoa/go/assert"
)

func TestA(t *testing.T) {
	r := assert.New(t)

	// Test A record pack/unpack
	rr := &A{
		Hdr: RR_Header{
			Name:   "example.com.",
			Rrtype: TypeA,
			Class:  ClassINET,
			Ttl:    3600,
		},
		A: ipTo4(net.ParseIP("1.2.3.4")),
	}

	// Pack the RR
	msg := make([]byte, 512)
	off, err := rr.pack(msg, 0)
	r.NoError(err)
	r.Equal(4, off) // A record is 4 bytes

	// Unpack the RR
	rr2 := &A{Hdr: RR_Header{Name: "example.com.", Rrtype: TypeA, Class: ClassINET}}
	_, err = rr2.unpack(msg, 0)
	r.NoError(err)
	r.Equal("1.2.3.4", net.IP(rr2.A[:]).String())
}

func TestNS(t *testing.T) {
	r := assert.New(t)

	// Test NS record pack/unpack
	rr := &NS{
		Hdr: RR_Header{
			Name:   "example.com.",
			Rrtype: TypeNS,
			Class:  ClassINET,
			Ttl:    3600,
		},
		NS: "ns1.example.com.",
	}

	// Pack the RR
	msg := make([]byte, 512)
	off, err := rr.pack(msg, 0)
	r.NoError(err)
	t.Logf("NS pack: off=%d (expected ~17 bytes for ns1.example.com.)", off)

	// Unpack the RR (truncate to actual packed size)
	msg = msg[:off]
	rr2 := &NS{Hdr: RR_Header{Name: "example.com.", Rrtype: TypeNS, Class: ClassINET}}
	off2, err := rr2.unpack(msg, 0)
	r.NoError(err)
	t.Logf("NS unpack: off2=%d", off2)
	r.Equal(off, off2)
	r.Equal("ns1.example.com.", rr2.NS)
}

func TestCNAME(t *testing.T) {
	r := assert.New(t)

	// Test CNAME record pack/unpack
	rr := &CNAME{
		Hdr: RR_Header{
			Name:   "www.example.com.",
			Rrtype: TypeCNAME,
			Class:  ClassINET,
			Ttl:    300,
		},
		CNAME: "example.com.",
	}

	// Pack the RR
	msg := make([]byte, 512)
	off, err := rr.pack(msg, 0)
	r.NoError(err)
	t.Logf("CNAME pack: off=%d (expected ~13 bytes for example.com.)", off)

	// Unpack the RR (truncate to actual packed size)
	msg = msg[:off]
	rr2 := &CNAME{Hdr: RR_Header{Name: "www.example.com.", Rrtype: TypeCNAME, Class: ClassINET}}
	off2, err := rr2.unpack(msg, 0)
	r.NoError(err)
	t.Logf("CNAME unpack: off2=%d", off2)
	r.Equal(off, off2)
	r.Equal("example.com.", rr2.CNAME)
}

func TestMX(t *testing.T) {
	r := assert.New(t)

	// Test MX record pack/unpack
	rr := &MX{
		Hdr: RR_Header{
			Name:   "example.com.",
			Rrtype: TypeMX,
			Class:  ClassINET,
			Ttl:    3600,
		},
		Preference: 10,
		MX:         "mail.example.com.",
	}

	// Pack the RR
	msg := make([]byte, 512)
	off, err := rr.pack(msg, 0)
	r.NoError(err)
	t.Logf("MX pack: off=%d (expected ~20 bytes: 2 for preference + 18 for mail.example.com.)", off)

	// Unpack the RR (truncate to actual packed size)
	msg = msg[:off]
	rr2 := &MX{Hdr: RR_Header{Name: "example.com.", Rrtype: TypeMX, Class: ClassINET}}
	off2, err := rr2.unpack(msg, 0)
	r.NoError(err)
	t.Logf("MX unpack: off2=%d", off2)
	r.Equal(off, off2)
	r.Equal(uint16(10), rr2.Preference)
	r.Equal("mail.example.com.", rr2.MX)
}

func TestTXT(t *testing.T) {
	r := assert.New(t)

	// Test TXT record pack/unpack
	rr := &TXT{
		Hdr: RR_Header{
			Name:   "example.com.",
			Rrtype: TypeTXT,
			Class:  ClassINET,
			Ttl:    3600,
		},
		TXT: []string{"v=spf1 include:_spf.example.com ~all"},
	}

	// Pack the RR
	msg := make([]byte, 512)
	off, err := rr.pack(msg, 0)
	r.NoError(err)
	r.True(off > 0)

	// Truncate msg to actual packed size for unpacking
	msg = msg[:off]

	// Unpack the RR
	rr2 := &TXT{Hdr: RR_Header{Name: "example.com.", Rrtype: TypeTXT, Class: ClassINET}}
	off2, err := rr2.unpack(msg, 0)
	r.NoError(err)
	r.Equal(off, off2)
	r.Equal(1, len(rr2.TXT))
	r.Equal("v=spf1 include:_spf.example.com ~all", rr2.TXT[0])
}

func TestTXTMultipleStrings(t *testing.T) {
	r := assert.New(t)

	// Test TXT record with multiple strings
	rr := &TXT{
		Hdr: RR_Header{
			Name:   "example.com.",
			Rrtype: TypeTXT,
			Class:  ClassINET,
			Ttl:    3600,
		},
		TXT: []string{"hello", "world"},
	}

	// Pack the RR
	msg := make([]byte, 512)
	off, err := rr.pack(msg, 0)
	r.NoError(err)
	r.True(off > 0)

	// Truncate msg to actual packed size for unpacking
	msg = msg[:off]

	// Unpack the RR
	rr2 := &TXT{Hdr: RR_Header{Name: "example.com.", Rrtype: TypeTXT, Class: ClassINET}}
	off2, err := rr2.unpack(msg, 0)
	r.NoError(err)
	r.Equal(off, off2)
	r.Equal(2, len(rr2.TXT))
	r.Equal("hello", rr2.TXT[0])
	r.Equal("world", rr2.TXT[1])
}

func TestAAAA(t *testing.T) {
	r := assert.New(t)

	// Test AAAA record pack/unpack
	rr := &AAAA{
		Hdr: RR_Header{
			Name:   "example.com.",
			Rrtype: TypeAAAA,
			Class:  ClassINET,
			Ttl:    3600,
		},
		AAAA: net.ParseIP("2001:db8::1"),
	}

	// Pack the RR
	msg := make([]byte, 512)
	off, err := rr.pack(msg, 0)
	r.NoError(err)
	r.Equal(16, off) // AAAA record is 16 bytes

	// Unpack the RR
	rr2 := &AAAA{Hdr: RR_Header{Name: "example.com.", Rrtype: TypeAAAA, Class: ClassINET}}
	_, err = rr2.unpack(msg, 0)
	r.NoError(err)
	r.Equal("2001:db8::1", rr2.AAAA.String())
}

func TestPackRR(t *testing.T) {
	r := assert.New(t)

	// Test packing a complete RR (header + data)
	rr := &NS{
		Hdr: RR_Header{
			Name:     "example.com.",
			Rrtype:   TypeNS,
			Class:    ClassINET,
			Ttl:      3600,
			Rdlength: 0, // Will be calculated
		},
		NS: "ns1.example.com.",
	}

	msg := make([]byte, 512)
	off, err := packRR(rr, msg, 0)
	r.NoError(err)
	r.True(off > 0)
}

func TestUnpackRR(t *testing.T) {
	r := assert.New(t)

	// Test unpacking RR from a wire format message
	// NS record: example.com NS ns1.example.com
	// Wire format (starting from domain name):
	// \x07example\x03com\x00 (domain name)
	// \x00\x02 (NS type) \x00\x01 (IN class) \x00\x00\x0e\x10 (3600 TTL)
	// \x00\x11 (RDLENGTH=17) \x03ns1\x07example\x03com\x00 (RDATA)
	wire := "\x07example\x03com\x00\x00\x02\x00\x01\x00\x00\x0e\x10\x00\x11\x03ns1\x07example\x03com\x00"
	msg := s2b(wire)

	rr, off, err := UnpackRR(msg, 0)
	r.NoError(err)
	r.Equal(len(msg), off)

	// Check the unpacked RR
	ns, ok := rr.(*NS)
	r.True(ok)
	r.Equal("example.com.", ns.Hdr.Name)
	r.Equal(TypeNS, ns.Hdr.Rrtype)
	r.Equal(ClassINET, ns.Hdr.Class)
	r.Equal(uint32(3600), ns.Hdr.Ttl)
	r.Equal("ns1.example.com.", ns.NS)
}

// Helper function to pack a complete RR
func packRR(rr RR, msg []byte, off int) (int, error) {
	// Pack domain name
	off, err := packDomainName(rr.Header().Name, msg, off)
	if err != nil {
		return off, err
	}

	// Pack type
	off, err = packUint16(uint16(rr.Header().Rrtype), msg, off)
	if err != nil {
		return off, err
	}

	// Pack class
	off, err = packUint16(uint16(rr.Header().Class), msg, off)
	if err != nil {
		return off, err
	}

	// Pack TTL
	off, err = packUint32(rr.Header().Ttl, msg, off)
	if err != nil {
		return off, err
	}

	// Pack RDATA
	rdlengthOff := off
	off += 2 // Skip RDLENGTH for now

	rdStart := off
	off, err = rr.pack(msg, off)
	if err != nil {
		return off, err
	}

	// Fill in RDLENGTH
	rdlength := uint16(off - rdStart)
	_, err = packUint16(rdlength, msg, rdlengthOff)
	if err != nil {
		return off, err
	}

	return off, nil
}

func BenchmarkPackA(b *testing.B) {
	rr := &A{
		Hdr: RR_Header{
			Name:   "example.com.",
			Rrtype: TypeA,
			Class:  ClassINET,
			Ttl:    3600,
		},
		A: ipTo4(net.ParseIP("1.2.3.4")),
	}
	msg := make([]byte, 512)

	for b.Loop() {
		_, _ = rr.pack(msg, 0)
	}
}

func BenchmarkPackNS(b *testing.B) {
	rr := &NS{
		Hdr: RR_Header{
			Name:   "example.com.",
			Rrtype: TypeNS,
			Class:  ClassINET,
			Ttl:    3600,
		},
		NS: "ns1.example.com.",
	}
	msg := make([]byte, 512)

	for b.Loop() {
		_, _ = rr.pack(msg, 0)
	}
}

func BenchmarkUnpackA(b *testing.B) {
	rr := &A{
		Hdr: RR_Header{
			Name:   "example.com.",
			Rrtype: TypeA,
			Class:  ClassINET,
			Ttl:    3600,
		},
		A: ipTo4(net.ParseIP("1.2.3.4")),
	}
	msg := make([]byte, 512)
	_, _ = rr.pack(msg, 0)

	for b.Loop() {
		rr2 := &A{Hdr: RR_Header{Name: "example.com.", Rrtype: TypeA, Class: ClassINET}}
		_, _ = rr2.unpack(msg, 0)
	}
}

func TestSOA(t *testing.T) {
	r := assert.New(t)

	// Test SOA pack/unpack
	rr := &SOA{
		Hdr: RR_Header{
			Name:   "example.com.",
			Rrtype: TypeSOA,
			Class:  ClassINET,
			Ttl:    3600,
		},
		Ns:      "ns1.example.com.",
		Mbox:    "hostmaster.example.com.",
		Serial:  2023010101,
		Refresh: 3600,
		Retry:   600,
		Expire:  604800,
		Minttl:  86400,
	}

	msg := make([]byte, 512)
	off, err := rr.pack(msg, 0)
	r.NoError(err)
	r.True(off > 0)

	// Unpack
	rr2 := &SOA{Hdr: RR_Header{Name: "example.com.", Rrtype: TypeSOA, Class: ClassINET}}
	off2, err := rr2.unpack(msg, 0)
	r.NoError(err)
	r.Equal(off, off2)
	r.Equal(rr.Ns, rr2.Ns)
	r.Equal(rr.Mbox, rr2.Mbox)
	r.Equal(rr.Serial, rr2.Serial)
	r.Equal(rr.Refresh, rr2.Refresh)
	r.Equal(rr.Retry, rr2.Retry)
	r.Equal(rr.Expire, rr2.Expire)
	r.Equal(rr.Minttl, rr2.Minttl)
}

func TestPTR(t *testing.T) {
	r := assert.New(t)

	// Test PTR pack/unpack
	rr := &PTR{
		Hdr: RR_Header{
			Name:   "1.0.0.127.in-addr.arpa.",
			Rrtype: TypePTR,
			Class:  ClassINET,
			Ttl:    3600,
		},
		Ptr: "localhost.",
	}

	msg := make([]byte, 512)
	off, err := rr.pack(msg, 0)
	r.NoError(err)
	r.True(off > 0)

	// Unpack
	rr2 := &PTR{Hdr: RR_Header{Name: "1.0.0.127.in-addr.arpa.", Rrtype: TypePTR, Class: ClassINET}}
	off2, err := rr2.unpack(msg, 0)
	r.NoError(err)
	r.Equal(off, off2)
	r.Equal(rr.Ptr, rr2.Ptr)
}

func TestSRV(t *testing.T) {
	r := assert.New(t)

	// Test SRV pack/unpack
	rr := &SRV{
		Hdr: RR_Header{
			Name:   "_ldap._tcp.example.com.",
			Rrtype: TypeSRV,
			Class:  ClassINET,
			Ttl:    3600,
		},
		Priority: 10,
		Weight:   60,
		Port:     389,
		Target:   "ldap.example.com.",
	}

	msg := make([]byte, 512)
	off, err := rr.pack(msg, 0)
	r.NoError(err)
	r.True(off > 0)

	// Unpack
	rr2 := &SRV{Hdr: RR_Header{Name: "_ldap._tcp.example.com.", Rrtype: TypeSRV, Class: ClassINET}}
	off2, err := rr2.unpack(msg, 0)
	r.NoError(err)
	r.Equal(off, off2)
	r.Equal(rr.Priority, rr2.Priority)
	r.Equal(rr.Weight, rr2.Weight)
	r.Equal(rr.Port, rr2.Port)
	r.Equal(rr.Target, rr2.Target)
}
