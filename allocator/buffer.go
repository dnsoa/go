package allocator

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"
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

// IsEmpty returns true if the buffer has zero length.
func (b Buffer) IsEmpty() bool {
	return len(b) == 0
}

// Truncate discards all but the first n bytes from the buffer.
func (b Buffer) Truncate(n int) Buffer {
	if n < 0 {
		n = 0
	}
	if n > len(b) {
		n = len(b)
	}
	return b[:n]
}

// Grow grows the buffer's capacity to guarantee space for another n bytes.
// It returns the extended buffer.
func (b Buffer) Grow(n int) Buffer {
	if n <= 0 {
		return b
	}
	if len(b)+n > cap(b) {
		// Need to grow
		newCap := cap(b) * 2
		if newCap < len(b)+n {
			newCap = len(b) + n
		}
		newBuf := make(Buffer, len(b), newCap)
		copy(newBuf, b)
		return newBuf
	}
	return b
}

// Clone returns a copy of the buffer.
func (b Buffer) Clone() Buffer {
	if b == nil {
		return nil
	}
	clone := make(Buffer, len(b))
	copy(clone, b)
	return clone
}

// Equal returns true if b and x contain the same bytes.
func (b Buffer) Equal(x Buffer) bool {
	return bytes.Equal(b, x)
}

// EqualBytes returns true if b and x contain the same bytes.
func (b Buffer) EqualBytes(x []byte) bool {
	return bytes.Equal(b, x)
}

// EqualString returns true if b equals the string s.
func (b Buffer) EqualString(s string) bool {
	return string(b) == s
}

// Compare returns an integer comparing two buffers lexicographically.
func (b Buffer) Compare(x Buffer) int {
	return bytes.Compare(b, x)
}

// Contains reports whether subslice is within b.
func (b Buffer) Contains(subslice Buffer) bool {
	return bytes.Contains(b, subslice)
}

// ContainsBytes reports whether byte slice is within b.
func (b Buffer) ContainsBytes(subslice []byte) bool {
	return bytes.Contains(b, subslice)
}

// ContainsString reports whether s is within b.
func (b Buffer) ContainsString(s string) bool {
	return strings.Contains(string(b), s)
}

// ContainsByte reports whether byte c is within b.
func (b Buffer) ContainsByte(c byte) bool {
	return bytes.IndexByte(b, c) >= 0
}

// HasPrefix tests whether the buffer begins with prefix.
func (b Buffer) HasPrefix(prefix Buffer) bool {
	return bytes.HasPrefix(b, prefix)
}

// HasPrefixBytes tests whether the buffer begins with prefix.
func (b Buffer) HasPrefixBytes(prefix []byte) bool {
	return bytes.HasPrefix(b, prefix)
}

// HasPrefixString tests whether the buffer begins with prefix string.
func (b Buffer) HasPrefixString(prefix string) bool {
	return strings.HasPrefix(string(b), prefix)
}

// HasSuffix tests whether the buffer ends with suffix.
func (b Buffer) HasSuffix(suffix Buffer) bool {
	return bytes.HasSuffix(b, suffix)
}

// HasSuffixBytes tests whether the buffer ends with suffix.
func (b Buffer) HasSuffixBytes(suffix []byte) bool {
	return bytes.HasSuffix(b, suffix)
}

// HasSuffixString tests whether the buffer ends with suffix string.
func (b Buffer) HasSuffixString(suffix string) bool {
	return strings.HasSuffix(string(b), suffix)
}

// Index returns the index of the first instance of subslice in b, or -1 if subslice is not present in b.
func (b Buffer) Index(subslice Buffer) int {
	return bytes.Index(b, subslice)
}

// IndexByte returns the index of the first instance of c in b, or -1 if c is not present in b.
func (b Buffer) IndexByte(c byte) int {
	return bytes.IndexByte(b, c)
}

// IndexString returns the index of the first instance of s in b, or -1 if s is not present in b.
func (b Buffer) IndexString(s string) int {
	return strings.Index(string(b), s)
}

// LastIndex returns the index of the last instance of subslice in b, or -1 if subslice is not present in b.
func (b Buffer) LastIndex(subslice Buffer) int {
	return bytes.LastIndex(b, subslice)
}

// LastIndexByte returns the index of the last instance of c in b, or -1 if c is not present in b.
func (b Buffer) LastIndexByte(c byte) int {
	return bytes.LastIndexByte(b, c)
}

// TrimSpace returns a subslice of the buffer with all leading and trailing Unicode whitespace removed.
func (b Buffer) TrimSpace() Buffer {
	return bytes.TrimSpace(b)
}

// TrimPrefix returns a buffer without the provided leading prefix string.
// If the buffer doesn't start with prefix, it is returned unchanged.
func (b Buffer) TrimPrefix(prefix string) Buffer {
	if b.HasPrefixString(prefix) {
		return b[len(prefix):]
	}
	return b
}

// TrimSuffix returns a buffer without the provided trailing suffix string.
// If the buffer doesn't end with suffix, it is returned unchanged.
func (b Buffer) TrimSuffix(suffix string) Buffer {
	if b.HasSuffixString(suffix) {
		return b[:len(b)-len(suffix)]
	}
	return b
}

