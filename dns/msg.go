package dns

import (
	"errors"
	"fmt"
	"sync"
)

const (
	maxCompressionOffset    = 2 << 13 // We have 14 bits for the compression pointer
	maxDomainNameWireOctets = 255     // See RFC 1035 section 2.3.4

	// This is the maximum number of compression pointers that should occur in a
	// semantically valid message. Each label in a domain name must be at least one
	// octet and is separated by a period. The root label won't be represented by a
	// compression pointer to a compression pointer, hence the -2 to exclude the
	// smallest valid root label.
	//
	// It is possible to construct a valid message that has more compression pointers
	// than this, and still doesn't loop, by pointing to a previous pointer. This is
	// not something a well written implementation should ever do, so we leave them
	// to trip the maximum compression pointer check.
	maxCompressionPointers = (maxDomainNameWireOctets+1)/2 - 2

	// This is the maximum length of a domain name in presentation format. The
	// maximum wire length of a domain name is 255 octets (see above), with the
	// maximum label length being 63. The wire format requires one extra byte over
	// the presentation format, reducing the number of octets by 1. Each label in
	// the name will be separated by a single period, with each octet in the label
	// expanding to at most 4 bytes (\DDD). If all other labels are of the maximum
	// length, then the final label can only be 61 octets long to not exceed the
	// maximum allowed wire length.
	maxDomainNamePresentationLength = 61*4 + 1 + 63*4 + 1 + 63*4 + 1 + 63*4 + 1
)

var (
	ErrBuf        error = errors.New("buffer size too small") // ErrBuf indicates that the buffer used is too small for the message.
	ErrLongDomain error = fmt.Errorf("domain name exceeded %d wire-format octets", maxDomainNameWireOctets)
	ErrRdata      error = errors.New("dns: invalid rdata in message")
)

func UnpackDomainName(msg []byte, off int) ([]byte, int, error) {
	start := off
	end := off
	off1 := 0
	lenmsg := len(msg)
	budget := maxDomainNameWireOctets
	ptr := 0 // number of pointers followed

	// First pass: check if we can use fast path (no special chars or compression)
	fastPath := true
	off = start
Loop:
	for {
		if off >= lenmsg {
			return nil, lenmsg, ErrBuf
		}
		c := int(msg[off])
		off++
		switch c & 0xC0 {
		case 0x00:
			if c == 0x00 {
				break Loop
			}
			if off+c > lenmsg {
				return nil, lenmsg, ErrBuf
			}
			// Check for special characters that need escaping (no budget consumption here)
			for i := 0; i < c; i++ {
				b := msg[off+i]
				if isDomainNameLabelSpecial(b) || b < ' ' || b > '~' {
					fastPath = false
					break Loop
				}
			}
			off += c
			end = off
		case 0xC0:
			// compression pointer
			if off >= lenmsg {
				return nil, lenmsg, errors.New("dns: compression pointer out of bounds")
			}
			c1 := msg[off]
			off++
			if ptr == 0 {
				off1 = off
			}
			if ptr++; ptr > maxCompressionPointers {
				return nil, lenmsg, errors.New("too many compression pointers")
			}
			off = (c^0xC0)<<8 | int(c1)
			fastPath = false // compression requires copy
			break Loop
		default:
			return nil, lenmsg, ErrRdata
		}
	}
	if ptr == 0 {
		off1 = off
	}

	// Check if we can use zero-copy optimization for single-label names
	if fastPath && end > start {
		// Count labels to determine if zero-copy is viable
		labelCount := 0
		checkOff := start
		for checkOff < end {
			labelLen := int(msg[checkOff])
			if labelLen == 0 {
				break
			}
			labelCount++
			checkOff += labelLen + 1
		}

		if labelCount == 1 {
			// Single-label name: use buffer pool with minimal overhead
			labelLen := int(msg[start])
			buf := AcquireBuffer()
			defer ReleaseBuffer(buf)
			s := (*buf)[:0]
			s = append(s, msg[start+1:start+1+labelLen]...)
			s = append(s, '.')
			result := make([]byte, len(s))
			copy(result, s)
			return result, off1, nil
		}
	}

	// Slow path: use buffer pool for multi-label names, escaping, or compression
	buf := AcquireBuffer()
	defer ReleaseBuffer(buf)

	s := (*buf)[:0]
	off = start
SlowLoop:
	for {
		if off >= lenmsg {
			return nil, lenmsg, ErrBuf
		}
		c := int(msg[off])
		off++
		switch c & 0xC0 {
		case 0x00:
			if c == 0x00 {
				break SlowLoop
			}
			if off+c > lenmsg {
				return nil, lenmsg, ErrBuf
			}
			budget -= c + 1
			if budget <= 0 {
				return nil, lenmsg, ErrLongDomain
			}
			for _, b := range msg[off : off+c] {
				if isDomainNameLabelSpecial(b) {
					s = append(s, '\\', b)
				} else if b < ' ' || b > '~' {
					s = append(s, escapeByte(b)...)
				} else {
					s = append(s, b)
				}
			}
			s = append(s, '.')
			off += c
		case 0xC0:
			if off >= lenmsg {
				return nil, lenmsg, errors.New("dns: compression pointer out of bounds")
			}
			c1 := msg[off]
			off++
			if ptr == 0 {
				off1 = off
			}
			if ptr++; ptr > maxCompressionPointers {
				return nil, lenmsg, errors.New("too many compression pointers")
			}
			off = (c^0xC0)<<8 | int(c1)
		default:
			return nil, lenmsg, ErrRdata
		}
	}
	if len(s) == 0 {
		return []byte{'.'}, off1, nil
	}
	result := make([]byte, len(s))
	copy(result, s)
	return result, off1, nil
}

