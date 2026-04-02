package allocator

import "testing"

func TestGlobalDefaultCanBeReplaced(t *testing.T) {
	previous := ResetDefault()
	t.Cleanup(func() {
		SetDefault(previous)
	})

	custom := New()
	if old := SetDefault(custom); old == nil {
		t.Fatal("SetDefault should return previous pool")
	}
	if Default() != custom {
		t.Fatal("Default should return custom pool")
	}

	buf := Get(4)
	if custom.CurrentBytes() != 4 {
		t.Fatalf("Get should use custom allocator, bytes = %d", custom.CurrentBytes())
	}
	if err := Release(buf); err != nil {
		t.Fatalf("Release returned error: %v", err)
	}
	if custom.CurrentBytes() != 0 {
		t.Fatalf("Release should return buffer to custom allocator, bytes = %d", custom.CurrentBytes())
	}

	ResetDefault()
	if Default() == custom {
		t.Fatal("ResetDefault should replace custom pool")
	}
}

func TestGlobalReleaseReturnsCurrentPoolError(t *testing.T) {
	previous := ResetDefault()
	t.Cleanup(func() {
		SetDefault(previous)
	})

	custom := New()
	SetDefault(custom)

	buf := make(Buffer, 3)
	if err := Release(&buf); err == nil || err.Error() != "allocator Put() buffer cap must be 2^n" {
		t.Fatalf("Release should return allocator error, got %v", err)
	}
}

func TestSetDefaultNilKeepsCurrentPool(t *testing.T) {
	previous := ResetDefault()
	t.Cleanup(func() {
		SetDefault(previous)
	})

	custom := New()
	SetDefault(custom)

	returned := SetDefault(nil)
	if returned != custom {
		t.Fatal("SetDefault(nil) should return current pool")
	}
	if Default() != custom {
		t.Fatal("SetDefault(nil) should keep current pool")
	}
}
