package dns

import (
	"encoding/binary"
	"net"
	"strconv"
	"unsafe"

	"github.com/dnsoa/go/pool"
)

type Question struct {
	// Name refers to the raw query name to be resolved in the query.
	//
	// +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
	// |                                               |
	// /                     QNAME                     /
	// /                                               /
	// +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
	Name []byte

	// Type specifies the type of the query to perform.
	//
	// +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
	// |                     QTYPE                     |
	// +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
	Type Type

	// Class specifies the class of the query to perform.
	//
	// +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
	// |                     QCLASS                    |
	// +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
	Class Class
}

type RR_Header struct {
	Name     string `dns:"cdomain-name"`
	Rrtype   Type
	Class    Class
	Ttl      uint32
	Rdlength uint16 // Length of data after header.
}

func (h *RR_Header) Header() *RR_Header { return h }
func (h *RR_Header) pack(msg []byte, off int) (off1 int, err error) {
	// RR_Header has no RDATA to pack.
	return off, nil
}
func (h *RR_Header) unpack(msg []byte, off int) (int, error) {
	panic("dns: internal error: unpack should never be called on RR_Header")
}
func (h *RR_Header) String() string {
	var s string

	if h.Rrtype == TypeOPT {
		s = ";"
		// and maybe other things
	}

	s += sprintName(h.Name) + "\t"
	s += strconv.FormatInt(int64(h.Ttl), 10) + "\t"
	s += Class(h.Class).String() + "\t"
	s += Type(h.Rrtype).String() + "\t"
	return s
}

type RR interface {
	Header() *RR_Header
	pack(msg []byte, off int) (off1 int, err error)
	unpack(msg []byte, off int) (off1 int, err error)
	String() string
}

type Response struct {
	Answer []RR
	Ns     []RR
	Extra  []RR
	// Question holds the question section of the response message.
	Question Question
	// Header is the wire format for the DNS packet header.
	Header Header
}

var responsePool = pool.NewPool(func() *Response {
	resp := new(Response)
	resp.Answer = make([]RR, 0, 8)
	resp.Ns = make([]RR, 0, 8)
	resp.Extra = make([]RR, 0, 8)
	return resp
})

// AcquireResponse returns new dns response.
func AcquireResponse() *Response {
	return responsePool.Get()
}

// ReleaseResponse returnes the dns response to the pool.
func ReleaseResponse(msg *Response) {
	responsePool.Put(msg)
}

func s2b(s string) []byte {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

func b2s(b []byte) string {
	return unsafe.String(unsafe.SliceData(b), len(b))
}
func (r *Response) SetQuestion(name string, typ Type, class Class) {
	r.Question.Name = s2b(name)
	r.Question.Type = typ
	r.Question.Class = class
}

func (r *Response) Pack() []byte {
	var buf []byte
	hdr := r.Header.Pack()
	buf = append(buf, hdr[:]...)

	buf = append(buf, EncodeDomain(nil, b2s(r.Question.Name))...)
	buf = append(buf, byte(r.Question.Type>>8), byte(r.Question.Type))
	buf = append(buf, byte(r.Question.Class>>8), byte(r.Question.Class))
	// for _, rr := range r.Answer {
	// 	buf = append(buf, rr.Pack()...)
	// }
	return buf
}

func (r *Response) Unpack(payload []byte) error {
	if err := r.Header.Unpack(payload); err != nil {
		return err
	}

	if r.Header.Qdcount != 1 {
		return ErrInvalidHeader
	}
	q, off, err := unpackQuestion(payload, headerSize)
	if err != nil {
		return err
	}
	r.Question = q

	r.Answer, off, err = unpackRRslice(int(r.Header.Ancount), payload, off)
	if err != nil {
		return err
	}

	r.Ns, off, err = unpackRRslice(int(r.Header.Nscount), payload, off)
	if err != nil {
		return err
	}
	r.Extra, _, err = unpackRRslice(int(r.Header.Arcount), payload, off)
	if err != nil {
		return err
	}

	// for i := uint16(0); i < r.Header.Arcount; i++ {
	// 	rr, _, err := r.unpackRR(payload, off)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	r.Extra = append(r.Extra, rr)
	// 	// payload = payload[off:]
	// }

	return nil
}

func unpackQuestion(msg []byte, off int) (Question, int, error) {
	var (
		q   Question
		err error
	)
	q.Name, off, err = UnpackDomainName(msg, off)
	if err != nil {
		return q, off, err
	}
	if len(msg) < off+4 {
		return q, off, ErrInvalidQuestion
	}
	q.Type = Type(binary.BigEndian.Uint16(msg[off : off+2]))
	off += 2
	q.Class = Class(binary.BigEndian.Uint16(msg[off : off+2]))
	off += 2
	return q, off, err
}

func (r *Response) unpackRR(data []byte, off int) (RR, int, error) {
	var rr RR
	var name []byte
	var err error
	if len(data) < 11 {
		return rr, 0, ErrInvalidRR
	}
	name, off, err = UnpackDomainName(data, off)
	if err != nil {
		return rr, off, err
	}
	typ := Type(binary.BigEndian.Uint16(data[off : off+2]))
	off += 2
	class := Class(binary.BigEndian.Uint16(data[off : off+2]))
	off += 2
	ttl := binary.BigEndian.Uint32(data[off : off+4])
	off += 4
	rdlength := binary.BigEndian.Uint16(data[off : off+2])
	off += 2
	rrHdr := RR_Header{
		Name:     b2s(name),
		Rrtype:   typ,
		Class:    class,
		Ttl:      ttl,
		Rdlength: rdlength,
	}
	switch typ {
	case TypeA:
		ip := net.IP(data[off : off+int(rdlength)])
		rr = &A{
			Hdr: rrHdr,
			A:   ip,
		}
		off += int(rdlength)
	case TypeOPT:
		rr = &OPT{
			Hdr: rrHdr,
		}
		off += int(rdlength)

	}

	// hdr := rr.Header()
	return rr, off, nil
}

func (r *Response) Reset() {
	r.Answer = r.Answer[:0]
	r.Extra = r.Extra[:0]
	r.Question = Question{}
}
