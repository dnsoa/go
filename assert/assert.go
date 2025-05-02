package assert

import (
	"reflect"
	"runtime/debug"
	"strings"
)

// TestingT is the interface used for tests.
type TestingT interface {
	Errorf(format string, args ...any)
	FailNow()
}

// Equal asserts that the two parameters are equal.
func Equal[T comparable](t TestingT, a T, b T) {
	if a == b {
		return
	}

	t.Errorf(twoParameters, file(), "Equal", a, b)
	t.FailNow()
}

// NotEqual asserts that the two parameters are not equal.
func NotEqual[T comparable](t TestingT, a T, b T) {
	if a != b {
		return
	}

	t.Errorf(twoParameters, file(), "NotEqual", a, b)
	t.FailNow()
}

// DeepEqual asserts that the two parameters are deeply equal.
func DeepEqual[T any](t TestingT, a T, b T) {
	if reflect.DeepEqual(a, b) {
		return
	}

	t.Errorf(twoParameters, file(), "DeepEqual", a, b)
	t.FailNow()
}

// Contains asserts that a contains b.
func Contains(t TestingT, a any, b any) {
	if contains(a, b) {
		return
	}

	t.Errorf(twoParameters, file(), "Contains", a, b)
	t.FailNow()
}

// NotContains asserts that a doesn't contain b.
func NotContains(t TestingT, a any, b any) {
	if !contains(a, b) {
		return
	}

	t.Errorf(twoParameters, file(), "NotContains", a, b)
	t.FailNow()
}

// contains returns whether container contains the given the element.
// It works with strings, maps and slices.
func contains(container any, element any) bool {
	containerValue := reflect.ValueOf(container)

	switch containerValue.Kind() {
	case reflect.String:
		elementValue := reflect.ValueOf(element)
		return strings.Contains(containerValue.String(), elementValue.String())

	case reflect.Map:
		keys := containerValue.MapKeys()

		for _, key := range keys {
			if key.Interface() == element {
				return true
			}
		}

	case reflect.Slice:
		elementValue := reflect.ValueOf(element)

		if elementValue.Kind() == reflect.Slice {
			elementLength := elementValue.Len()

			if elementLength == 0 {
				return true
			}

			if elementLength > containerValue.Len() {
				return false
			}

			matchingElements := 0

			for i := 0; i < containerValue.Len(); i++ {
				if containerValue.Index(i).Interface() == elementValue.Index(matchingElements).Interface() {
					matchingElements++
				} else {
					matchingElements = 0
				}

				if matchingElements == elementLength {
					return true
				}
			}

			return false
		}

		for i := 0; i < containerValue.Len(); i++ {
			if containerValue.Index(i).Interface() == element {
				return true
			}
		}
	}

	return false
}

const oneParameter = `
  %s
󰅙  assert.%s
    󰯬  %v`

const twoParameters = `
  %s
󰅙  assert.%s
    󰯬  %v
    󰯯  %v`

// file returns the first line containing "_test.go" in the debug stack.
func file() string {
	stack := string(debug.Stack())
	lines := strings.Split(stack, "\n")
	name := ""

	for _, line := range lines {
		if strings.Contains(line, "_test.go") {
			space := strings.LastIndex(line, " ")

			if space != -1 {
				line = line[:space]
			}

			name = strings.TrimSpace(line)
			break
		}
	}

	return name
}

// Nil asserts that the given parameter equals nil.
func Nil(t TestingT, a any) {
	if isNil(a) {
		return
	}

	t.Errorf(oneParameter, file(), "Nil", a)
	t.FailNow()
}

// NotNil asserts that the given parameter does not equal nil.
func NotNil(t TestingT, a any) {
	if !isNil(a) {
		return
	}

	t.Errorf(oneParameter, file(), "NotNil", a)
	t.FailNow()
}

// isNil returns true if the object is nil.
func isNil(object any) bool {
	if object == nil {
		return true
	}

	value := reflect.ValueOf(object)

	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Ptr, reflect.UnsafePointer, reflect.Interface, reflect.Slice:
		return value.IsNil()
	}

	return false
}

// True asserts that the given parameter is true.
func True(t TestingT, a bool) {
	Equal(t, a, true)
}

// False asserts that the given parameter is false.
func False(t TestingT, a bool) {
	Equal(t, a, false)
}

// Empty asserts that the given parameter is empty.
func Empty(t TestingT, a any) {
	if isEmpty(a) {
		return
	}

	t.Errorf(oneParameter, file(), "Empty", a)
	t.FailNow()
}

// isEmpty gets whether the specified object is considered empty or not.
func isEmpty(object any) bool {

	// get nil case out of the way
	if object == nil {
		return true
	}

	objValue := reflect.ValueOf(object)

	switch objValue.Kind() {
	// collection types are empty when they have no element
	case reflect.Chan, reflect.Map, reflect.Slice:
		return objValue.Len() == 0
	// pointers are empty if nil or if the value they point to is empty
	case reflect.Ptr:
		if objValue.IsNil() {
			return true
		}
		deref := objValue.Elem().Interface()
		return isEmpty(deref)
	// for all other types, compare against the zero value
	// array types are empty when they match their zero-initialized state
	default:
		zero := reflect.Zero(objValue.Type())
		return reflect.DeepEqual(object, zero.Interface())
	}
}

// NotEmpty asserts that the specified object is not empty.  I.e. not nil, "", false, 0 or
// either a slice or a channel with len == 0.
func NotEmpty(t TestingT, a any) {
	if !isEmpty(a) {
		return
	}
	t.Errorf(oneParameter, file(), "NotEmpty", a)
	t.FailNow()

}

// Error asserts that the given error is not nil.
func Error(t TestingT, err error) {
	if err != nil {
		return
	}

	t.Errorf(oneParameter, file(), "Error", err)
	t.FailNow()
}

// NoError asserts that the given error is nil.
func NoError(t TestingT, err error) {
	if err != nil {
		t.Errorf(oneParameter, file(), "NoError", err)
		t.FailNow()
	}
}
