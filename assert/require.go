package assert

type Assertions struct {
	t TestingT
}

type tHelper interface {
	Helper()
}

// New makes a new Assertions object for the specified TestingT.
func New(t TestingT) *Assertions {
	return &Assertions{
		t: t,
	}
}

// Contains asserts that the specified string, list(array, slice...) or map contains the
// specified substring or element.
//
//	a.Contains("Hello World", "World")
//	a.Contains(["Hello", "World"], "World")
//	a.Contains({"Hello": "World"}, "Hello")
func (a *Assertions) Contains(s any, contains any) {
	if h, ok := a.t.(tHelper); ok {
		h.Helper()
	}
	Contains(a.t, s, contains)
}

// NotContains asserts that the specified string, list(array, slice...) or map does not
// contain the specified substring or element.
//
//	a.NotContains("Hello World", "World")
//	a.NotContains(["Hello", "World"], "World")
//	a.NotContains({"Hello": "World"}, "Hello")
func (a *Assertions) NotContains(s any, notContains any) {
	if h, ok := a.t.(tHelper); ok {
		h.Helper()
	}
	NotContains(a.t, s, notContains)
}

// Empty asserts that the specified object is empty.  I.e. nil, "", false, 0 or either
// a slice or a channel with len == 0.
//
//	a.Empty(obj)
func (a *Assertions) Empty(object any) {
	if h, ok := a.t.(tHelper); ok {
		h.Helper()
	}
	Empty(a.t, object)
}

// NotEmpty asserts that the specified object is not empty.  I.e. not nil, "", false, 0 or
// either a slice or a channel with len == 0.
//
//	a.NotEmpty(obj)
func (a *Assertions) NotEmpty(object any) {
	if h, ok := a.t.(tHelper); ok {
		h.Helper()
	}
	NotEmpty(a.t, object)
}

// Equal asserts that two objects are equal.
//
//	a.Equal(123, 123)
//
// Pointer variable equality is determined based on the equality of the
// referenced values (as opposed to the memory addresses). Function equality
// cannot be determined and will always fail.
func (a *Assertions) Equal(expected any, actual any) {
	if h, ok := a.t.(tHelper); ok {
		h.Helper()
	}
	Equal(a.t, expected, actual)
}

// DeepEqual asserts that two objects are deeply equal.
// This is a deep comparison, so it will check the values of all fields
// in the struct, and all elements in the array/slice.
// It will also check the values of all keys in the map.
func (a *Assertions) DeepEqual(expected any, actual any) {
	if h, ok := a.t.(tHelper); ok {
		h.Helper()
	}
	DeepEqual(a.t, expected, actual)
}

// Error asserts that a function returned an error (i.e. not `nil`).
//
//	  actualObj, err := SomeFunction()
//	  if a.Error(err) {
//		   assert.Equal(t, expectedError, err)
//	  }
func (a *Assertions) Error(err error) {
	if h, ok := a.t.(tHelper); ok {
		h.Helper()
	}
	Error(a.t, err)
}

// False asserts that the specified value is false.
//
//	a.False(myBool)
func (a *Assertions) False(value bool) {
	if h, ok := a.t.(tHelper); ok {
		h.Helper()
	}
	False(a.t, value)
}

// Nil asserts that the specified object is nil.
//
//	a.Nil(err)
func (a *Assertions) Nil(object any) {
	if h, ok := a.t.(tHelper); ok {
		h.Helper()
	}
	Nil(a.t, object)
}

// NoError asserts that a function returned no error (i.e. `nil`).
//
//	  actualObj, err := SomeFunction()
//	  if a.NoError(err) {
//		   assert.Equal(t, expectedObj, actualObj)
//	  }
func (a *Assertions) NoError(err error) {
	if h, ok := a.t.(tHelper); ok {
		h.Helper()
	}
	NoError(a.t, err)
}

// NotEqual asserts that the specified values are NOT equal.
//
//	a.NotEqual(obj1, obj2)
//
// Pointer variable equality is determined based on the equality of the
// referenced values (as opposed to the memory addresses).
func (a *Assertions) NotEqual(expected any, actual any) {
	if h, ok := a.t.(tHelper); ok {
		h.Helper()
	}
	NotEqual(a.t, expected, actual)
}

// NotNil asserts that the specified object is not nil.
//
//	a.NotNil(err)
func (a *Assertions) NotNil(object any) {
	if h, ok := a.t.(tHelper); ok {
		h.Helper()
	}
	NotNil(a.t, object)
}

// True asserts that the specified value is true.
//
//	a.True(myBool)
func (a *Assertions) True(value bool) {
	if h, ok := a.t.(tHelper); ok {
		h.Helper()
	}
	True(a.t, value)
}
