package dns

import (
	"encoding/hex"
	"testing"

	"github.com/dnsoa/go/assert"
)

func TestRequest(t *testing.T) {
	// r := require.New(t)
	req := &Request{}
	// cookie, err := hex.DecodeString("f23036f16bfde3df")
	// r.NoError(err)
	req.SetEDNS0(4096, true)
	// req.SetEDNS0Cookie(cookie)
	req.SetQuestion("t3n.de", TypeTXT, ClassINET)
	t.Logf("%x", req.Raw)
}

func TestRequestUnpack(t *testing.T) {
	r := assert.New(t)
	msg, _ := hex.DecodeString("4ffd0120000100000000000105617874717303636f6d0000010001000029100000000000000c000a000874b82f2641563c8e")
	/*
			Domain Name System (query)
		    Transaction ID: 0x4ffd
		    Flags: 0x0120 Standard query
		    Questions: 1
		    Answer RRs: 0
		    Authority RRs: 0
		    Additional RRs: 1
		    Queries
		    Additional records
		        <Root>: type OPT
		            Name: <Root>
		            Type: OPT (41)
		            UDP payload size: 4096
		            Higher bits in extended RCODE: 0x00
		            EDNS0 version: 0
		            Z: 0x0000
		                0... .... .... .... = DO bit: Cannot handle DNSSEC security RRs
		                .000 0000 0000 0000 = Reserved: 0x0000
		            Data length: 12
		            Option: COOKIE
		                Option Code: COOKIE (10)
		                Option Length: 8
		                Option Data: 74b82f2641563c8e
		                Client Cookie: 74b82f2641563c8e
		                Server Cookie: <MISSING>
		    [Response In: 17]
	*/
	req := AcquireRequest()
	r.NoError(req.Unpack(msg))
	r.Equal(uint16(0x4ffd), req.Header.ID)
	r.Equal(uint16(0x0120), req.Header.Bits)
	r.Equal(Opcode(0), req.Header.OpCode())
	r.Equal(uint16(1), req.Header.Qdcount)
	r.Equal(uint16(0), req.Header.Ancount)
	r.Equal(uint16(1), req.Header.Arcount)
	r.Equal(uint16(0), req.Header.Nscount)
	r.False(req.Header.Response())
	r.True(req.Header.RecursionDesired())
	r.False(req.Header.Truncated())
	r.False(req.Header.Zero())
	r.True(req.Header.AuthenticatedData())
	r.False(req.Header.Authoritative())
	r.Equal("\x05axtqs\x03com\x00", b2s(req.Question.Name))
	r.Equal(TypeA, req.Question.Type)
	r.Equal(ClassINET, req.Question.Class)
	r.Equal(s2b("axtqs.com"), req.Domain)
	//OPT
	r.Equal("", req.OPT.Hdr.Name)
	r.Equal(TypeOPT, req.OPT.Hdr.Rrtype)
	r.Equal(uint16(4096), uint16(req.OPT.Hdr.Class))
	r.Equal(uint32(0), req.OPT.Hdr.Ttl)
	r.Equal(uint16(12), req.OPT.Hdr.Rdlength)
	r.Equal(1, len(req.OPT.Options))
	r.Equal(OptionCodeCookie, req.OPT.Options[0].Code)
	r.Equal(uint16(8), req.OPT.Options[0].Length)
	cookie, _ := hex.DecodeString("74b82f2641563c8e")
	r.Equal(cookie, req.OPT.Options[0].Data)

}

func BenchmarkRequestMessage(b *testing.B) {
	req := AcquireRequest()
	defer ReleaseRequest(req)
	cookie := []byte("f23036f16bfde3df")

	for b.Loop() {
		req.SetEDNS0(4096, true)
		req.SetEDNS0Cookie(cookie)
		req.SetQuestion("t3n.de", TypeTXT, ClassINET)
	}
}
