// Package goenv provides functions to manage environment variables
// from .env files and retrieve typed values from the environment.
package env

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// IsSet returns if the given env key is set.
// remember ENV must be a non-empty. All empty
// values are considered unset.
func IsSet(key string) bool {
	return Get(key, "") != ""
}

// Get a value from the ENV. If it doesn't exist the
// default value will be returned.
func Get(key string, defaultValue string) string {
	if v, ok := os.LookupEnv(key); ok {
		return fastTrim(v)
	}
	return defaultValue
}

// GetRaw gets a value from the ENV without trimming.
// If it doesn't exist the default value will be returned.
func GetRaw(key string, defaultValue string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return defaultValue
}

// String returns the string value represented by the string.
func String(key string, defaultValue string) string {
	return Get(key, defaultValue)
}

// Bool returns the boolean value represented by the string.
func Bool(key string, defaultValue bool) bool {
	parsed, err := ParseBool(key, defaultValue)
	if err != nil {
		return defaultValue
	}
	return parsed
}

// ParseBool returns the boolean value represented by the string.
// If the key is not set (or is set to empty), defaultValue will be returned.
// If the key is set but cannot be parsed, it returns defaultValue and an error.
func ParseBool(key string, defaultValue bool) (bool, error) {
	val := strings.TrimSpace(GetRaw(key, ""))
	if val == "" {
		return defaultValue, nil
	}
	parsed, err := strconv.ParseBool(val)
	if err != nil {
		return defaultValue, fmt.Errorf("failed to parse %s as bool: %w", key, err)
	}
	return parsed, nil
}

// Int returns the integer value represented by the string.
// It supports all integer types (int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64).
func Int[T ~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64](key string, defaultValue T) (T, error) {
	v := Get(key, "")
	if v == "" {
		return defaultValue, nil
	}

	var result T
	var err error

	switch any(result).(type) {
	case int:
		val, e := strconv.Atoi(v)
		result = any(val).(T)
		err = e
	case int8:
		val, e := strconv.ParseInt(v, 10, 8)
		result = any(int8(val)).(T)
		err = e
	case int16:
		val, e := strconv.ParseInt(v, 10, 16)
		result = any(int16(val)).(T)
		err = e
	case int32:
		val, e := strconv.ParseInt(v, 10, 32)
		result = any(int32(val)).(T)
		err = e
	case int64:
		val, e := strconv.ParseInt(v, 10, 64)
		result = any(val).(T)
		err = e
	case uint:
		val, e := strconv.ParseUint(v, 10, 0)
		result = any(uint(val)).(T)
		err = e
	case uint8:
		val, e := strconv.ParseUint(v, 10, 8)
		result = any(uint8(val)).(T)
		err = e
	case uint16:
		val, e := strconv.ParseUint(v, 10, 16)
		result = any(uint16(val)).(T)
		err = e
	case uint32:
		val, e := strconv.ParseUint(v, 10, 32)
		result = any(uint32(val)).(T)
		err = e
	case uint64:
		val, e := strconv.ParseUint(v, 10, 64)
		result = any(val).(T)
		err = e
	default:
		err = fmt.Errorf("unsupported integer type %T", result)
	}
	if err != nil {
		return result, fmt.Errorf("failed to parse %s as integer: %w", key, err)
	}
	return result, nil
}

// Duration returns a parsed time.Duration if found in
// the environment value, returns the default value duration
// otherwise.
func Duration(key string, defaultValue time.Duration) (time.Duration, error) {
	v := Get(key, "")
	if v == "" {
		return defaultValue, nil
	}
	return time.ParseDuration(v)
}

// Load loads environment variables from .env file(s).
// It searches for .env file in the current directory by default.
// If multiple files are provided, they will be loaded in order.
// Returns an error if any file cannot be loaded.
func Load(filenames ...string) (err error) {
	return LoadWithOptions(LoadOptions{Overload: false}, filenames...)
}

// LoadOptions configures Load behavior.
type LoadOptions struct {
	// Overload controls whether values from file overwrite existing environment variables.
	Overload bool
}

