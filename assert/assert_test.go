package assert_test

import (
	"errors"
	"testing"

	"github.com/dnsoa/go/assert"
)

// noop implements the Test interface with noop functions so you can test the failure cases.
type noop struct{}

// Errorf does nothing.
func (t *noop) Errorf(format string, args ...any) {}

// FailNow does nothing.
func (t *noop) FailNow() {}

// fail creates a new test that is expected to fail.
func fail(_ *testing.T) *noop {
	return &noop{}
}

type T struct{ A int }

func TestEqual(t *testing.T) {
	assert.Equal(t, 0, 0)
	assert.Equal(t, "Hello", "Hello")
	assert.Equal(t, T{A: 10}, T{A: 10})
	a := &T{A: 10}
	b := &T{A: 10}
	assert.Equal(t, a, b)
}

func TestNotEqual(t *testing.T) {
	assert.NotEqual(t, 0, 1)
	assert.NotEqual(t, "Hello", "World")
	assert.NotEqual(t, &T{A: 30}, &T{A: 10})
	assert.NotEqual(t, T{A: 10}, T{A: 20})
}

func TestDeepEqual(t *testing.T) {
	assert.DeepEqual(t, 0, 0)
	assert.DeepEqual(t, "Hello", "Hello")
	assert.DeepEqual(t, []byte("Hello"), []byte("Hello"))
	assert.DeepEqual(t, T{A: 10}, T{A: 10})
	assert.DeepEqual(t, &T{A: 10}, &T{A: 10})
}

func TestFailEqual(t *testing.T) {
	assert.Equal(fail(t), 0, 1)
}

func TestFailNotEqual(t *testing.T) {
	assert.NotEqual(fail(t), 0, 0)
}

func TestFailDeepEqual(t *testing.T) {
	assert.DeepEqual(fail(t), "Hello", "World")
}

func TestContains(t *testing.T) {
	assert.Contains(t, "Hello", "H")
	assert.Contains(t, "Hello", "Hello")
	assert.Contains(t, []string{"Hello", "World"}, "Hello")
	assert.Contains(t, []int{1, 2, 3}, 2)
	assert.Contains(t, []int{1, 2, 3}, []int{3})
	assert.Contains(t, []int{1, 2, 3}, []int{1, 2})
	assert.Contains(t, []byte{'H', 'e', 'l', 'l', 'o'}, byte('e'))
	assert.Contains(t, []byte{'H', 'e', 'l', 'l', 'o'}, []byte{'e', 'l'})
	assert.Contains(t, map[string]int{"Hello": 1, "World": 2}, "Hello")
}

func TestNotContains(t *testing.T) {
	assert.NotContains(t, "Hello", "h")
	assert.NotContains(t, "Hello", "hello")
	assert.NotContains(t, []string{"Hello", "World"}, "hello")
	assert.NotContains(t, []int{1, 2, 3}, 4)
	assert.NotContains(t, []int{1, 2, 3}, []int{2, 1})
	assert.NotContains(t, []int{1, 2, 3}, []int{1, 2, 3, 4})
	assert.NotContains(t, []byte{'H', 'e', 'l', 'l', 'o'}, byte('a'))
	assert.NotContains(t, []byte{'H', 'e', 'l', 'l', 'o'}, []byte{'l', 'e'})
	assert.NotContains(t, map[string]int{"Hello": 1, "World": 2}, "hello")
}

func TestFailContains(t *testing.T) {
	assert.Contains(fail(t), "Hello", "h")
}

func TestFailNotContains(t *testing.T) {
	assert.NotContains(fail(t), "Hello", "H")
}

func TestNil(t *testing.T) {
	var (
		nilPointer   *T
		nilInterface any
		nilSlice     []byte
		nilMap       map[byte]byte
		nilChannel   chan byte
		nilFunction  func()
	)

	assert.Nil(t, nil)
	assert.Nil(t, nilPointer)
	assert.Nil(t, nilInterface)
	assert.Nil(t, nilSlice)
	assert.Nil(t, nilMap)
	assert.Nil(t, nilChannel)
	assert.Nil(t, nilFunction)
}

func TestNotNil(t *testing.T) {
	assert.NotNil(t, 1)
	assert.NotNil(t, "Hello")
	assert.NotNil(t, T{})
	assert.NotNil(t, &T{})
	assert.NotNil(t, make([]byte, 0))
	assert.NotNil(t, make(map[byte]byte))
	assert.NotNil(t, make(chan byte))
	assert.NotNil(t, TestNotNil)
}

func TestFailNil(t *testing.T) {
	assert.Nil(fail(t), 0)
}

func TestFailNotNil(t *testing.T) {
	assert.NotNil(fail(t), nil)
}

func TestTrue(t *testing.T) {
	assert.True(t, true)
	assert.False(t, false)
}

func TestEmpty(t *testing.T) {
	assert.Empty(t, nil)
	assert.Empty(t, "")
	assert.Empty(t, false)
	assert.Empty(t, 0)
	assert.Empty(t, make([]byte, 0))
	assert.Empty(t, make(chan byte))
	var a *struct{}
	assert.Empty(t, a)
	var b = &T{
		A: 0,
	}
	assert.Empty(t, b)
	assert.Empty(fail(t), 1)
}

func TestNotEmpty(t *testing.T) {
	assert.NotEmpty(t, 1)
	assert.NotEmpty(t, "Hello")
	assert.NotEmpty(t, []byte{1})
	assert.NotEmpty(t, map[byte]byte{1: 1})
	ch := make(chan byte, 1)
	ch <- 1
	assert.NotEmpty(t, ch)
	assert.NotEmpty(fail(t), make(chan byte, 1))
}

func TestError(t *testing.T) {
	assert.Error(t, errors.New("some error"))
	assert.Error(fail(t), nil)
}

func TestNoError(t *testing.T) {
	assert.NoError(t, nil)
	assert.NoError(fail(t), errors.New("some error"))
}

func TestLen(t *testing.T) {
	assert.Len(t, "Hello", 5)
	assert.Len(t, []byte("Hello"), 5)
	assert.Len(t, []int{1, 2, 3}, 3)
	assert.Len(t, map[byte]byte{1: 1}, 1)
	assert.Len(fail(t), &T{}, 4)
	assert.Len(fail(t), "Hello", 1)

}
