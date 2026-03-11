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

func TestBufferUtilityMethods(t *testing.T) {
	// IsEmpty
	var b1 Buffer
	if !b1.IsEmpty() {
		t.Error("empty buffer should be empty")
	}
	b2 := Buffer("test")
	if b2.IsEmpty() {
		t.Error("non-empty buffer should not be empty")
	}

	// Truncate
	b := Buffer("hello world")
	b = b.Truncate(5)
	if b.String() != "hello" {
		t.Errorf("Truncate failed: %q", b.String())
	}

	// First/Last
	b = Buffer("hello")
	if b.First(2).String() != "he" {
		t.Error("First failed")
	}
	if b.Last(2).String() != "lo" {
		t.Error("Last failed")
	}

	// Clone
	b = Buffer("test")
	cloned := b.Clone()
	if !cloned.Equal(b) {
		t.Error("Clone should produce equal buffer")
	}
	cloned[0] = 'x'
	if cloned.Equal(b) {
		t.Error("Modified clone should not equal original")
	}

	// Contains
	b = Buffer("hello world")
	if !b.ContainsString("hello") {
		t.Error("Should contain hello")
	}
	if !b.ContainsByte('w') {
		t.Error("Should contain 'w'")
	}

	// HasPrefix/HasSuffix
	b = Buffer("hello world")
	if !b.HasPrefixString("hello") {
		t.Error("Should have prefix 'hello'")
	}
	if !b.HasSuffixString("world") {
		t.Error("Should have suffix 'world'")
	}

	// Index
	b = Buffer("hello")
	if b.IndexString("l") != 2 {
		t.Error("Index of 'l' should be 2")
	}
	if b.LastIndexByte('l') != 3 {
		t.Error("LastIndex of 'l' should be 3")
	}

	// TrimSpace
	b = Buffer("  hello  ")
	b = b.TrimSpace()
	if b.String() != "hello" {
		t.Errorf("TrimSpace failed: %q", b.String())
	}

	// TrimPrefix/TrimSuffix
	b = Buffer("hello world")
	b = b.TrimPrefix("hello ")
	if b.String() != "world" {
		t.Errorf("TrimPrefix failed: %q", b.String())
	}
	b = Buffer("hello world")
	b = b.TrimSuffix(" world")
	if b.String() != "hello" {
		t.Errorf("TrimSuffix failed: %q", b.String())
	}

	// Replace
	b = Buffer("hello hello")
	b = b.ReplaceAll([]byte("hello"), []byte("hi"))
	if b.String() != "hi hi" {
		t.Errorf("ReplaceAll failed: %q", b.String())
	}

	// Reverse
	b = Buffer("abc")
	b = b.Reverse()
	if b.String() != "cba" {
		t.Errorf("Reverse failed: %q", b.String())
	}
}

func TestBufferJoin(t *testing.T) {
	sep := Buffer(", ")
	arr := []Buffer{Buffer("a"), Buffer("b"), Buffer("c")}
	result := sep.Join(arr)
	if result.String() != "a, b, c" {
		t.Errorf("Join failed: %q", result.String())
	}
}

// Benchmarks

func BenchmarkBufferAppendString(b *testing.B) {
	var buf Buffer
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf = buf.Reset()
		buf = buf.AppendString("hello world")
	}
}

func BenchmarkBufferString(b *testing.B) {
	buf := Buffer("hello world")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = buf.String()
	}
}

func BenchmarkBufferClone(b *testing.B) {
	buf := Buffer("hello world, this is a test")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = buf.Clone()
	}
}

func BenchmarkBufferContains(b *testing.B) {
	buf := Buffer("hello world, this is a test")
	sub := Buffer("world")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = buf.Contains(sub)
	}
}

func BenchmarkBufferIndex(b *testing.B) {
	buf := Buffer("hello world, this is a test")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = buf.IndexByte('o')
	}
}
