package dns

import (
	"encoding/binary"
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
	A   net.IP
	Hdr RR_Header
}

func (rr *A) Header() *RR_Header { return &rr.Hdr }
func (rr *A) String() string {
	if rr.A == nil {
		return rr.Hdr.String()
	}
	return rr.Hdr.String() + rr.A.String()
}
func (rr *A) pack(msg []byte, off int) (off1 int, err error) {
	off, err = packDataA(rr.A, msg, off)
	if err != nil {
		return off, err
	}
	return off, nil
}

func (rr *A) unpack(msg []byte, off int) (off1 int, err error) {
	if len(msg) < off+net.IPv4len {
		return off, ErrInvalidRR
	}
	data := msg[off : off+net.IPv4len]
	rr.A = net.IPv4(data[0], data[1], data[2], data[3]).To4()
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
func (rr *NS) Pack() []byte {
	var buf []byte
	// NAME
	buf = append(buf, 0xc0, 0x0c)
	// TYPE
	binary.BigEndian.PutUint16(buf[len(buf):len(buf)+2], uint16(TypeNS))
	buf = append(buf, byte(TypeNS>>8), byte(TypeNS))
	// CLASS
	binary.BigEndian.PutUint16(buf[len(buf):len(buf)+2], uint16(rr.Hdr.Class))
	buf = append(buf, byte(rr.Hdr.Class>>8), byte(rr.Hdr.Class))
	// TTL
	binary.BigEndian.PutUint32(buf[len(buf):len(buf)+4], rr.Hdr.Ttl)
	buf = append(buf, byte(rr.Hdr.Ttl>>24), byte(rr.Hdr.Ttl>>16), byte(rr.Hdr.Ttl>>8), byte(rr.Hdr.Ttl))
	// RDLENGTH + RDATA
	rd := EncodeDomain(nil, rr.NS)
	binary.BigEndian.PutUint16(buf[len(buf):len(buf)+2], uint16(len(rd)))
	buf = append(buf, byte(len(rd)>>8), byte(len(rd)))
	buf = append(buf, rd...)
	return buf
}

// CNAME 记录
type CNAME struct {
	Hdr   RR_Header
	CNAME string
}

func (rr *CNAME) Header() *RR_Header { return &rr.Hdr }
func (rr *CNAME) Pack() []byte {
	var buf []byte
	buf = append(buf, 0xc0, 0x0c)
	binary.BigEndian.PutUint16(buf[len(buf):len(buf)+2], uint16(TypeCNAME))
	buf = append(buf, byte(TypeCNAME>>8), byte(TypeCNAME))
	binary.BigEndian.PutUint16(buf[len(buf):len(buf)+2], uint16(rr.Hdr.Class))
	buf = append(buf, byte(rr.Hdr.Class>>8), byte(rr.Hdr.Class))
	binary.BigEndian.PutUint32(buf[len(buf):len(buf)+4], rr.Hdr.Ttl)
	buf = append(buf, byte(rr.Hdr.Ttl>>24), byte(rr.Hdr.Ttl>>16), byte(rr.Hdr.Ttl>>8), byte(rr.Hdr.Ttl))
	rd := EncodeDomain(nil, rr.CNAME)
	binary.BigEndian.PutUint16(buf[len(buf):len(buf)+2], uint16(len(rd)))
	buf = append(buf, byte(len(rd)>>8), byte(len(rd)))
	buf = append(buf, rd...)
	return buf
}

// MX 记录
type MX struct {
	Hdr        RR_Header
	Preference uint16
	MX         string
}

func (rr *MX) Header() *RR_Header { return &rr.Hdr }
func (rr *MX) Pack() []byte {
	var buf []byte
	buf = append(buf, 0xc0, 0x0c)
	binary.BigEndian.PutUint16(buf[len(buf):len(buf)+2], uint16(TypeMX))
	buf = append(buf, byte(TypeMX>>8), byte(TypeMX))
	binary.BigEndian.PutUint16(buf[len(buf):len(buf)+2], uint16(rr.Hdr.Class))
	buf = append(buf, byte(rr.Hdr.Class>>8), byte(rr.Hdr.Class))
	binary.BigEndian.PutUint32(buf[len(buf):len(buf)+4], rr.Hdr.Ttl)
	buf = append(buf, byte(rr.Hdr.Ttl>>24), byte(rr.Hdr.Ttl>>16), byte(rr.Hdr.Ttl>>8), byte(rr.Hdr.Ttl))
	rd := EncodeDomain(nil, rr.MX)
	rdlen := 2 + len(rd)
	binary.BigEndian.PutUint16(buf[len(buf):len(buf)+2], uint16(rdlen))
	buf = append(buf, byte(rdlen>>8), byte(rdlen))
	buf = append(buf, byte(rr.Preference>>8), byte(rr.Preference))
	buf = append(buf, rd...)
	return buf
}

// TXT 记录
type TXT struct {
	Hdr RR_Header
	TXT []string
}

func (rr *TXT) Header() *RR_Header { return &rr.Hdr }
func (rr *TXT) Pack() []byte {
	var buf []byte
	buf = append(buf, 0xc0, 0x0c)
	binary.BigEndian.PutUint16(buf[len(buf):len(buf)+2], uint16(TypeTXT))
	buf = append(buf, byte(TypeTXT>>8), byte(TypeTXT))
	binary.BigEndian.PutUint16(buf[len(buf):len(buf)+2], uint16(rr.Hdr.Class))
	buf = append(buf, byte(rr.Hdr.Class>>8), byte(rr.Hdr.Class))
	binary.BigEndian.PutUint32(buf[len(buf):len(buf)+4], rr.Hdr.Ttl)
	buf = append(buf, byte(rr.Hdr.Ttl>>24), byte(rr.Hdr.Ttl>>16), byte(rr.Hdr.Ttl>>8), byte(rr.Hdr.Ttl))
	var txtData []byte
	for _, s := range rr.TXT {
		txtData = append(txtData, byte(len(s)))
		txtData = append(txtData, s...)
	}
	binary.BigEndian.PutUint16(buf[len(buf):len(buf)+2], uint16(len(txtData)))
	buf = append(buf, byte(len(txtData)>>8), byte(len(txtData)))
	buf = append(buf, txtData...)
	return buf
}

// AAAA 记录
type AAAA struct {
	Hdr  RR_Header
	AAAA net.IP
}

func (rr *AAAA) Header() *RR_Header { return &rr.Hdr }
func (rr *AAAA) Pack() []byte {
	var buf []byte
	buf = append(buf, 0xc0, 0x0c)
	binary.BigEndian.PutUint16(buf[len(buf):len(buf)+2], uint16(TypeAAAA))
	buf = append(buf, byte(TypeAAAA>>8), byte(TypeAAAA))
	binary.BigEndian.PutUint16(buf[len(buf):len(buf)+2], uint16(rr.Hdr.Class))
	buf = append(buf, byte(rr.Hdr.Class>>8), byte(rr.Hdr.Class))
	binary.BigEndian.PutUint32(buf[len(buf):len(buf)+4], rr.Hdr.Ttl)
	buf = append(buf, byte(rr.Hdr.Ttl>>24), byte(rr.Hdr.Ttl>>16), byte(rr.Hdr.Ttl>>8), byte(rr.Hdr.Ttl))
	binary.BigEndian.PutUint16(buf[len(buf):len(buf)+2], 16)
	buf = append(buf, 0, 16)
	buf = append(buf, rr.AAAA.To16()...)
	return buf
}

// OPTRecord 已在其他文件实现
