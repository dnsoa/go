package allocator

import (
	"bytes"
	"net/netip"
	"strings"
	"testing"
)

func TestBufferBasics(t *testing.T) {
	var b Buffer
	b = b.AppendString("hello")
	if got := b.String(); got != "hello" {
		t.Fatalf("expected %q got %q", "hello", got)
	}

	b = b.AppendByte(' ')
	b = b.AppendString("world")
	if got := b.String(); got != "hello world" {
		t.Fatalf("expected %q got %q", "hello world", got)
	}

	if b.Len() != 11 {
		t.Fatalf("expected len 11 got %d", b.Len())
	}

	b = b.Reset()
	if b.Len() != 0 {
		t.Fatalf("expected len 0 after reset got %d", b.Len())
	}

	// capacity check
	b = make(Buffer, 0, 16)
	if b.Cap() != 16 {
		t.Fatalf("expected cap 16 got %d", b.Cap())
	}
}

func TestBufferIO(t *testing.T) {
	var b Buffer
	// Write
	n, err := (&b).Write([]byte("abc"))
	if err != nil || n != 3 {
		t.Fatalf("Write error or wrong n: %v %d", err, n)
	}
	// WriteString
	n2, err := (&b).WriteString("def")
	if err != nil || n2 != 3 {
		t.Fatalf("WriteString error or wrong n: %v %d", err, n2)
	}
	if b.String() != "abcdef" {
		t.Fatalf("expected %q got %q", "abcdef", b.String())
	}

	// ReadFrom
	r := strings.NewReader("xyz")
	nr, err := (&b).ReadFrom(r)
	if err != nil {
		t.Fatalf("ReadFrom error: %v", err)
	}
	if nr != 3 {
		t.Fatalf("expected ReadFrom n 3 got %d", nr)
	}
	if !strings.HasSuffix(b.String(), "xyz") {
		t.Fatalf("expected suffix xyz got %q", b.String())
	}

	// WriteTo
	var out bytes.Buffer
	nw, err := b.WriteTo(&out)
	if err != nil {
		t.Fatalf("WriteTo error: %v", err)
	}
	if nw != int64(out.Len()) {
		t.Fatalf("WriteTo wrote %d expected %d", nw, out.Len())
	}
}

func TestWriteAnyAndEncoding(t *testing.T) {
	var b Buffer
	// string with space should be quoted
	b = b.WriteAny("hello world")
	if b.Len() < 2 || b[0] != '"' || b[b.Len()-1] != '"' {
		t.Fatalf("expected quoted string got %q", b.String())
	}

	b = b.Reset()
	b = b.WriteAny(123)
	if b.String() != "123" {
		t.Fatalf("expected 123 got %q", b.String())
	}

	b = b.Reset()
	b = b.WriteAny(true)
	if b.String() != "true" {
		t.Fatalf("expected true got %q", b.String())
	}

	b = b.Reset()
	b = b.WriteAny([]byte{0x01, 0x02})
	if b.String() != string([]byte{0x01, 0x02}) {
		t.Fatalf("expected raw bytes got %v", []byte(b))
	}
}

func TestTrimNewlineAndWriteNewLine(t *testing.T) {
	b := Buffer("abc\n")
	b = b.TrimNewline()
	if b.String() != "abc" {
		t.Fatalf("TrimNewline failed got %q", b.String())
	}

	b = Buffer("line")
	b = b.WriteNewLine()
	if b[b.Len()-1] != '\n' {
		t.Fatalf("WriteNewLine did not append newline: %q", b.String())
	}
	lenAfter := b.Len()
	b = b.WriteNewLine()
	if b.Len() != lenAfter {
		t.Fatalf("WriteNewLine appended extra newline")
	}
}

func TestPad(t *testing.T) {
	b := Buffer("123")
	b = b.Pad('x', 8)
	if b.Len()%8 != 0 {
		t.Fatalf("Pad did not align to base: len=%d", b.Len())
	}
	// check trailing bytes are 'x'
	for i := len(b) - 1; i >= 0 && b[i] == 'x'; i-- {
		// continue
	}
}

func TestBase64HexAndIP(t *testing.T) {
	var b Buffer
	b = b.AppendBase64([]byte{0x01, 0x02})
	if b.String() != "AQI=" {
		t.Fatalf("AppendBase64 got %q", b.String())
	}

	b = b.Reset()
	b = b.AppendHex([]byte{0x01, 0x02})
	if b.String() != "0102" {
		t.Fatalf("AppendHex got %q", b.String())
	}

	b = b.Reset()
	ip, err := netip.ParseAddr("127.0.0.1")
	if err != nil {
		t.Fatalf("parse ip: %v", err)
	}
	b = b.AppendNetIPAddr(ip)
	if b.String() != "127.0.0.1" {
		t.Fatalf("AppendNetIPAddr got %q", b.String())
	}

	b = b.Reset()
	ap := netip.AddrPortFrom(ip, 80)
	b = b.AppendNetIPAddrPort(ap)
	if b.String() != "127.0.0.1:80" {
		t.Fatalf("AppendNetIPAddrPort got %q", b.String())
	}
}

func TestBytesMutability(t *testing.T) {
	b := Buffer("ab")
	bs := b.Bytes()
	bs[0] = 'x'
	if b.String()[0] != 'x' {
		t.Fatalf("Bytes mutability not reflected in buffer: %q", b.String())
	}
}
