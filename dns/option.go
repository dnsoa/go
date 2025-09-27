package dns

import (
	"encoding/binary"
	"errors"
	"fmt"
)

var (
	// ErrInvalidHeader is returned when dns message does not have the expected header size.
	ErrInvalidHeader = errors.New("dns message does not have the expected header size")
	// ErrInvalidQuestion is returned when dns message does not have the expected question size.
	ErrInvalidQuestion = errors.New("dns message does not have the expected question size")
	// ErrInvalidAnswer is returned when dns message does not have the expected answer size.
	ErrInvalidAnswer = errors.New("dns message does not have the expected answer size")
	// ErrInvalidOPT is returned when dns message does not have the expected OPT size.
	ErrInvalidOPT = errors.New("dns message does not have the expected OPT size")
	//ErrInvalidRR is returned when dns message does not have the expected RR size.
	ErrInvalidRR = errors.New("dns message does not have the expected RR size")
)

// OptionCode represents the code of a DNS Option, see RFC6891, section 6.1.2
type OptionCode uint16

func (c OptionCode) String() string {
	switch c {
	default:
		return "Unknown"
	case OptionCodeNSID:
		return "NSID"
	case OptionCodeDAU:
		return "DAU"
	case OptionCodeDHU:
		return "DHU"
	case OptionCodeN3U:
		return "N3U"
	case OptionCodeEDNSClientSubnet:
		return "EDNSClientSubnet"
	case OptionCodeEDNSExpire:
		return "EDNSExpire"
	case OptionCodeCookie:
		return "Cookie"
	case OptionCodeEDNSKeepAlive:
		return "EDNSKeepAlive"
	case OptionCodePadding:
		return "CodePadding"
	case OptionCodeChain:
		return "CodeChain"
	case OptionCodeEDNSKeyTag:
		return "CodeEDNSKeyTag"
	case OptionCodeEDNSClientTag:
		return "EDNSClientTag"
	case OptionCodeEDNSServerTag:
		return "EDNSServerTag"
	case OptionCodeDeviceID:
		return "DeviceID"
	}
}

// OptionCode known values. See IANA
const (
	OptionCodeNSID             OptionCode = 3
	OptionCodeDAU              OptionCode = 5
	OptionCodeDHU              OptionCode = 6
	OptionCodeN3U              OptionCode = 7
	OptionCodeEDNSClientSubnet OptionCode = 8
	OptionCodeEDNSExpire       OptionCode = 9
	OptionCodeCookie           OptionCode = 10
	OptionCodeEDNSKeepAlive    OptionCode = 11
	OptionCodePadding          OptionCode = 12
	OptionCodeChain            OptionCode = 13
	OptionCodeEDNSKeyTag       OptionCode = 14
	OptionCodeEDNSClientTag    OptionCode = 16
	OptionCodeEDNSServerTag    OptionCode = 17
	OptionCodeDeviceID         OptionCode = 26946
	_DO                                   = 1 << 15 // DNSSEC OK
)

type OPT struct {
	// Name     string
	// Type     Type
	// MaxSize  uint16
	// TTL      uint32
	// RDLength uint16
	Options []Option
	Hdr     RR_Header
}

func (rr *OPT) Header() *RR_Header { return &rr.Hdr }
func (r *OPT) String() string {
	s := r.Hdr.String() + "\n"
	for _, o := range r.Options {
		s += "    Option: " + o.String() + "\n"
	}
	return s
}
func (r *OPT) pack(msg []byte, off int) (off1 int, err error) {
	// OPT has no RDATA to pack.
	return off, nil
}

func (r *OPT) unpack(msg []byte, off int) (int, error) {
	fmt.Println("unpack OPT")
	for i := off; i < off+int(r.Hdr.Class); {
		if i+4 > len(msg) {
			return off, ErrInvalidOPT
		}
		code := OptionCode(binary.BigEndian.Uint16(msg[i : i+2]))
		length := binary.BigEndian.Uint16(msg[i+2 : i+4])
		if i+4+int(length) > len(msg) {
			return off, ErrInvalidOPT
		}
		r.Options = append(r.Options, Option{Code: code, Length: length, Data: msg[i+4 : i+4+int(length)]})
		i += 4 + int(length)
	}
	return off + int(r.Hdr.Rdlength), nil
}
func (r *OPT) Pack() []byte {
	var b [11]byte
	binary.BigEndian.PutUint16(b[1:3], uint16(r.Hdr.Rrtype))
	binary.BigEndian.PutUint16(b[3:5], uint16(r.Hdr.Class))
	binary.BigEndian.PutUint32(b[5:9], r.Hdr.Ttl)
	binary.BigEndian.PutUint16(b[9:11], r.Hdr.Rdlength)
	return b[:]

}

func (r *OPT) Unpack(data []byte) error {
	if len(data) < 11 {
		return ErrInvalidOPT
	}
	r.Hdr.Rrtype = Type(binary.BigEndian.Uint16(data[1:3]))
	r.Hdr.Class = Class(binary.BigEndian.Uint16(data[3:5]))
	r.Hdr.Ttl = binary.BigEndian.Uint32(data[5:9])
	r.Hdr.Rdlength = binary.BigEndian.Uint16(data[9:11])
	if r.Hdr.Rdlength == 0 {
		return nil
	}
	if len(data) < 11+int(r.Hdr.Rdlength) {
		return ErrInvalidOPT
	}
	for i := 11; i < 11+int(r.Hdr.Rdlength); {
		if i+4 > len(data) {
			return ErrInvalidOPT
		}
		code := OptionCode(binary.BigEndian.Uint16(data[i : i+2]))
		length := binary.BigEndian.Uint16(data[i+2 : i+4])
		if i+4+int(length) > len(data) {
			return ErrInvalidOPT
		}
		r.Options = append(r.Options, Option{Code: code, Length: length, Data: data[i+4 : i+4+int(length)]})
		i += 4 + int(length)
	}
	return nil
}

func (r *OPT) AddOption(code OptionCode, data []byte) {
	r.Options = append(r.Options, Option{Code: code, Length: uint16(len(data)), Data: data})
	r.Hdr.Rdlength += 4 + uint16(len(data))
}

type Option struct {
	Data   []byte
	Code   OptionCode
	Length uint16
}

func (o *Option) String() string {
	return fmt.Sprintf("Option{Code: %s, Data: %x}", o.Code, o.Data)
}
