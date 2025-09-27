package dns

import (
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"math/big"
	"net"
	"strings"
)

func packDataA(a net.IP, msg []byte, off int) (int, error) {
	switch len(a) {
	case net.IPv4len, net.IPv6len:
		// It must be a slice of 4, even if it is 16, we encode only the first 4
		if off+net.IPv4len > len(msg) {
			return len(msg), &Error{err: "overflow packing a"}
		}

		copy(msg[off:], a.To4())
		off += net.IPv4len
	case 0:
		// Allowed, for dynamic updates.
	default:
		return len(msg), &Error{err: "overflow packing a"}
	}
	return off, nil
}

// truncateMsgFromRdLength truncates msg to match the expected length of the RR.
// Returns an error if msg is smaller than the expected size.
func truncateMsgFromRdlength(msg []byte, off int, rdlength uint16) (truncmsg []byte, err error) {
	lenrd := off + int(rdlength)
	if lenrd > len(msg) {
		return msg, &Error{err: "overflowing header size"}
	}
	return msg[:lenrd], nil
}

var base32HexNoPadEncoding = base32.HexEncoding.WithPadding(base32.NoPadding)

func fromBase32(s []byte) (buf []byte, err error) {
	for i, b := range s {
		if b >= 'a' && b <= 'z' {
			s[i] = b - 32
		}
	}
	buflen := base32HexNoPadEncoding.DecodedLen(len(s))
	buf = make([]byte, buflen)
	n, err := base32HexNoPadEncoding.Decode(buf, s)
	buf = buf[:n]
	return
}

func toBase32(b []byte) string {
	return base32HexNoPadEncoding.EncodeToString(b)
}

func fromBase64(s []byte) (buf []byte, err error) {
	buflen := base64.StdEncoding.DecodedLen(len(s))
	buf = make([]byte, buflen)
	n, err := base64.StdEncoding.Decode(buf, s)
	buf = buf[:n]
	return
}