// LoadWithOptions loads environment variables from .env file(s) using options.
func LoadWithOptions(opts LoadOptions, filenames ...string) error {
	filenames = filenamesOrDefault(filenames)
	for _, filename := range filenames {
		if err := loadFile(filename, opts.Overload); err != nil {
			return err
		}
	}
	return nil
}

// Overload loads environment variables from .env file(s) and overwrites existing variables.
func Overload(filenames ...string) error {
	return LoadWithOptions(LoadOptions{Overload: true}, filenames...)
}

// Marshal outputs the given environment as a dotenv-formatted environment file.
// Each line is in the format: KEY="VALUE" where VALUE is backslash-escaped.
func Marshal() (string, error) {
	return MarshalWithOptions(MarshalOptions{QuoteAll: true, TrimValues: true})
}

// MarshalOptions controls formatting of Marshal output.
type MarshalOptions struct {
	// QuoteAll forces all values to be emitted as KEY="VALUE".
	// When false, MarshalWithOptions will keep legacy behavior of emitting integers unquoted.
	QuoteAll bool
	// TrimValues applies the same trim logic as Get/String (leading/trailing ' ' only).
	TrimValues bool
}

// MarshalWithOptions outputs the current process environment as a dotenv-formatted environment file.
func MarshalWithOptions(opts MarshalOptions) (string, error) {
	envMap := map[string]string{}
	for _, e := range os.Environ() {
		pair := strings.SplitN(e, "=", 2)
		if len(pair) == 0 {
			continue
		}
		key := pair[0]
		val := ""
		if len(pair) == 2 {
			val = pair[1]
		}
		if opts.TrimValues {
			val = fastTrim(val)
		}
		envMap[key] = val
	}

	lines := make([]string, 0, len(envMap))
	for k, v := range envMap {
		if !opts.QuoteAll {
			if d, err := strconv.Atoi(v); err == nil {
				lines = append(lines, fmt.Sprintf(`%s=%d`, k, d))
				continue
			}
		}
		lines = append(lines, fmt.Sprintf(`%s="%s"`, k, doubleQuoteEscape(v)))
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n"), nil
}

const doubleQuoteSpecialChars = "\\\n\r\"!$`"

func doubleQuoteEscape(line string) string {
	for _, c := range doubleQuoteSpecialChars {
		toReplace := "\\" + string(c)
		if c == '\n' {
			toReplace = `\n`
		}
		if c == '\r' {
			toReplace = `\r`
		}
		line = strings.Replace(line, string(c), toReplace, -1)
	}
	return line
}
func filenamesOrDefault(filenames []string) []string {
	if len(filenames) == 0 {
		return []string{".env"}
	}
	return filenames
}

func loadFile(filename string, overload bool) error {
	envMap, err := readFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filename, err)
	}

	currentEnv := map[string]bool{}
	rawEnv := os.Environ()
	for _, rawEnvLine := range rawEnv {
		key := strings.Split(rawEnvLine, "=")[0]
		currentEnv[key] = true
	}

	for key, value := range envMap {
		if !currentEnv[key] || overload {
			if err := os.Setenv(key, value); err != nil {
				return fmt.Errorf("failed to set env %s: %w", key, err)
			}
		}
	}

	return nil
}

func readFile(filename string) (envMap map[string]string, err error) {
	file, err := os.Open(filename)
	if err != nil {
		return
	}
	defer file.Close()

	var buf bytes.Buffer
	_, err = io.Copy(&buf, file)
	if err != nil {
		return nil, err
	}
	envMap = map[string]string{}
	err = parseBytes(buf.Bytes(), envMap)
	return
}

func fastTrim(s string) string {
	if s == "" {
		return s
	}

	start := 0
	end := len(s)
	for start < end {
		if s[start] != ' ' {
			break
		}
		start++
	}
	for end > start {
		if s[end-1] != ' ' {
			break
		}
		end--
	}
	if start == 0 && end == len(s) {
		return s
	}
	return s[start:end]
}
