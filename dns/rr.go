package dns

import (
	"net"
	"strconv"
)

// RFC3597 represents an unknown/generic RR. See RFC 3597.
type RFC3597 struct {
	Hdr   RR_Header
	Rdata string `dns:"hex"`
}

func (rr *RFC3597) pack(msg []byte, off int) (off1 int, err error) {
	off, err = packStringHex(rr.Rdata, msg, off)
	if err != nil {
		return off, err
	}
	return off, nil
}
func (rr *RFC3597) unpack(msg []byte, off int) (off1 int, err error) {
	rdStart := off
	_ = rdStart

	rr.Rdata, off, err = unpackStringHex(msg, off, rdStart+int(rr.Hdr.Rdlength))
	if err != nil {
		return off, err
	}
	return off, nil
}
func (rr *RFC3597) Header() *RR_Header { return &rr.Hdr }
func (rr *RFC3597) String() string {
	// Let's call it a hack
	s := rfc3597Header(rr.Hdr)

	s += "\\# " + strconv.Itoa(len(rr.Rdata)/2) + " " + rr.Rdata
	return s
}

type A struct {
	A   [4]byte // Fixed-size array to avoid net.IP allocations
	Hdr RR_Header
}

// ipTo4 converts net.IP to [4]byte
func ipTo4(ip net.IP) [4]byte {
	if ip4 := ip.To4(); ip4 != nil {
		return [4]byte{ip4[0], ip4[1], ip4[2], ip4[3]}
	}
	return [4]byte{}
}

func (rr *A) Header() *RR_Header { return &rr.Hdr }
func (rr *A) String() string {
	if rr.A == [4]byte{} {
		return rr.Hdr.String()
	}
	return rr.Hdr.String() + net.IP(rr.A[:]).String()
}
func (rr *A) pack(msg []byte, off int) (off1 int, err error) {
	if off+net.IPv4len > len(msg) {
		return off, &Error{err: "overflow packing A"}
	}
	copy(msg[off:], rr.A[:])
	off += net.IPv4len
	return off, nil
}

func (rr *A) unpack(msg []byte, off int) (off1 int, err error) {
	if len(msg) < off+net.IPv4len {
		return off, ErrInvalidRR
	}
	copy(rr.A[:], msg[off:off+net.IPv4len])
	off += net.IPv4len
	return off, nil
}

func rfc3597Header(h RR_Header) string {
	var s string

	s += sprintName(h.Name) + "\t"
	s += strconv.FormatInt(int64(h.Ttl), 10) + "\t"
	s += "CLASS" + strconv.Itoa(int(h.Class)) + "\t"
	s += "TYPE" + strconv.Itoa(int(h.Rrtype)) + "\t"
	return s
}

// NS 记录
type NS struct {
	Hdr RR_Header
	NS  string
}

func (rr *NS) Header() *RR_Header { return &rr.Hdr }

func (rr *NS) pack(msg []byte, off int) (off1 int, err error) {
	off, err = packDomainName(rr.NS, msg, off)
	if err != nil {
		return off, err
	}
	return off, nil
}

func (rr *NS) unpack(msg []byte, off int) (off1 int, err error) {
	name, off, err := UnpackDomainName(msg, off)
	if err != nil {
		return off, err
	}
	rr.NS = b2s(name)
	return off, nil
}

func (rr *NS) String() string {
	return rr.Hdr.String() + rr.NS
}

// CNAME record
type CNAME struct {
	Hdr   RR_Header
	CNAME string
}

func (rr *CNAME) Header() *RR_Header { return &rr.Hdr }

func (rr *CNAME) pack(msg []byte, off int) (off1 int, err error) {
	off, err = packDomainName(rr.CNAME, msg, off)
	if err != nil {
		return off, err
	}
	return off, nil
}

func (rr *CNAME) unpack(msg []byte, off int) (off1 int, err error) {
	name, off, err := UnpackDomainName(msg, off)
	if err != nil {
		return off, err
	}
	rr.CNAME = b2s(name)
	return off, nil
}