func toBase64(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

// dynamicUpdate returns true if the Rdlength is zero.
func noRdata(h RR_Header) bool { return h.Rdlength == 0 }

func unpackUint8(msg []byte, off int) (i uint8, off1 int, err error) {
	if off+1 > len(msg) {
		return 0, len(msg), &Error{err: "overflow unpacking uint8"}
	}
	return msg[off], off + 1, nil
}

func packUint8(i uint8, msg []byte, off int) (off1 int, err error) {
	if off+1 > len(msg) {
		return len(msg), &Error{err: "overflow packing uint8"}
	}
	msg[off] = i
	return off + 1, nil
}

func unpackUint16(msg []byte, off int) (i uint16, off1 int, err error) {
	if off+2 > len(msg) {
		return 0, len(msg), &Error{err: "overflow unpacking uint16"}
	}
	return binary.BigEndian.Uint16(msg[off:]), off + 2, nil
}

func packUint16(i uint16, msg []byte, off int) (off1 int, err error) {
	if off+2 > len(msg) {
		return len(msg), &Error{err: "overflow packing uint16"}
	}
	binary.BigEndian.PutUint16(msg[off:], i)
	return off + 2, nil
}

func unpackUint32(msg []byte, off int) (i uint32, off1 int, err error) {
	if off+4 > len(msg) {
		return 0, len(msg), &Error{err: "overflow unpacking uint32"}
	}
	return binary.BigEndian.Uint32(msg[off:]), off + 4, nil
}

func packUint32(i uint32, msg []byte, off int) (off1 int, err error) {
	if off+4 > len(msg) {
		return len(msg), &Error{err: "overflow packing uint32"}
	}
	binary.BigEndian.PutUint32(msg[off:], i)
	return off + 4, nil
}

func unpackUint48(msg []byte, off int) (i uint64, off1 int, err error) {
	if off+6 > len(msg) {
		return 0, len(msg), &Error{err: "overflow unpacking uint64 as uint48"}
	}
	// Used in TSIG where the last 48 bits are occupied, so for now, assume a uint48 (6 bytes)
	i = uint64(msg[off])<<40 | uint64(msg[off+1])<<32 | uint64(msg[off+2])<<24 | uint64(msg[off+3])<<16 |
		uint64(msg[off+4])<<8 | uint64(msg[off+5])
	off += 6
	return i, off, nil
}

func packUint48(i uint64, msg []byte, off int) (off1 int, err error) {
	if off+6 > len(msg) {
		return len(msg), &Error{err: "overflow packing uint64 as uint48"}
	}
	msg[off] = byte(i >> 40)
	msg[off+1] = byte(i >> 32)
	msg[off+2] = byte(i >> 24)
	msg[off+3] = byte(i >> 16)
	msg[off+4] = byte(i >> 8)
	msg[off+5] = byte(i)
	off += 6
	return off, nil
}

func unpackUint64(msg []byte, off int) (i uint64, off1 int, err error) {
	if off+8 > len(msg) {
		return 0, len(msg), &Error{err: "overflow unpacking uint64"}
	}
	return binary.BigEndian.Uint64(msg[off:]), off + 8, nil
}

func packUint64(i uint64, msg []byte, off int) (off1 int, err error) {
	if off+8 > len(msg) {
		return len(msg), &Error{err: "overflow packing uint64"}
	}
	binary.BigEndian.PutUint64(msg[off:], i)
	off += 8
	return off, nil
}

func unpackString(msg []byte, off int) (string, int, error) {
	if off+1 > len(msg) {
		return "", off, &Error{err: "overflow unpacking txt"}
	}
	l := int(msg[off])
	off++
	if off+l > len(msg) {
		return "", off, &Error{err: "overflow unpacking txt"}
	}
	var s strings.Builder
	consumed := 0
	for i, b := range msg[off : off+l] {
		switch {
		case b == '"' || b == '\\':
			if consumed == 0 {
				s.Grow(l * 2)
			}
			s.Write(msg[off+consumed : off+i])
			s.WriteByte('\\')
			s.WriteByte(b)
			consumed = i + 1
		case b < ' ' || b > '~': // unprintable
			if consumed == 0 {
				s.Grow(l * 2)
			}
			s.Write(msg[off+consumed : off+i])
			s.WriteString(escapeByte(b))
			consumed = i + 1
		}
	}
	if consumed == 0 { // no escaping needed
		return string(msg[off : off+l]), off + l, nil
	}
	s.Write(msg[off+consumed : off+l])
	return s.String(), off + l, nil
}

func packString(s string, msg []byte, off int) (int, error) {
	off, err := packTxtString(s, msg, off)
	if err != nil {
		return len(msg), err
	}
	return off, nil
}

func unpackStringBase32(msg []byte, off, end int) (string, int, error) {
	if end > len(msg) {
		return "", len(msg), &Error{err: "overflow unpacking base32"}
	}
	s := toBase32(msg[off:end])
	return s, end, nil
}

func packStringBase32(s string, msg []byte, off int) (int, error) {
	b32, err := fromBase32([]byte(s))
	if err != nil {
		return len(msg), err
	}
	if off+len(b32) > len(msg) {
		return len(msg), &Error{err: "overflow packing base32"}
	}
	copy(msg[off:off+len(b32)], b32)
	off += len(b32)
	return off, nil
}

func unpackStringBase64(msg []byte, off, end int) (string, int, error) {
	// Rest of the RR is base64 encoded value, so we don't need an explicit length
	// to be set. Thus far all RR's that have base64 encoded fields have those as their
	// last one. What we do need is the end of the RR!
	if end > len(msg) {
		return "", len(msg), &Error{err: "overflow unpacking base64"}
	}
	s := toBase64(msg[off:end])
	return s, end, nil
}

func packStringBase64(s string, msg []byte, off int) (int, error) {
	b64, err := fromBase64([]byte(s))
	if err != nil {
		return len(msg), err
	}
	if off+len(b64) > len(msg) {
		return len(msg), &Error{err: "overflow packing base64"}
	}
	copy(msg[off:off+len(b64)], b64)
	off += len(b64)
	return off, nil
}

func unpackStringHex(msg []byte, off, end int) (string, int, error) {
	// Rest of the RR is hex encoded value, so we don't need an explicit length
	// to be set. NSEC and TSIG have hex fields with a length field.
	// What we do need is the end of the RR!
	if end > len(msg) {
		return "", len(msg), &Error{err: "overflow unpacking hex"}
	}

	s := hex.EncodeToString(msg[off:end])
	return s, end, nil
}

func packStringHex(s string, msg []byte, off int) (int, error) {
	h, err := hex.DecodeString(s)
	if err != nil {
		return len(msg), err
	}
	if off+len(h) > len(msg) {
		return len(msg), &Error{err: "overflow packing hex"}
	}
	copy(msg[off:off+len(h)], h)
	off += len(h)
	return off, nil
}

func unpackStringAny(msg []byte, off, end int) (string, int, error) {
	if end > len(msg) {
		return "", len(msg), &Error{err: "overflow unpacking anything"}
	}
	return string(msg[off:end]), end, nil
}

func packStringAny(s string, msg []byte, off int) (int, error) {
	if off+len(s) > len(msg) {
		return len(msg), &Error{err: "overflow packing anything"}
	}
	copy(msg[off:off+len(s)], s)
	off += len(s)
	return off, nil
}

func unpackStringTxt(msg []byte, off int) ([]string, int, error) {
	txt, off, err := unpackTxt(msg, off)
	if err != nil {
		return nil, len(msg), err
	}
	return txt, off, nil
}

func packStringTxt(s []string, msg []byte, off int) (int, error) {
	off, err := packTxt(s, msg, off)
	if err != nil {
		return len(msg), err
	}
	return off, nil
}

func packTxt(txt []string, msg []byte, offset int) (int, error) {
	if len(txt) == 0 {
		if offset >= len(msg) {
			return offset, ErrBuf
		}
		msg[offset] = 0
		return offset, nil
	}
	var err error
	for _, s := range txt {
		offset, err = packTxtString(s, msg, offset)
		if err != nil {
			return offset, err
		}
	}
	return offset, nil
}

func packTxtString(s string, msg []byte, offset int) (int, error) {
	lenByteOffset := offset
	if offset >= len(msg) || len(s) > 256*4+1 /* If all \DDD */ {
		return offset, ErrBuf
	}
	offset++
	for i := 0; i < len(s); i++ {
		if len(msg) <= offset {
			return offset, ErrBuf
		}
		if s[i] == '\\' {
			i++
			if i == len(s) {
				break
			}
			// check for \DDD
			if isDDD(s[i:]) {
				msg[offset] = dddToByte(s[i:])
				i += 2
			} else {
				msg[offset] = s[i]
			}
		} else {
			msg[offset] = s[i]
		}
		offset++
	}
	l := offset - lenByteOffset - 1
	if l > 255 {
		return offset, &Error{err: "string exceeded 255 bytes in txt"}
	}
	msg[lenByteOffset] = byte(l)
	return offset, nil
}

func packOctetString(s string, msg []byte, offset int) (int, error) {
	if offset >= len(msg) || len(s) > 256*4+1 {
		return offset, ErrBuf
	}
	for i := 0; i < len(s); i++ {
		if len(msg) <= offset {
			return offset, ErrBuf
		}
		if s[i] == '\\' {
			i++
			if i == len(s) {
				break
			}
			// check for \DDD
			if isDDD(s[i:]) {
				msg[offset] = dddToByte(s[i:])
				i += 2
			} else {
				msg[offset] = s[i]
			}
		} else {
			msg[offset] = s[i]
		}
		offset++
	}
	return offset, nil
}

func unpackTxt(msg []byte, off0 int) (ss []string, off int, err error) {
	off = off0
	var s string
	for off < len(msg) && err == nil {
		s, off, err = unpackString(msg, off)
		if err == nil {
			ss = append(ss, s)
		}
	}
	return
}

// Helpers for dealing with escaped bytes
func isDigit(b byte) bool { return b >= '0' && b <= '9' }

func isDDD[T ~[]byte | ~string](s T) bool {
	return len(s) >= 3 && isDigit(s[0]) && isDigit(s[1]) && isDigit(s[2])
}

func dddToByte[T ~[]byte | ~string](s T) byte {
	_ = s[2] // bounds check hint to compiler; see golang.org/issue/14808
	return byte((s[0]-'0')*100 + (s[1]-'0')*10 + (s[2] - '0'))
}

// Helper function for packing and unpacking
func intToBytes(i *big.Int, length int) []byte {
	buf := i.Bytes()
	if len(buf) < length {
		b := make([]byte, length)
		copy(b[length-len(buf):], buf)
		return b
	}
	return buf
}
