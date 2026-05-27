package gitcfg

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"unicode"
)

// Scalar is the set of types supported by Get and GetDefault.
type Scalar interface {
	~string | ~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64 | ~bool
}

func normalizeKey(key string) (string, error) {
	section, name, err := splitConfigKey(key)
	if err != nil {
		return "", err
	}
	return strings.ToLower(section) + "." + strings.ToLower(name), nil
}

func splitConfigKey(key string) (section, name string, err error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", "", ErrInvalidKeyFormat
	}

	i := strings.LastIndexByte(key, '.')
	if i <= 0 || i == len(key)-1 {
		return "", "", ErrInvalidKeyFormat
	}

	section = strings.TrimSpace(key[:i])
	name = strings.TrimSpace(key[i+1:])
	if !isValidSection(section) || !isValidVariableName(name) {
		return "", "", ErrInvalidKeyFormat
	}

	return section, name, nil
}

func isValidSection(section string) bool {
	if section == "" {
		return false
	}
	for _, r := range section {
		if r == 0 || r == '\n' || r == '\r' || unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func isValidVariableName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			continue
		}
		return false
	}
	return true
}

func parseBool(v string) (bool, error) {
	if v == "" {
		return true, nil
	}

	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true", "yes", "on", "1":
		return true, nil
	case "false", "no", "off", "0":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean value: %s", v)
	}
}

func convertValue[T Scalar](v string) (T, error) {
	var zero T
	typ := reflect.TypeOf(zero)
	if typ == nil {
		return zero, fmt.Errorf("%w: unsupported type", ErrInvalidValue)
	}

	var rv reflect.Value
	var err error

	switch typ.Kind() {
	case reflect.String:
		rv = reflect.ValueOf(v).Convert(typ)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		var n int64
		n, err = strconv.ParseInt(v, 10, typ.Bits())
		if err == nil {
			rv = reflect.ValueOf(n).Convert(typ)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		var n uint64
		n, err = strconv.ParseUint(v, 10, typ.Bits())
		if err == nil {
			rv = reflect.ValueOf(n).Convert(typ)
		}
	case reflect.Float32, reflect.Float64:
		var n float64
		n, err = strconv.ParseFloat(v, typ.Bits())
		if err == nil {
			rv = reflect.ValueOf(n).Convert(typ)
		}
	case reflect.Bool:
		var b bool
		b, err = parseBool(v)
		if err == nil {
			rv = reflect.ValueOf(b).Convert(typ)
		}
	default:
		err = fmt.Errorf("unsupported type")
	}

	if err != nil {
		return zero, fmt.Errorf("%w: %v", ErrInvalidValue, err)
	}

	return rv.Interface().(T), nil
}
