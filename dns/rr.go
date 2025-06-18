package dns

import (
	"encoding/binary"
	"net"
)

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
