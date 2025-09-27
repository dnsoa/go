package pool

import (
	"encoding/base64"
	"encoding/hex"
	"net/netip"
	"strconv"
)

type AppendBuffer []byte

func (b AppendBuffer) Str(s string) AppendBuffer {
	return append(b, s...)
}

func (b AppendBuffer) Bytes(s []byte) AppendBuffer {
	return append(b, s...)
}

func (b AppendBuffer) Byte(c byte) AppendBuffer {
	return append(b, c)
}

func (b AppendBuffer) Base64(data []byte) AppendBuffer {
	return base64.StdEncoding.AppendEncode(b, data)
}

func (b AppendBuffer) Hex(data []byte) AppendBuffer {
	return hex.AppendEncode(b, data)
}

func (b AppendBuffer) NetIPAddr(ip netip.Addr) AppendBuffer {
	return ip.AppendTo(b)
}

func (b AppendBuffer) NetIPAddrPort(addr netip.AddrPort) AppendBuffer {
	return addr.AppendTo(b)
}

func (b AppendBuffer) Uint64(i uint64, base int) AppendBuffer {
	return strconv.AppendUint(b, i, base)
}

func (b AppendBuffer) Float64(f float64) AppendBuffer {
	return strconv.AppendFloat(b, f, 'f', -1, 64)
}

func (b AppendBuffer) Int64(i int64, base int) AppendBuffer {
	return strconv.AppendInt(b, i, base)
}

func (b AppendBuffer) Int(i int, base int) AppendBuffer {
	return strconv.AppendInt(b, int64(i), base)
}

func (b AppendBuffer) Uint(i uint, base int) AppendBuffer {
	return strconv.AppendUint(b, uint64(i), base)
}

func (b AppendBuffer) Bool(v bool) AppendBuffer {
	return strconv.AppendBool(b, v)
}

func (b AppendBuffer) Uint8(i uint8) AppendBuffer {
	return b.Uint64(uint64(i), 10)
}

func (b AppendBuffer) Uint16(i uint16) AppendBuffer {
	return b.Uint64(uint64(i), 10)
}

func (b AppendBuffer) Uint32(i uint32) AppendBuffer {
	return b.Uint64(uint64(i), 10)
}
func (b AppendBuffer) Cap() int {
	return cap(b)
}

func (b AppendBuffer) Pad(c byte, base int) AppendBuffer {
	n := (base - len(b)%base) % base
	if n == 0 {
		return b
	}
	if n <= 32 {
		b = append(b, make([]byte, 32)...)
		b = b[:len(b)+n-32]
	} else {
		b = append(b, make([]byte, n)...)
	}
	if c != 0 {
		m := len(b) - 1
		_ = b[m]
		for i := m - n + 1; i <= m; i++ {
			b[i] = c
		}
	}
	return b
}
