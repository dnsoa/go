package assert

import (
	"bufio"
	"bytes"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"unicode"
	"unicode/utf8"
)

// TestingT is the interface used for tests.
type TestingT interface {
	Errorf(format string, args ...any)
	FailNow()
}

// Equal asserts that the two parameters are equal.
func Equal[T comparable](t TestingT, expected T, actual T, msgAndArgs ...any) {
	if objectsAreEqual(expected, actual) {
		return
	}
	if h, ok := t.(tHelper); ok {
		h.Helper()
	}

	Fail(t, fmt.Sprintf("Not equal: \n"+
		"expected: %v\n"+
		"actual  : %v", expected, actual), msgAndArgs...)
	t.FailNow()
}

// NotEqual asserts that the two parameters are not equal.
func NotEqual[T comparable](t TestingT, expected T, actual T, msgAndArgs ...any) {
	if !objectsAreEqual(expected, actual) {
		return
	}
	if h, ok := t.(tHelper); ok {
		h.Helper()
	}

	Fail(t, fmt.Sprintf("Should not be: %#v\n", actual), msgAndArgs...)
	t.FailNow()
}

// DeepEqual asserts that the two parameters are deeply equal.
func DeepEqual[T any](t TestingT, expected T, actual T, msgAndArgs ...any) {
	if reflect.DeepEqual(actual, expected) {
		return
	}
	if h, ok := t.(tHelper); ok {
		h.Helper()
	}

	Fail(t, fmt.Sprintf("Not deep equal: \n"+
		"expected: %#v\n"+
		"actual  : %#v", expected, actual), msgAndArgs...)
	t.FailNow()
}

// Contains asserts that a contains b.
func Contains(t TestingT, a any, b any, msgAndArgs ...any) {
	if contains(a, b) {
		return
	}
	if h, ok := t.(tHelper); ok {
		h.Helper()
	}

	Fail(t, fmt.Sprintf("%#v does not contain %#v", a, b), msgAndArgs...)
	t.FailNow()
}

