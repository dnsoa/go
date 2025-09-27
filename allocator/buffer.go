package allocator

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/netip"
	"os"
	"strconv"
	"time"
	"unsafe"
)

var _ io.Writer = (*Buffer)(nil)
var _ io.StringWriter = (*Buffer)(nil)
var _ io.ReaderFrom = (*Buffer)(nil)
var _ io.WriterTo = (*Buffer)(nil)

// Buffer is a thin wrapper around a byte slice.
type Buffer []byte

// AppendByte writes a single byte to the Buffer and returns the new Buffer.
func (b Buffer) AppendByte(v byte) Buffer {
	return append(b, v)
}

// AppendString writes a string to the Buffer and returns the new Buffer.
func (b Buffer) AppendString(s string) Buffer {
	return append(b, s...)
}

// AppendInt appends an integer to the underlying buffer (assuming base 10).
func (b Buffer) AppendInt(i int64) Buffer {
	return strconv.AppendInt(b, i, 10)
}

// AppendTime appends the time formatted using the specified layout.
func (b Buffer) AppendTime(t time.Time, layout string) Buffer {
	return t.AppendFormat(b, layout)
}

// AppendUint appends an unsigned integer to the underlying buffer (assuming
// base 10).
func (b Buffer) AppendUint(i uint64) Buffer {
	return strconv.AppendUint(b, i, 10)
}

// AppendBool appends a bool to the underlying buffer.
func (b Buffer) AppendBool(v bool) Buffer {
	return strconv.AppendBool(b, v)
}

// AppendFloat appends a float to the underlying buffer. It doesn't quote NaN
// or +/- Inf.
func (b Buffer) AppendFloat(f float64, bitSize int) Buffer {
	return strconv.AppendFloat(b, f, 'f', -1, bitSize)
}

func (b Buffer) AppendInt8(i int8) Buffer {
	return b.AppendInt(int64(i))
}

func (b Buffer) AppendInt16(i int16) Buffer {
	return b.AppendInt(int64(i))
}

func (b Buffer) AppendInt32(i int32) Buffer {
	return b.AppendInt(int64(i))
}

func (b Buffer) AppendInt64(i int64) Buffer {
	return b.AppendInt(i)
}

func (b Buffer) AppendUint8(i uint8) Buffer {
	return b.AppendUint(uint64(i))
}

func (b Buffer) AppendUint16(i uint16) Buffer {
	return b.AppendUint(uint64(i))
}

func (b Buffer) AppendUint32(i uint32) Buffer {
	return b.AppendUint(uint64(i))
}

func (b Buffer) AppendBase64(data []byte) Buffer {
	return base64.StdEncoding.AppendEncode(b, data)
}

func (b Buffer) AppendHex(data []byte) Buffer {
	return hex.AppendEncode(b, data)
}

func (b Buffer) AppendNetIPAddr(ip netip.Addr) Buffer {
	return ip.AppendTo(b)
}

func (b Buffer) AppendNetIPAddrPort(addr netip.AddrPort) Buffer {
	return addr.AppendTo(b)
}

func (b Buffer) AppendSpace() Buffer {
	return append(b, ' ')
}

func (b Buffer) AppendComma() Buffer {
	return append(b, ',')
}

// Len returns the length of the underlying byte slice.
func (b Buffer) Len() int {
	return len(b)
}

// Cap returns the capacity of the underlying byte slice.
func (b Buffer) Cap() int {
	return cap(b)
}

// Bytes returns a mutable reference to the underlying byte slice.
func (b Buffer) Bytes() []byte {
	return []byte(b)
}

// String returns a string copy of the underlying byte slice.
func (b Buffer) String() string {
	return unsafe.String(unsafe.SliceData(b), len(b))
}

// Reset resets the underlying byte slice and returns the cleared Buffer.
func (b Buffer) Reset() Buffer {
	return b[:0]
}

