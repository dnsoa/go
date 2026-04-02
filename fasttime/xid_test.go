package fasttime

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewXID(t *testing.T) {
	id := NewXID()

	if id == nilXID {
		t.Fatal("expected non-nil XID")
	}
	if len(id) != 12 {
		t.Fatalf("expected 12 bytes, got %d", len(id))
	}
}

func TestNewXIDWithTime(t *testing.T) {
	now := time.Now()
	sec := now.Unix()
	id := NewXIDWithTime(sec)

	got := id.Time()
	// allow 1 second tolerance
	if diff := got.Unix() - sec; diff != 0 {
		t.Fatalf("expected timestamp %d, got %d (diff=%d)", sec, got.Unix(), diff)
	}
}

func TestXIDFields(t *testing.T) {
	id := NewXID()

	machine := id.Machine()
	if len(machine) != 3 {
		t.Fatalf("expected 3-byte machine id, got %d", len(machine))
	}

	pid := id.Pid()
	if pid == 0 {
		t.Fatal("expected non-zero pid")
	}

	counter1 := id.Counter()
	counter2 := NewXID().Counter()
	if counter2-counter1 != 1 {
		t.Fatalf("expected consecutive counters to differ by 1, got %d and %d", counter1, counter2)
	}
}

func TestXIDStringRoundTrip(t *testing.T) {
	ids := []XID{NewXID(), NewXID(), NewXID()}

	for _, id := range ids {
		s := id.String()
		if len(s) != 20 {
			t.Fatalf("expected 20-char string, got %d: %s", len(s), s)
		}

		parsed, err := ParseXID(s)
		if err != nil {
			t.Fatalf("ParseXID(%q) error: %v", s, err)
		}
		if parsed != id {
			t.Fatalf("round-trip failed: got %x, want %x", parsed, id)
		}
	}
}

func TestParseXIDErrors(t *testing.T) {
	_, err := ParseXID("")
	if err != ErrInvalidXID {
		t.Fatalf("expected ErrInvalidXID for empty string, got %v", err)
	}

	_, err = ParseXID("short")
	if err != ErrInvalidXID {
		t.Fatalf("expected ErrInvalidXID for short string, got %v", err)
	}

	_, err = ParseXID("abcdefghijklmnopqrstu") // 21 chars
	if err != ErrInvalidXID {
		t.Fatalf("expected ErrInvalidXID for long string, got %v", err)
	}

	_, err = ParseXID("0000000000000000000!") // invalid char
	if err != ErrInvalidXID {
		t.Fatalf("expected ErrInvalidXID for invalid char, got %v", err)
	}
}

func TestXIDJSON(t *testing.T) {
	id := NewXID()

	// Marshal
	data, err := json.Marshal(id)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	// Unmarshal
	var parsed XID
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	if parsed != id {
		t.Fatalf("JSON round-trip failed: got %x, want %x", parsed, id)
	}
}

func TestXIDJSONNull(t *testing.T) {
	// Marshal nilXID
	data, err := json.Marshal(nilXID)
	if err != nil {
		t.Fatalf("json.Marshal nilXID error: %v", err)
	}
	if string(data) != "null" {
		t.Fatalf("expected 'null', got %s", data)
	}

	// Unmarshal null
	var id XID
	if err := json.Unmarshal([]byte("null"), &id); err != nil {
		t.Fatalf("json.Unmarshal null error: %v", err)
	}
	if id != nilXID {
		t.Fatal("expected nilXID after unmarshaling null")
	}
}

func TestXIDJSONInvalid(t *testing.T) {
	var id XID
	err := json.Unmarshal([]byte(`""`), &id)
	if err == nil {
		t.Fatal("expected error for empty JSON string")
	}

	err = json.Unmarshal([]byte(`"short"`), &id)
	if err == nil {
		t.Fatal("expected error for short JSON string")
	}

	err = json.Unmarshal([]byte(`notjson`), &id)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestXIDTextMarshal(t *testing.T) {
	id := NewXID()

	text, err := id.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText error: %v", err)
	}

	var parsed XID
	if err := parsed.UnmarshalText(text); err != nil {
		t.Fatalf("UnmarshalText error: %v", err)
	}
	if parsed != id {
		t.Fatalf("Text round-trip failed: got %x, want %x", parsed, id)
	}
}

func TestXIDUniqueness(t *testing.T) {
	seen := make(map[XID]bool)
	for i := 0; i < 10000; i++ {
		id := NewXID()
		if seen[id] {
			t.Fatal("duplicate XID generated")
		}
		seen[id] = true
	}
}

func TestXIDConcurrency(t *testing.T) {
	const goroutines = 100
	const idsPerGoroutine = 1000

	ch := make(chan XID, goroutines*idsPerGoroutine)
	for g := 0; g < goroutines; g++ {
		go func() {
			for i := 0; i < idsPerGoroutine; i++ {
				ch <- NewXID()
			}
		}()
	}

	seen := make(map[XID]bool)
	for i := 0; i < goroutines*idsPerGoroutine; i++ {
		id := <-ch
		if seen[id] {
			t.Fatal("duplicate XID in concurrent generation")
		}
		seen[id] = true
	}
}

func BenchmarkXID(b *testing.B) {
	for b.Loop() {
		NewXID()
	}
}

func BenchmarkXIDString(b *testing.B) {
	id := NewXID()
	for b.Loop() {
		_ = id.String()
	}
}

func BenchmarkXIDParse(b *testing.B) {
	s := NewXID().String()
	for b.Loop() {
		ParseXID(s)
	}
}
