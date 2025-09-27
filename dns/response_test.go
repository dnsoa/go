package dns

import (
	"encoding/hex"
	"net"
	"testing"

	"github.com/dnsoa/go/assert"
)

func TestResponse(t *testing.T) {
	r := assert.New(t)
	// _ = r
	resp := new(Response)
	// resp.Header = new(Header)
	resp.Header.ID = 0x4ffd
	resp.Header.SetResponse()
	r.True(resp.Header.Response())
	resp.Header.SetAuthoritative()
	r.True(resp.Header.Authoritative())
	resp.Header.SetRecursionDesired()
	r.True(resp.Header.RecursionDesired())
	resp.Header.SetRcode(RcodeSuccess)
	r.Equal(RcodeSuccess, resp.Header.Rcode())
	resp.Header.Qdcount = 1
	resp.Header.Ancount = 2
	resp.Header.Arcount = 1
	resp.Header.Nscount = 0

	resp.SetQuestion("axtqs.com", TypeA, ClassINET)
	// cookie, err := hex.DecodeString("f23036f16bfde3df")
	// r.NoError(err)
	// resp.SetEDNS0(4096, true)
	// // resp.SetEDNS0Cookie(cookie)
	t.Logf("%x", resp.Header.Bits)
	t.Logf("%s", resp.Header.String())
	t.Logf("%x", resp.Header.Pack())
	t.Logf("%x", EncodeDomain(nil, "axtqs.com"))
	resp.Answer = make([]RR, 2)
	resp.Answer[0] = &A{
		Hdr: RR_Header{
			Name:   "axtqs.com",
			Rrtype: TypeA,
			Class:  ClassINET,
			Ttl:    600,
		},
		A: net.IPv4(1, 1, 1, 1).To4(),
	}
	resp.Answer[1] = &A{
		Hdr: RR_Header{
			Name:   "axtqs.com",
			Rrtype: TypeA,
			Class:  ClassINET,
			Ttl:    600,
		},
		A: net.IPv4(3, 3, 3, 3).To4(),
	}
	t.Logf("%x", resp.Pack())
	//4ffd85000001000200000001
	//4ffd8500000100020000000105617874717303636f6d0000010001c00c0001000100000258000401010101c00c000100010000025800040303030300002904d0000000000000

}

func TestResponseUnpack(t *testing.T) {
	r := assert.New(t)
	payload, _ := hex.DecodeString("4ffd8500000100020000000105617874717303636f6d0000010001c00c0001000100000258000401010101c00c000100010000025800040303030300002904d0000000000000")
	/*
			Domain Name System (response)
		    Transaction ID: 0x4ffd
		    Flags: 0x8500 Standard query response, No error
		        1... .... .... .... = Response: Message is a response
		        .000 0... .... .... = Opcode: Standard query (0)
		        .... .1.. .... .... = Authoritative: Server is an authority for domain
		        .... ..0. .... .... = Truncated: Message is not truncated
		        .... ...1 .... .... = Recursion desired: Do query recursively
		        .... .... 0... .... = Recursion available: Server can't do recursive queries
		        .... .... .0.. .... = Z: reserved (0)
		        .... .... ..0. .... = Answer authenticated: Answer/authority portion was not authenticated by the server
		        .... .... ...0 .... = Non-authenticated data: Unacceptable
		        .... .... .... 0000 = Reply code: No error (0)
		    Questions: 1
		    Answer RRs: 2
		    Authority RRs: 0
		    Additional RRs: 1
		    Queries
		        axtqs.com: type A, class IN
		            Name: axtqs.com
		            [Name Length: 9]
		            [Label Count: 2]
		            Type: A (Host Address) (1)
		            Class: IN (0x0001)
		    Answers
		        axtqs.com: type A, class IN, addr 1.1.1.1
		            Name: axtqs.com
		            Type: A (Host Address) (1)
		            Class: IN (0x0001)
		            Time to live: 600 (10 minutes)
		            Data length: 4
		            Address: 1.1.1.1
		        axtqs.com: type A, class IN, addr 3.3.3.3
		            Name: axtqs.com
		            Type: A (Host Address) (1)
		            Class: IN (0x0001)
		            Time to live: 600 (10 minutes)
		            Data length: 4
		            Address: 3.3.3.3
		    Additional records
		        <Root>: type OPT
		            Name: <Root>
		            Type: OPT (41)
		            UDP payload size: 1232
		            Higher bits in extended RCODE: 0x00
		            EDNS0 version: 0
		            Z: 0x0000
		                0... .... .... .... = DO bit: Cannot handle DNSSEC security RRs
		                .000 0000 0000 0000 = Reserved: 0x0000
		            Data length: 0
		    [Request In: 16]
		    [Time: 0.111483988 seconds]
	*/
	resp := AcquireResponse()
	r.NoError(resp.Unpack(payload))
	r.Equal(uint16(0x4ffd), resp.Header.ID)
	r.Equal(uint16(0x8500), resp.Header.Bits)
	r.Equal(Opcode(0), resp.Header.OpCode())
	r.Equal(uint16(1), resp.Header.Qdcount)
	r.Equal(uint16(2), resp.Header.Ancount)
	r.Equal(uint16(1), resp.Header.Arcount)
	r.Equal(uint16(0), resp.Header.Nscount)
	r.True(resp.Header.Response())
	r.True(resp.Header.RecursionDesired())
	r.False(resp.Header.Truncated())
	r.False(resp.Header.Zero())
	r.False(resp.Header.AuthenticatedData())
	r.True(resp.Header.Authoritative())
	r.Equal("axtqs.com.", b2s(resp.Question.Name))
	r.Equal(TypeA, resp.Question.Type)
	r.Equal(ClassINET, resp.Question.Class)
	r.Equal(2, len(resp.Answer))
	r.Equal("axtqs.com.", resp.Answer[0].Header().Name)
	r.Equal(TypeA, resp.Answer[0].Header().Rrtype)
	r.Equal(ClassINET, resp.Answer[0].Header().Class)
	r.Equal(uint32(600), resp.Answer[0].Header().Ttl)
	r.Equal(net.IPv4(1, 1, 1, 1).To4(), resp.Answer[0].(*A).A)
	r.Equal("axtqs.com.", resp.Answer[1].Header().Name)
	r.Equal(TypeA, resp.Answer[1].Header().Rrtype)
	r.Equal(ClassINET, resp.Answer[1].Header().Class)
	r.Equal(uint32(600), resp.Answer[1].Header().Ttl)
	r.Equal(net.IPv4(3, 3, 3, 3).To4(), resp.Answer[1].(*A).A)
	// r.Equal(0, len(resp.Ns))
	r.Equal(1, len(resp.Extra))
	r.Equal(TypeOPT, resp.Extra[0].Header().Rrtype)
	r.Equal(uint16(1232), uint16(resp.Extra[0].Header().Class))
	opt := resp.Extra[0].(*OPT)
	r.Equal(uint32(0), opt.Hdr.Ttl)
	// r.Equal(OptionCodeCookie, opt.Options[0].Code)
}
