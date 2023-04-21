package storeit

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
)

func AnyToString(val any) (str string, err error) {
	if val == nil {
		return
	}

	switch value := val.(type) {
	case int:
		str = strconv.Itoa(value)
	case int8:
		str = strconv.Itoa(int(value))
	case int16:
		str = strconv.Itoa(int(value))
	case int32: // same as `rune`
		str = strconv.Itoa(int(value))
	case int64:
		str = strconv.FormatInt(value, 10)
	case uint:
		str = strconv.FormatUint(uint64(value), 10)
	case uint8:
		str = strconv.FormatUint(uint64(value), 10)
	case uint16:
		str = strconv.FormatUint(uint64(value), 10)
	case uint32:
		str = strconv.FormatUint(uint64(value), 10)
	case uint64:
		str = strconv.FormatUint(value, 10)
	case float32:
		str = strconv.FormatFloat(float64(value), 'f', -1, 32)
	case float64:
		str = strconv.FormatFloat(value, 'f', -1, 64)
	case bool:
		str = strconv.FormatBool(value)
	case string:
		str = value
	case []byte:
		str = string(value)
	case time.Duration:
		str = value.String()
	case json.Number:
		str = value.String()
	default:
		err = errors.New("convert value type error")
	}
	return
}

func AnyToInt(in any) (iVal int, err error) {
	switch tVal := in.(type) {
	case nil:
		iVal = 0
	case int:
		iVal = tVal
	case int8:
		iVal = int(tVal)
	case int16:
		iVal = int(tVal)
	case int32:
		iVal = int(tVal)
	case int64:
		iVal = int(tVal)
	case uint:
		iVal = int(tVal)
	case uint8:
		iVal = int(tVal)
	case uint16:
		iVal = int(tVal)
	case uint32:
		iVal = int(tVal)
	case uint64:
		iVal = int(tVal)
	case float32:
		iVal = int(tVal)
	case float64:
		iVal = int(tVal)
	case time.Duration:
		iVal = int(tVal)
	case string:
		iVal, err = strconv.Atoi(strings.TrimSpace(tVal))
	case json.Number:
		var i64 int64
		i64, err = tVal.Int64()
		iVal = int(i64)
	default:
		err = errors.New("convert value type error")
	}
	return
}