// Trim returns a buffer with all leading and trailing bytes contained in cutset removed.
func (b Buffer) Trim(cutset string) Buffer {
	return Buffer(strings.Trim(string(b), cutset))
}

// ToLower returns a copy of the buffer with all Unicode letters mapped to their lower case.
func (b Buffer) ToLower() Buffer {
	if b == nil {
		return nil
	}
	return bytes.ToLower(b)
}

// ToUpper returns a copy of the buffer with all Unicode letters mapped to their upper case.
func (b Buffer) ToUpper() Buffer {
	if b == nil {
		return nil
	}
	return bytes.ToUpper(b)
}

// Replace returns a copy of the buffer with the first n non-overlapping instances
// of old replaced by new. If n < 0, there is no limit on the number of replacements.
func (b Buffer) Replace(old, new []byte, n int) Buffer {
	return bytes.Replace(b, old, new, n)
}

// ReplaceAll returns a copy of the buffer with all non-overlapping instances
// of old replaced by new.
func (b Buffer) ReplaceAll(old, new []byte) Buffer {
	return bytes.ReplaceAll(b, old, new)
}

// Split slices b into all subslices separated by sep and returns a slice of the subslices between those separators.
func (b Buffer) Split(sep []byte) [][]byte {
	return bytes.Split(b, sep)
}

// SplitN slices b into subslices separated by sep and returns a slice of the subslices between those separators.
// The count determines the number of subslices to return.
func (b Buffer) SplitN(sep []byte, n int) [][]byte {
	return bytes.SplitN(b, sep, n)
}

// First returns the first n bytes of the buffer.
func (b Buffer) First(n int) Buffer {
	if n > len(b) {
		n = len(b)
	}
	if n < 0 {
		n = 0
	}
	return b[:n]
}

// Last returns the last n bytes of the buffer.
func (b Buffer) Last(n int) Buffer {
	if n > len(b) {
		n = len(b)
	}
	if n < 0 {
		n = 0
	}
	return b[len(b)-n:]
}

// AppendFormat formats according to a format specifier and appends to the buffer.
func (b Buffer) AppendFormat(format string, a ...any) Buffer {
	return append(b, fmt.Sprintf(format, a...)...)
}

// AppendJSON appends the JSON encoding of v to the buffer.
func (b Buffer) AppendJSON(v any) Buffer {
	js, err := json.Marshal(v)
	if err != nil {
		return b
	}
	return b.AppendBytes(js)
}

// AppendQuotedString appends s as a quoted JSON string to the buffer.
func (b Buffer) AppendQuotedString(s string) Buffer {
	return b.AppendString(strconv.Quote(s))
}

// Count counts the number of non-overlapping instances of subslice in b.
func (b Buffer) Count(subslice Buffer) int {
	return bytes.Count(b, subslice)
}

// Repeat returns a new buffer consisting of b count times.
func (b Buffer) Repeat(count int) Buffer {
	if count == 0 {
		return Buffer{}
	}
	if count < 0 {
		panic("negative repeat count")
	}
	result := make(Buffer, 0, len(b)*count)
	for i := 0; i < count; i++ {
		result = append(result, b...)
	}
	return result
}

// Join joins the elements of arr with b as the separator and returns the result.
func (b Buffer) Join(arr []Buffer) Buffer {
	if len(arr) == 0 {
		return Buffer{}
	}
	if len(arr) == 1 {
		return arr[0].Clone()
	}
	// Calculate total size
	totalLen := len(b) * (len(arr) - 1)
	for _, v := range arr {
		totalLen += len(v)
	}
	result := make(Buffer, 0, totalLen)
	for i, v := range arr {
		if i > 0 {
			result = append(result, b...)
		}
		result = append(result, v...)
	}
	return result
}

// Runes returns a slice of runes (Unicode code points) equivalent to the buffer.
func (b Buffer) Runes() []rune {
	return bytes.Runes(b)
}

// HasRune reports whether the buffer contains the specified Unicode code point.
func (b Buffer) HasRune(r rune) bool {
	return bytes.ContainsRune(b, r)
}

// ToTitle returns a copy of the buffer with all Unicode letters mapped to their title case.
func (b Buffer) ToTitle() Buffer {
	if b == nil {
		return nil
	}
	return bytes.ToTitle(b)
}

// IsASCII returns true if b contains only ASCII characters.
func (b Buffer) IsASCII() bool {
	for _, c := range b {
		if c > unicode.MaxASCII {
			return false
		}
	}
	return true
}

// Minimize returns a buffer with capacity equal to its length, freeing unused memory.
func (b Buffer) Minimize() Buffer {
	if cap(b) == len(b) {
		return b
	}
	result := make(Buffer, len(b))
	copy(result, b)
	return result
}

// Swap swaps the values of indices i and j.
func (b Buffer) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

// Reverse reverses the buffer in place.
func (b Buffer) Reverse() Buffer {
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
	return b
}

// Unique removes consecutive duplicate bytes from the buffer.
func (b Buffer) Unique() Buffer {
	if len(b) == 0 {
		return b
	}
	result := make(Buffer, 0, len(b))
	result = append(result, b[0])
	for i := 1; i < len(b); i++ {
		if b[i] != b[i-1] {
			result = append(result, b[i])
		}
	}
	return result
}