func (rr *CNAME) String() string {
	return rr.Hdr.String() + rr.CNAME
}

// MX record
type MX struct {
	Hdr        RR_Header
	Preference uint16
	MX         string
}

func (rr *MX) Header() *RR_Header { return &rr.Hdr }

func (rr *MX) pack(msg []byte, off int) (off1 int, err error) {
	off, err = packUint16(rr.Preference, msg, off)
	if err != nil {
		return off, err
	}
	off, err = packDomainName(rr.MX, msg, off)
	if err != nil {
		return off, err
	}
	return off, nil
}

func (rr *MX) unpack(msg []byte, off int) (off1 int, err error) {
	rr.Preference, off, err = unpackUint16(msg, off)
	if err != nil {
		return off, err
	}
	name, off, err := UnpackDomainName(msg, off)
	if err != nil {
		return off, err
	}
	rr.MX = b2s(name)
	return off, nil
}

func (rr *MX) String() string {
	return rr.Hdr.String() + strconv.Itoa(int(rr.Preference)) + " " + rr.MX
}

// TXT record
type TXT struct {
	Hdr RR_Header
	TXT []string
}

func (rr *TXT) Header() *RR_Header { return &rr.Hdr }

func (rr *TXT) pack(msg []byte, off int) (off1 int, err error) {
	off, err = packStringTxt(rr.TXT, msg, off)
	if err != nil {
		return off, err
	}
	return off, nil
}

func (rr *TXT) unpack(msg []byte, off int) (off1 int, err error) {
	rr.TXT, off, err = unpackStringTxt(msg, off)
	if err != nil {
		return off, err
	}
	return off, nil
}

func (rr *TXT) String() string {
	s := rr.Hdr.String()
	for i, txt := range rr.TXT {
		if i > 0 {
			s += " "
		}
		s += strconv.Quote(txt)
	}
	return s
}

// AAAA record
type AAAA struct {
	Hdr  RR_Header
	AAAA net.IP
}

func (rr *AAAA) Header() *RR_Header { return &rr.Hdr }

func (rr *AAAA) pack(msg []byte, off int) (off1 int, err error) {
	if off+net.IPv6len > len(msg) {
		return off, &Error{err: "overflow packing AAAA"}
	}
	ip := rr.AAAA.To16()
	if ip == nil {
		return off, &Error{err: "invalid IPv6 address"}
	}
	copy(msg[off:], ip)
	off += net.IPv6len
	return off, nil
}

func (rr *AAAA) unpack(msg []byte, off int) (off1 int, err error) {
	if len(msg) < off+net.IPv6len {
		return off, ErrInvalidRR
	}
	data := msg[off : off+net.IPv6len]
	rr.AAAA = make(net.IP, net.IPv6len)
	copy(rr.AAAA, data)
	off += net.IPv6len
	return off, nil
}

func (rr *AAAA) String() string {
	if rr.AAAA == nil {
		return rr.Hdr.String()
	}
	return rr.Hdr.String() + rr.AAAA.String()
}

// SOA record (Start of Authority)
// RFC 1035, section 3.3.13
type SOA struct {
	Hdr     RR_Header
	Ns      string  // Primary name server
	Mbox    string  // Responsible mailbox
	Serial  uint32  // Serial number
	Refresh uint32  // Refresh interval
	Retry   uint32  // Retry interval
	Expire  uint32  // Expire limit
	Minttl  uint32  // Minimum TTL
}

func (rr *SOA) Header() *RR_Header { return &rr.Hdr }

func (rr *SOA) pack(msg []byte, off int) (off1 int, err error) {
	off, err = packDomainName(rr.Ns, msg, off)
	if err != nil {
		return off, err
	}
	off, err = packDomainName(rr.Mbox, msg, off)
	if err != nil {
		return off, err
	}
	off, err = packUint32(rr.Serial, msg, off)
	if err != nil {
		return off, err
	}
	off, err = packUint32(rr.Refresh, msg, off)
	if err != nil {
		return off, err
	}
	off, err = packUint32(rr.Retry, msg, off)
	if err != nil {
		return off, err
	}
	off, err = packUint32(rr.Expire, msg, off)
	if err != nil {
		return off, err
	}
	off, err = packUint32(rr.Minttl, msg, off)
	if err != nil {
		return off, err
	}
	return off, nil
}

