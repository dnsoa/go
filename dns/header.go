package dns

import (
	"encoding/binary"
	"strconv"
)

// Header is the wire format for the DNS packet header.
type Header struct {
	ID uint16
	// Flags is an arbitrary 16bit represents QR, Opcode, AA, TC, RD, RA, Z and RCODE.
	//
	//   0  1  2  3  4  5  6  7  8  9  A  B  C  D  E  F
	// +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
	// |QR|   Opcode  |AA|TC|RD|RA|   Z    |   RCODE   |
	// +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
	Bits uint16
	// Qdcount specifies the number of entries in the question section
	// Ancount specifies the number of resource records in the answer section
	// Nscount specifies the number of name server resource records in the authority records section
	// Arcount specifies the number of resource records in the additional records section
	Qdcount, Ancount, Nscount, Arcount uint16
}

const (
	headerSize = 12

	// Header.Bits
	_QR = 1 << 15 // query/response (response=1)
	_AA = 1 << 10 // authoritative
	_TC = 1 << 9  // truncated
	_RD = 1 << 8  // recursion desired
	_RA = 1 << 7  // recursion available
	_Z  = 1 << 6  // Z
	_AD = 1 << 5  // authenticated data
	_CD = 1 << 4  // checking disabled
)

func (h *Header) SetResponse() {
	h.Bits |= _QR
}

func (h *Header) Response() bool {
	return h.Bits&_QR != 0
}

func (h *Header) SetOpCode(op Opcode) {
	h.Bits &^= 0xf << 11
	h.Bits |= uint16(op) << 11
}

func (h *Header) OpCode() Opcode {
	return Opcode(h.Bits >> 11 & 0xf)
}

func (h *Header) SetRcode(rcode Rcode) {
	// 清除当前Header中的Rcode字段（低4位）
	h.Bits &= 0xFFFF ^ 0xF

	// 设置QR位为1，表示这是一个响应消息
	// 并将传入的rcode值设置到Rcode字段
	h.Bits |= 1<<15 | uint16(rcode)
}

func (h *Header) Rcode() Rcode {
	return Rcode(h.Bits & 0xF)
}

func (h *Header) SetAuthoritative() {
	h.Bits |= _AA
}

func (h *Header) Authoritative() bool {
	return h.Bits&_AA != 0
}

func (h *Header) SetTruncated() {
	h.Bits |= _TC
}

func (h *Header) Truncated() bool {
	return h.Bits&_TC != 0
}

func (h *Header) SetRecursionDesired() {
	h.Bits |= _RD
}

func (h *Header) RecursionDesired() bool {
	return h.Bits&_RD != 0
}

func (h *Header) SetRecursionAvailable() {
	h.Bits |= _RA
}

func (h *Header) RecursionAvailable() bool {
	return h.Bits&_RA != 0
}

func (h *Header) SetZero() {
	h.Bits |= _Z
}

func (h *Header) Zero() bool {
	return h.Bits&_Z != 0
}

func (h *Header) SetAuthenticatedData() {
	h.Bits |= _AD
}

func (h *Header) AuthenticatedData() bool {
	return h.Bits&_AD != 0
}

func (h *Header) SetCheckingDisabled() {
	h.Bits |= _CD
}

func (h *Header) CheckingDisabled() bool {
	return h.Bits&_CD != 0
}

// Pack returns the wire format of the header.
func (h *Header) Pack() [headerSize]byte {
	return [headerSize]byte{
		// ID
		byte(h.ID >> 8), byte(h.ID),
		// Flags
		byte(h.Bits >> 8), byte(h.Bits),
		// Qdcount
		byte(h.Qdcount >> 8), byte(h.Qdcount),
		// Ancount
		byte(h.Ancount >> 8), byte(h.Ancount),
		// Nscount
		byte(h.Nscount >> 8), byte(h.Nscount),
		// Arcount
		byte(h.Arcount >> 8), byte(h.Arcount),
	}
}

func (h *Header) Unpack(b []byte) error {
	if len(b) < headerSize {
		return ErrInvalidHeader
	}
	_ = b[11]
	h.ID = binary.BigEndian.Uint16(b[0:2])
	h.Bits = binary.BigEndian.Uint16(b[2:4])
	h.Qdcount = binary.BigEndian.Uint16(b[4:6])
	h.Ancount = binary.BigEndian.Uint16(b[6:8])
	h.Nscount = binary.BigEndian.Uint16(b[8:10])
	h.Arcount = binary.BigEndian.Uint16(b[10:12])
	return nil
}

func (h *Header) String() string {
	if h == nil {
		return "<nil> Header"
	}

	s := ";; opcode: " + OpcodeToString[h.OpCode()]
	s += ", status: " + RcodeToString[h.Rcode()]
	s += ", id: " + strconv.Itoa(int(h.ID)) + "\n"

	s += ";; flags:"
	if h.Response() {
		s += " qr"
	}
	if h.Authoritative() {
		s += " aa"
	}
	if h.Truncated() {
		s += " tc"
	}
	if h.RecursionDesired() {
		s += " rd"
	}
	if h.RecursionAvailable() {
		s += " ra"
	}
	if h.Zero() { // Hmm
		s += " z"
	}
	if h.AuthenticatedData() {
		s += " ad"
	}
	if h.CheckingDisabled() {
		s += " cd"
	}

	s += ";"
	return s
}