// UnpackRR unpacks msg[off:] into an RR.
func UnpackRR(msg []byte, off int) (rr RR, off1 int, err error) {
	h, off, msg, err := unpackHeader(msg, off)
	if err != nil {
		return nil, len(msg), err
	}

	return UnpackRRWithHeader(h, msg, off)
}

// Pools for common RR types to reduce allocations
var (
	poolA   = sync.Pool{New: func() any { return new(A) }}
	poolNS  = sync.Pool{New: func() any { return new(NS) }}
	poolCNAME = sync.Pool{New: func() any { return new(CNAME) }}
	poolMX  = sync.Pool{New: func() any { return new(MX) }}
	poolTXT = sync.Pool{New: func() any { return new(TXT) }}
	poolAAAA = sync.Pool{New: func() any { return new(AAAA) }}
)

// acquireRR gets an RR from the appropriate pool or creates a new one
func acquireRR(typ Type) RR {
	switch typ {
	case TypeA:
		return poolA.Get().(*A)
	case TypeNS:
		return poolNS.Get().(*NS)
	case TypeCNAME:
		return poolCNAME.Get().(*CNAME)
	case TypeMX:
		return poolMX.Get().(*MX)
	case TypeTXT:
		return poolTXT.Get().(*TXT)
	case TypeAAAA:
		return poolAAAA.Get().(*AAAA)
	default:
		if newFn, ok := TypeToRR[typ]; ok {
			return newFn()
		}
		return new(RFC3597)
	}
}

// releaseRR returns an RR to the appropriate pool
func releaseRR(rr RR) {
	switch rr.(type) {
	case *A:
		poolA.Put(rr)
	case *NS:
		poolNS.Put(rr)
	case *CNAME:
		poolCNAME.Put(rr)
	case *MX:
		poolMX.Put(rr)
	case *TXT:
		poolTXT.Put(rr)
	case *AAAA:
		poolAAAA.Put(rr)
	}
}

// UnpackRRWithHeader unpacks the record type specific payload given an existing
// RR_Header.
func UnpackRRWithHeader(h RR_Header, msg []byte, off int) (rr RR, off1 int, err error) {
	rr = acquireRR(h.Rrtype)
	*rr.Header() = h

	if off < 0 || off > len(msg) {
		return &h, off, &Error{err: "bad off"}
	}

	end := off + int(h.Rdlength)
	if end < off || end > len(msg) {
		return &h, end, &Error{err: "bad rdlength"}
	}

	if noRdata(h) {
		return rr, off, nil
	}

	off, err = rr.unpack(msg, off)
	if err != nil {
		return nil, end, err
	}
	if off != end {
		return &h, end, &Error{err: "bad rdlength"}
	}

	return rr, off, nil
}

// unpackHeader unpacks an RR header, returning the offset to the end of the header and a
// re-sliced msg according to the expected length of the RR.
func unpackHeader(msg []byte, off int) (rr RR_Header, off1 int, truncmsg []byte, err error) {
	hdr := RR_Header{}
	if off == len(msg) {
		return hdr, off, msg, nil
	}
	var u16 uint16
	var name []byte

	name, off, err = UnpackDomainName(msg, off)
	if err != nil {
		return hdr, len(msg), msg, err
	}
	hdr.Name = b2s(name)
	u16, off, err = unpackUint16(msg, off)
	if err != nil {
		return hdr, len(msg), msg, err
	}
	hdr.Rrtype = Type(u16)
	u16, off, err = unpackUint16(msg, off)
	if err != nil {
		return hdr, len(msg), msg, err
	}
	hdr.Class = Class(u16)
	hdr.Ttl, off, err = unpackUint32(msg, off)
	if err != nil {
		return hdr, len(msg), msg, err
	}
	hdr.Rdlength, off, err = unpackUint16(msg, off)
	if err != nil {
		return hdr, len(msg), msg, err
	}
	msg, err = truncateMsgFromRdlength(msg, off, hdr.Rdlength)
	return hdr, off, msg, err
}

// unpackRRslice unpacks msg[off:] into an []RR.
// If we cannot unpack the whole array, then it will return nil
// The dst parameter allows reuse of pre-allocated slices for zero-allocation unpacking.
func unpackRRslice(l int, msg []byte, off int, dst []RR) (dst1 []RR, off1 int, err error) {
	var r RR
	// Reuse provided slice if available
	dst = dst[:0]
	for i := 0; i < l; i++ {
		off1 := off
		r, off, err = UnpackRR(msg, off)
		if err != nil {
			off = len(msg)
			break
		}
		// If offset does not increase anymore, l is a lie
		if off1 == off {
			break
		}
		dst = append(dst, r)
	}
	if err != nil && off == len(msg) {
		dst = nil
	}
	return dst, off, err
}