func (rr *SOA) unpack(msg []byte, off int) (off1 int, err error) {
	name, off, err := UnpackDomainName(msg, off)
	if err != nil {
		return off, err
	}
	rr.Ns = b2s(name)
	name, off, err = UnpackDomainName(msg, off)
	if err != nil {
		return off, err
	}
	rr.Mbox = b2s(name)
	rr.Serial, off, err = unpackUint32(msg, off)
	if err != nil {
		return off, err
	}
	rr.Refresh, off, err = unpackUint32(msg, off)
	if err != nil {
		return off, err
	}
	rr.Retry, off, err = unpackUint32(msg, off)
	if err != nil {
		return off, err
	}
	rr.Expire, off, err = unpackUint32(msg, off)
	if err != nil {
		return off, err
	}
	rr.Minttl, off, err = unpackUint32(msg, off)
	if err != nil {
		return off, err
	}
	return off, nil
}

func (rr *SOA) String() string {
	return rr.Hdr.String() +
		rr.Ns + " " +
		rr.Mbox + " " +
		strconv.FormatUint(uint64(rr.Serial), 10) + " " +
		strconv.FormatUint(uint64(rr.Refresh), 10) + " " +
		strconv.FormatUint(uint64(rr.Retry), 10) + " " +
		strconv.FormatUint(uint64(rr.Expire), 10) + " " +
		strconv.FormatUint(uint64(rr.Minttl), 10)
}

// PTR record (Pointer)
// RFC 1035, section 3.3.12
type PTR struct {
	Hdr RR_Header
	Ptr string
}

func (rr *PTR) Header() *RR_Header { return &rr.Hdr }

func (rr *PTR) pack(msg []byte, off int) (off1 int, err error) {
	off, err = packDomainName(rr.Ptr, msg, off)
	if err != nil {
		return off, err
	}
	return off, nil
}

func (rr *PTR) unpack(msg []byte, off int) (off1 int, err error) {
	name, off, err := UnpackDomainName(msg, off)
	if err != nil {
		return off, err
	}
	rr.Ptr = b2s(name)
	return off, nil
}

func (rr *PTR) String() string {
	return rr.Hdr.String() + rr.Ptr
}

// SRV record (Service)
// RFC 2782
type SRV struct {
	Hdr      RR_Header
	Priority uint16 // Priority
	Weight   uint16 // Weight
	Port     uint16 // Port
	Target   string // Target domain name
}

func (rr *SRV) Header() *RR_Header { return &rr.Hdr }

func (rr *SRV) pack(msg []byte, off int) (off1 int, err error) {
	off, err = packUint16(rr.Priority, msg, off)
	if err != nil {
		return off, err
	}
	off, err = packUint16(rr.Weight, msg, off)
	if err != nil {
		return off, err
	}
	off, err = packUint16(rr.Port, msg, off)
	if err != nil {
		return off, err
	}
	off, err = packDomainName(rr.Target, msg, off)
	if err != nil {
		return off, err
	}
	return off, nil
}

func (rr *SRV) unpack(msg []byte, off int) (off1 int, err error) {
	rr.Priority, off, err = unpackUint16(msg, off)
	if err != nil {
		return off, err
	}
	rr.Weight, off, err = unpackUint16(msg, off)
	if err != nil {
		return off, err
	}
	rr.Port, off, err = unpackUint16(msg, off)
	if err != nil {
		return off, err
	}
	name, off, err := UnpackDomainName(msg, off)
	if err != nil {
		return off, err
	}
	rr.Target = b2s(name)
	return off, nil
}

func (rr *SRV) String() string {
	return rr.Hdr.String() +
		strconv.Itoa(int(rr.Priority)) + " " +
		strconv.Itoa(int(rr.Weight)) + " " +
		strconv.Itoa(int(rr.Port)) + " " +
		rr.Target
}