// NotContains asserts that a doesn't contain b.
func NotContains(t TestingT, a any, b any, msgAndArgs ...any) {
	if !contains(a, b) {
		return
	}
	if h, ok := t.(tHelper); ok {
		h.Helper()
	}

	Fail(t, fmt.Sprintf("%#v should not contain %#v", a, b), msgAndArgs...)
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
				return false
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

// Nil asserts that the given parameter equals nil.
func Nil(t TestingT, a any, msgAndArgs ...any) {
	if isNil(a) {
		return
	}
	if h, ok := t.(tHelper); ok {
		h.Helper()
	}

	Fail(t, fmt.Sprintf("Not nil: \n"+
		"expected: %v\n"+
		"actual  : %v", nil, a), msgAndArgs...)
	t.FailNow()
}

// NotNil asserts that the given parameter does not equal nil.
func NotNil(t TestingT, a any, msgAndArgs ...any) {
	if !isNil(a) {
		return
	}
	if h, ok := t.(tHelper); ok {
		h.Helper()
	}

	Fail(t, "Should not be nil", msgAndArgs...)
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
func True(t TestingT, actual bool, msgAndArgs ...any) {
	if actual {
		return
	}
	if h, ok := t.(tHelper); ok {
		h.Helper()
	}

	Fail(t, "Should be true", msgAndArgs...)
	t.FailNow()
}

// False asserts that the given parameter is false.
func False(t TestingT, actual bool, msgAndArgs ...any) {
	if !actual {
		return
	}
	if h, ok := t.(tHelper); ok {
		h.Helper()
	}
	Fail(t, "Should be false", msgAndArgs...)
	t.FailNow()
}

// Empty asserts that the given parameter is empty.
func Empty(t TestingT, a any, msgAndArgs ...any) {
	if isEmpty(a) {
		return
	}
	if h, ok := t.(tHelper); ok {
		h.Helper()
	}

	Fail(t, fmt.Sprintf("Should be empty, but was %v", a), msgAndArgs...)
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
func NotEmpty(t TestingT, object any, msgAndArgs ...any) {
	if !isEmpty(object) {
		return
	}
	if h, ok := t.(tHelper); ok {
		h.Helper()
	}

	Fail(t, fmt.Sprintf("Should NOT be empty, but was %v", object), msgAndArgs...)
	t.FailNow()

}

// Error asserts that the given error is not nil.
func Error(t TestingT, err error, msgAndArgs ...any) {
	if err != nil {
		return
	}
	if h, ok := t.(tHelper); ok {
		h.Helper()
	}

	Fail(t, "An error is expected but got nil.", msgAndArgs...)
	t.FailNow()
}

// NoError asserts that the given error is nil.
func NoError(t TestingT, err error, msgAndArgs ...any) {
	if err != nil {
		if h, ok := t.(tHelper); ok {
			h.Helper()
		}
		Fail(t, fmt.Sprintf("Received unexpected error:\n%+v", err), msgAndArgs...)
		t.FailNow()
	}
}

// getLen tries to get the length of an object.
// It returns (0, false) if impossible.
func getLen(x any) (length int, ok bool) {
	v := reflect.ValueOf(x)
	defer func() {
		ok = recover() == nil
	}()
	return v.Len(), true
}

// Len asserts that the specified object has specific length.
// Len also fails if the object has a type that len() not accept.
//
//	assert.Len(t, mySlice, 3)
func Len[T ~int | ~int8 | ~int16 | ~int32 | ~int64 |
	~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64](t TestingT, object any, length T, msgAndArgs ...any) {
	if h, ok := t.(tHelper); ok {
		h.Helper()
	}

	l, ok := getLen(object)
	if !ok {
		Fail(t, fmt.Sprintf("\"%v\" could not be applied builtin len()", object), msgAndArgs...)
		t.FailNow()
		return
	}

	if l != int(length) {
		Fail(t, fmt.Sprintf("Should have %d item(s), but has %d", length, l), msgAndArgs...)
		t.FailNow()
	}
}

// Fail reports a failure through
func Fail(t TestingT, failureMessage string, msgAndArgs ...any) {
	if h, ok := t.(tHelper); ok {
		h.Helper()
	}
	var content []labeledContent
	callInfo := CallerInfo()
	if len(callInfo) > 0 {
		content = append(content, labeledContent{"Trace", strings.Join(callInfo, "\n\t\t\t")})
	}
	content = append(content, labeledContent{"Error", failureMessage})

	// Add test name if the Go version supports it
	if n, ok := t.(interface {
		Name() string
	}); ok {
		content = append(content, labeledContent{"Test", n.Name()})
	}

	message := messageFromMsgAndArgs(msgAndArgs...)
	if len(message) > 0 {
		content = append(content, labeledContent{"Messages", message})
	}

	t.Errorf("\n%s", ""+labeledOutput(content...))
}

type labeledContent struct {
	label   string
	content string
}

// labeledOutput returns a string consisting of the provided labeledContent. Each labeled output is appended in the following manner:
//
//	\t{{label}}:{{align_spaces}}\t{{content}}\n
//
// The initial carriage return is required to undo/erase any padding added by testing.T.Errorf. The "\t{{label}}:" is for the label.
// If a label is shorter than the longest label provided, padding spaces are added to make all the labels match in length. Once this
// alignment is achieved, "\t{{content}}\n" is added for the output.
//
// If the content of the labeledOutput contains line breaks, the subsequent lines are aligned so that they start at the same location as the first line.
func labeledOutput(content ...labeledContent) string {
	longestLabel := 0
	for _, v := range content {
		if len(v.label) > longestLabel {
			longestLabel = len(v.label)
		}
	}
	var output string
	for _, v := range content {
		output += "\t" + v.label + ":" + strings.Repeat(" ", longestLabel-len(v.label)) + "\t" + indentMessageLines(v.content, longestLabel) + "\n"
	}
	return output
}

func messageFromMsgAndArgs(msgAndArgs ...any) string {
	if len(msgAndArgs) == 0 || msgAndArgs == nil {
		return ""
	}
	if len(msgAndArgs) == 1 {
		msg := msgAndArgs[0]
		if msgAsStr, ok := msg.(string); ok {
			return msgAsStr
		}
		return fmt.Sprintf("%+v", msg)
	}
	if len(msgAndArgs) > 1 {
		return fmt.Sprintf(msgAndArgs[0].(string), msgAndArgs[1:]...)
	}
	return ""
}

// Aligns the provided message so that all lines after the first line start at the same location as the first line.
// Assumes that the first line starts at the correct location (after carriage return, tab, label, spacer and tab).
// The longestLabelLen parameter specifies the length of the longest label in the output (required because this is the
// basis on which the alignment occurs).
func indentMessageLines(message string, longestLabelLen int) string {
	outBuf := new(bytes.Buffer)

	for i, scanner := 0, bufio.NewScanner(strings.NewReader(message)); scanner.Scan(); i++ {
		// no need to align first line because it starts at the correct location (after the label)
		if i != 0 {
			// append alignLen+1 spaces to align with "{{longestLabel}}:" before adding tab
			outBuf.WriteString("\n\t" + strings.Repeat(" ", longestLabelLen+1) + "\t")
		}
		outBuf.WriteString(scanner.Text())
	}

	return outBuf.String()
}

/* CallerInfo is necessary because the assert functions use the testing object
internally, causing it to print the file:line of the assert method, rather than where
the problem actually occurred in calling code.*/

// CallerInfo returns an array of strings containing the file and line number
// of each stack frame leading from the current test to the assert call that
// failed.
func CallerInfo() []string {
	var pc uintptr
	var ok bool
	var file string
	var line int
	var name string

	callers := []string{}
	for i := 0; ; i++ {
		pc, file, line, ok = runtime.Caller(i)
		if !ok {
			break
		}

		// This is a huge edge case, but it will panic if this is the case, see #180
		if file == "<autogenerated>" {
			break
		}

		f := runtime.FuncForPC(pc)
		if f == nil {
			break
		}
		name = f.Name()

		// testing.tRunner is the standard library function that calls tests
		if name == "testing.tRunner" {
			break
		}

		// 判断是否应该包含此调用
		if shouldIncludeCaller(file, name) {
			callers = append(callers, fmt.Sprintf("%s:%d", file, line))
		}

		// Drop the package
		segments := strings.Split(name, ".")
		name = segments[len(segments)-1]
		if isTest(name, "Test") ||
			isTest(name, "Benchmark") ||
			isTest(name, "Example") {
			break
		}
	}

	return callers
}

func shouldIncludeCaller(file, funcName string) bool {
	if strings.Contains(funcName, "/assert.") {
		return strings.HasSuffix(file, "_test.go")
	}

	return true
}

// Stolen from the `go test` tool.
// isTest tells whether name looks like a test (or benchmark, according to prefix).
// It is a Test (say) if there is a character after Test that is not a lower-case letter.
// We don't want TesticularCancer.
func isTest(name, prefix string) bool {
	if !strings.HasPrefix(name, prefix) {
		return false
	}
	if len(name) == len(prefix) { // "Test" is ok
		return true
	}
	r, _ := utf8.DecodeRuneInString(name[len(prefix):])
	return !unicode.IsLower(r)
}

func objectsAreEqual(expected, actual any) bool {
	if expected == nil || actual == nil {
		return expected == actual
	}

	exp, ok := expected.([]byte)
	if !ok {
		return reflect.DeepEqual(expected, actual)
	}

	act, ok := actual.([]byte)
	if !ok {
		return false
	}
	if exp == nil || act == nil {
		return exp == nil && act == nil
	}
	return bytes.Equal(exp, act)
}
