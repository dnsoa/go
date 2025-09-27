package sync

import "testing"

func TestOnceFunc(t *testing.T) {
	var count int
	f := func() { count++ }
	onceF := OnceFunc(f)

	for i := 0; i < 100; i++ {
		onceF()
	}
	if count != 1 {
		t.Errorf("expected count=1, got %d", count)
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil function")
		}
	}()
	OnceFunc(nil)
}

func TestOnceValue(t *testing.T) {
	var calls int
	f := func() int { calls++; return 42 }
	onceF := OnceValue(f)

	for i := 0; i < 100; i++ {
		if v := onceF(); v != 42 {
			t.Errorf("expected 42, got %d", v)
		}
	}
	if calls != 1 {
		t.Errorf("expected calls=1, got %d", calls)
	}
}
