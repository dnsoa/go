package assert_test

import (
	"errors"
	"testing"

	"github.com/dnsoa/go/assert"
)

func TestNew(t *testing.T) {
	a := assert.New(t)
	if a == nil {
		t.Fatal("a.New() returned nil")
	}
	a.Equal(0, 0)
	a.Equal("Hello", "Hello")
	a.Equal(T{A: 10}, T{A: 10})

	a.NotEqual(0, 1)
	a.NotEqual("Hello", "World")
	a.NotEqual(&T{A: 10}, &T{A: 10})
	a.NotEqual(T{A: 10}, T{A: 20})

	a.DeepEqual(0, 0)
	a.DeepEqual("Hello", "Hello")
	a.DeepEqual([]byte("Hello"), []byte("Hello"))
	a.DeepEqual(T{A: 10}, T{A: 10})
	a.DeepEqual(&T{A: 10}, &T{A: 10})

	a.Equal(0, 0)

	a.NotEqual(0, 1)

	a.Contains("Hello", "H")
	a.Contains("Hello", "Hello")
	a.Contains([]string{"Hello", "World"}, "Hello")
	a.Contains([]int{1, 2, 3}, 2)
	a.Contains([]int{1, 2, 3}, []int{})
	a.Contains([]int{1, 2, 3}, []int{1, 2})
	a.Contains([]byte{'H', 'e', 'l', 'l', 'o'}, byte('e'))
	a.Contains([]byte{'H', 'e', 'l', 'l', 'o'}, []byte{'e', 'l'})
	a.Contains(map[string]int{"Hello": 1, "World": 2}, "Hello")

	a.NotContains("Hello", "h")
	a.NotContains("Hello", "hello")
	a.NotContains([]string{"Hello", "World"}, "hello")
	a.NotContains([]int{1, 2, 3}, 4)
	a.NotContains([]int{1, 2, 3}, []int{2, 1})
	a.NotContains([]int{1, 2, 3}, []int{1, 2, 3, 4})
	a.NotContains([]byte{'H', 'e', 'l', 'l', 'o'}, byte('a'))
	a.NotContains([]byte{'H', 'e', 'l', 'l', 'o'}, []byte{'l', 'e'})
	a.NotContains(map[string]int{"Hello": 1, "World": 2}, "hello")

	a.Contains("Hello", "H")

	a.NotContains("Hello", "H2")

	var (
		nilPointer   *T
		nilInterface any
		nilSlice     []byte
		nilMap       map[byte]byte
		nilChannel   chan byte
		nilFunction  func()
	)

	a.Nil(nil)
	a.Nil(nilPointer)
	a.Nil(nilInterface)
	a.Nil(nilSlice)
	a.Nil(nilMap)
	a.Nil(nilChannel)
	a.Nil(nilFunction)

	a.NotNil(0)
	a.NotNil("Hello")
	a.NotNil(T{})
	a.NotNil(&T{})
	a.NotNil(make([]byte, 0))
	a.NotNil(make(map[byte]byte))
	a.NotNil(make(chan byte))
	a.NotNil(TestNotNil)

	a.True(true)
	a.False(false)

	a.Empty(nil)
	a.Empty("")
	a.Empty(false)
	a.Empty(0)
	a.Empty(make([]byte, 0))
	ch := make(chan byte, 1)
	a.Empty(ch)

	a.NotEmpty(1)
	a.NotEmpty("Hello")
	a.NotEmpty([]byte{1})
	a.NotEmpty(map[byte]byte{1: 1})
	ch <- 1
	a.NotEmpty(ch)

	a.Error(errors.New("some error"))

	a.NoError(nil)
}