// AppendBytes appends a byte slice and returns the new Buffer.
func (b Buffer) AppendBytes(bs []byte) Buffer {
	return append(b, bs...)
}

// Write implements io.Writer. It appends data to the buffer.
func (b *Buffer) Write(p []byte) (int, error) {
	*b = append(*b, p...)
	return len(p), nil
}

// WriteString implements io.StringWriter. It appends string data to the buffer.
func (b *Buffer) WriteString(s string) (int, error) {
	*b = append(*b, s...)
	return len(s), nil
}

// ReadFrom implements io.ReaderFrom. It reads from r and appends to the buffer.
func (b *Buffer) ReadFrom(r io.Reader) (int64, error) {
	p := *b
	nStart := int64(len(p))
	nMax := int64(cap(p))
	n := nStart
	if nMax == 0 {
		nMax = 64
		p = make([]byte, nMax)
	} else {
		p = p[:nMax]
	}
	for {
		if n == nMax {
			nMax *= 2
			bNew := make([]byte, nMax)
			copy(bNew, p[:n])
			p = bNew
		}
		nn, err := r.Read(p[n:])
		if nn > 0 {
			n += int64(nn)
		}
		if err != nil {
			*b = p[:n]
			n -= nStart
			if err == io.EOF {
				return n, nil
			}
			return n, err
		}
	}
}

// WriteTo implements io.WriterTo. It writes buffer content to w.
func (b Buffer) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write([]byte(b))
	return int64(n), err
}

func needsQuote(s string) bool {
	for i := range s {
		c := s[i]
		if c < 0x20 || c > 0x7e || c == ' ' || c == '\\' || c == '"' || c == '\n' || c == '\r' {
			return true
		}
	}
	return false
}

// WriteAny writes a value to the buffer and returns the new Buffer.
func (b Buffer) WriteAny(value any) Buffer {
	switch fValue := value.(type) {
	case string:
		if needsQuote(fValue) {
			b = b.AppendString(strconv.Quote(fValue))
		} else {
			b = b.AppendString(fValue)
		}
	case int:
		b = b.AppendInt(int64(fValue))
	case int8:
		b = b.AppendInt(int64(fValue))
	case int16:
		b = b.AppendInt(int64(fValue))
	case int32:
		b = b.AppendInt(int64(fValue))
	case int64:
		b = b.AppendInt(int64(fValue))
	case uint:
		b = b.AppendUint(uint64(fValue))
	case uint8:
		b = b.AppendUint(uint64(fValue))
	case uint16:
		b = b.AppendUint(uint64(fValue))
	case uint32:
		b = b.AppendUint(uint64(fValue))
	case uint64:
		b = b.AppendUint(uint64(fValue))
	case float32:
		// float32 precision
		b = b.AppendFloat(float64(fValue), 32)
	case float64:
		b = b.AppendFloat(float64(fValue), 64)
	case bool:
		b = b.AppendBool(fValue)
	case error:
		b = b.AppendString(fValue.Error())
	case []byte:
		b = b.AppendBytes(fValue)
	case time.Time:
		b = b.AppendTime(fValue, time.RFC3339Nano)
	case time.Duration:
		b = b.AppendString(fValue.String())
	case json.Number:
		b = b.AppendString(fValue.String())
	default:
		js, err := json.Marshal(fValue)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		} else {
			b = b.AppendBytes(js)
		}
	}
	return b
}

// TrimNewline trims any final "\n" byte from the end of the buffer and returns the result.
func (b Buffer) TrimNewline() Buffer {
	if i := len(b) - 1; i >= 0 {
		if b[i] == '\n' {
			b = b[:i]
		}
	}
	return b
}

// WriteNewLine writes a new line to the buffer if it's needed and returns the result.
func (b Buffer) WriteNewLine() Buffer {
	if length := b.Len(); length > 0 && b[length-1] != '\n' {
		b = b.AppendByte('\n')
	}
	return b
}

func (b Buffer) Pad(c byte, base int) Buffer {
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
