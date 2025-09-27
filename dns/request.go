package dns

import (
	"encoding/binary"
	"errors"
	"math/rand/v2"
	"net"
	"net/netip"

	"github.com/dnsoa/go/pool"
)

// A Name is a non-encoded and non-escaped domain name. It is used instead of strings to avoid
// allocations.
type Name struct {
	Data   [255]byte
	Length uint8
}

type Request struct {
	OPT      OPT
	Raw      []byte
	Domain   []byte
	Question Question
	Header   Header
}

var requestPool = pool.NewPool(func() *Request {
	req := new(Request)
	req.Raw = make([]byte, 0, 512)
	req.OPT.Options = make([]Option, 0, 8)
	return req
})

// AcquireRequest returns new dns request.
func AcquireRequest() *Request {
	return requestPool.Get()
}

// ReleaseRequest returnes the dns request to the pool.
func ReleaseRequest(msg *Request) {
	requestPool.Put(msg)
}

func (r *Request) SetEDNS0Cookie(cookie []byte) {
	r.OPT.AddOption(OptionCodeCookie, cookie)
}

func (r *Request) SetEDNS0NSID(nsid string) {
	r.OPT.AddOption(OptionCodeNSID, []byte(nsid))
}

func (r *Request) SetEDNS0Padding(size int) {
	r.OPT.AddOption(OptionCodePadding, make([]byte, size))
}

func (r *Request) SetEDNS0Chain(chain []byte) {
	r.OPT.AddOption(OptionCodeChain, chain)
}

func (r *Request) SetEDNS0Keepalive(timeout uint16) {
	r.OPT.AddOption(OptionCodeEDNSKeepAlive, []byte{byte(timeout >> 8), byte(timeout)})
}

/*
//	e.Code = dns.EDNS0SUBNET // by default this is filled in through unpacking OPT packets (unpackDataOpt)
//	e.Family = 1	// 1 for IPv4 source address, 2 for IPv6
//	e.SourceNetmask = 32	// 32 for IPV4, 128 for IPv6
//	e.SourceScope = 0
//	e.Address = net.ParseIP("127.0.0.1").To4()	// for IPv4
//	// e.Address = net.ParseIP("2001:7b8:32a::2")	// for IPV6
*/
func (r *Request) SetEDNS0ClientSubnet(clientSubnet netip.Prefix) error {
	var family uint16
	if clientSubnet.Addr().Is4() {
		family = 1
	} else {
		family = 2
	}
	b := make([]byte, 4)
	binary.BigEndian.PutUint16(b[0:], family)
	sourceNetmask := uint8(clientSubnet.Bits())
	b[2] = sourceNetmask
	b[3] = 0
	switch family {
	case 0:
		// "dig" sets AddressFamily to 0 if SourceNetmask is also 0
		// We might don't need to complain either
		if sourceNetmask != 0 {
			return errors.New("dns: bad address family")
		}
	case 1:
		if sourceNetmask > net.IPv4len*8 {
			return errors.New("dns: bad netmask")
		}
		needLength := (sourceNetmask + 8 - 1) / 8 // division rounding up
		ip := clientSubnet.Addr().As4()
		b = append(b, ip[:needLength]...)
	case 2:
		ip := clientSubnet.Addr().As16()
		if sourceNetmask > net.IPv6len*8 {
			return errors.New("dns: bad netmask")
		}
		if len(ip) != net.IPv6len {
			return errors.New("dns: bad address")
		}
		needLength := (sourceNetmask + 8 - 1) / 8 // division rounding up
		b = append(b, ip[:needLength]...)
	default:
		return errors.New("dns: bad address family")
	}
	r.OPT.AddOption(OptionCodeEDNSClientSubnet, b)
	return nil
}

func (r *Request) SetEDNS0(maxSize uint16, do bool) {
	r.OPT = OPT{
		Hdr: RR_Header{
			Name:   ".",
			Rrtype: TypeOPT,
			Class:  Class(maxSize),
		},
	}
	if do {
		r.OPT.Hdr.Ttl |= _DO
	} else {
		r.OPT.Hdr.Ttl &^= _DO
	}
}

func (r *Request) SetQuestion(domain string, typ Type, class Class) {
	r.Header.ID = uint16(rand.Uint32N(65536))
	r.Header.SetRecursionDesired()
	r.Header.SetAuthenticatedData()
	r.Header.Qdcount = 1
	if r.OPT.Hdr.Class != 0 {
		r.Header.Arcount = 1
	}

	hdr := r.Header.Pack()
	r.Raw = r.Raw[:0]
	r.Raw = append(r.Raw, hdr[:]...)
	// QNAME
	r.Raw = EncodeDomain(r.Raw, domain)
	r.Question.Name = r.Raw[headerSize : headerSize+len(domain)+2]
	r.Domain = s2b(domain)
	// QTYPE
	r.Raw = append(r.Raw, byte(typ>>8), byte(typ))
	r.Question.Type = typ
	// QCLASS
	r.Raw = append(r.Raw, byte(class>>8), byte(class))
	r.Question.Class = class
	if r.OPT.Hdr.Class == 0 {
		return
	}
	//OPT
	optHdr := r.OPT.Pack()
	r.Raw = append(r.Raw, optHdr...)
	for _, o := range r.OPT.Options {
		r.Raw = append(r.Raw, byte(o.Code>>8), byte(o.Code), byte(o.Length>>8), byte(o.Length))
		r.Raw = append(r.Raw, o.Data...)
	}
}

func (r *Request) Unpack(payload []byte) error {
	if err := r.Header.Unpack(payload); err != nil {
		return err
	}

	if r.Header.Qdcount != 1 {
		return ErrInvalidHeader
	}
	// QNAME
	payload = payload[12:]
	var i int
	var b byte
	for i, b = range payload {
		if b == 0 {
			break
		}
	}
	//each question size should be atleast 4 bytes long (2 byte QType + 2 byte QClass)
	if i == 0 || i+5 > len(payload) {
		return ErrInvalidQuestion
	}
	r.Question.Name = payload[:i+1]
	payload = payload[i:]
	// QTYPE
	r.Question.Type = Type(binary.BigEndian.Uint16(payload[1:3]))
	// QCLASS
	r.Question.Class = Class(binary.BigEndian.Uint16(payload[3:5]))
	// Domain
	i = int(r.Question.Name[0])
	domain := append(r.Domain[:0], r.Question.Name[1:]...)
	for domain[i] != 0 {
		j := int(domain[i])
		domain[i] = '.'
		i += j + 1
	}
	r.Domain = domain[:len(domain)-1]
	payload = payload[5:]
	if len(payload) == 0 {
		return nil
	}
	//OPT
	return r.OPT.Unpack(payload)
}

func (r *Request) Reset() {
	r.OPT = OPT{}
	r.Raw = r.Raw[:0]
	r.Domain = r.Domain[:0]
	r.Question = Question{}
	r.Header = Header{}
}
