package dns

import (
	"encoding/binary"
	"net"
	"strconv"
	"unsafe"

	"github.com/dnsoa/go/sync"
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

var responsePool = sync.NewPool(func() *Response {
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
	msg.Reset()
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
	// Calculate approximate size needed
	size := 512 // Header + Question + some RRs
	for _, rr := range r.Answer {
		if rr != nil {
			size += 128
		}
	}
	for _, rr := range r.Ns {
		if rr != nil {
			size += 128
		}
	}
	for _, rr := range r.Extra {
		if rr != nil {
			size += 128
		}
	}

	buf := make([]byte, size)
	off := 0

	// Pack header
	hdr := r.Header.Pack()
	copy(buf[off:], hdr[:])
	off += headerSize

	// Create compression map for domain names
	compression := make(map[string]int)

	// Pack question section
	off, _ = packDomainNameWithCompression(b2s(r.Question.Name), buf, off, compression)
	buf[off] = byte(r.Question.Type >> 8)
	buf[off+1] = byte(r.Question.Type)
	buf[off+2] = byte(r.Question.Class >> 8)
	buf[off+3] = byte(r.Question.Class)
	off += 4

	// Helper function to ensure buffer has enough space
	ensureSpace := func(needed int) {
		if off+needed > len(buf) {
			newBuf := make([]byte, len(buf)*2)
			copy(newBuf, buf)
			buf = newBuf
		}
	}

	// Helper function to pack an RR
	packRR := func(rr RR) {
		if rr == nil {
			return
		}
		h := rr.Header()

		// Pack RR name with compression
		off, _ = packDomainNameWithCompression(h.Name, buf, off, compression)

		// Ensure space for fixed part: Type(2) + Class(2) + TTL(4) + RDLENGTH(2) = 10 bytes
		ensureSpace(10)

		// Pack RR type, class, ttl
		buf[off] = byte(h.Rrtype >> 8)
		buf[off+1] = byte(h.Rrtype)
		buf[off+2] = byte(h.Class >> 8)
		buf[off+3] = byte(h.Class)
		buf[off+4] = byte(h.Ttl >> 24)
		buf[off+5] = byte(h.Ttl >> 16)
		buf[off+6] = byte(h.Ttl >> 8)
		buf[off+7] = byte(h.Ttl)
		off += 8

		// Reserve space for RDLENGTH
		rdlengthOff := off
		off += 2

		// Pack RDATA
		rdataStart := off
		off, _ = rr.pack(buf, off)

		// Fill in RDLENGTH
		rdlength := uint16(off - rdataStart)
		buf[rdlengthOff] = byte(rdlength >> 8)
		buf[rdlengthOff+1] = byte(rdlength)
	}

	// Pack answer section
	for _, rr := range r.Answer {
		packRR(rr)
	}

	// Pack authority section
	for _, rr := range r.Ns {
		packRR(rr)
	}

	// Pack additional section
	for _, rr := range r.Extra {
		packRR(rr)
	}

	return buf[:off]
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

	r.Answer, off, err = unpackRRslice(int(r.Header.Ancount), payload, off, r.Answer)
	if err != nil {
		return err
	}

	r.Ns, off, err = unpackRRslice(int(r.Header.Nscount), payload, off, r.Ns)
	if err != nil {
		return err
	}
	r.Extra, _, err = unpackRRslice(int(r.Header.Arcount), payload, off, r.Extra)
	if err != nil {
		return err
	}

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
		rr = &A{
			Hdr: rrHdr,
		}
		copy(rr.(*A).A[:], data[off:off+net.IPv4len])
		off += net.IPv4len
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
	r.Ns = r.Ns[:0]
	r.Extra = r.Extra[:0]
	r.Question = Question{}
	r.Header = Header{}
}
