package database

import (
	"github.com/segmentio/encoding/json"
	"reflect"
	"strconv"
	"time"
)

const datetimeFormat = time.RFC3339 // ISO 8601

var _ json.Marshaler = (*Datetime)(nil)
var _ json.Unmarshaler = (*Datetime)(nil)

type Datetime time.Time

var datetimeUnmarshalError = &json.UnmarshalTypeError{
	Value: "datetime",
	Type:  reflect.TypeOf(Datetime{}),
}

func (dt Datetime) Time() time.Time {
	return time.Time(dt)
}

func (dt Datetime) MarshalJSON() ([]byte, error) {
	t := time.Time(dt)
	if t.IsZero() {
		return nil, nil
	}

	s := strconv.Quote(t.UTC().Format(datetimeFormat))
	return []byte(s), nil
}

func (dt *Datetime) UnmarshalJSON(b []byte) error {
	if l := len(b); l < 2 {
		// Too short to be a quoted string
		return datetimeUnmarshalError
	} else if b[0] != '"' || b[l-1] != b[0] {
		// No surrounding quotes
		return datetimeUnmarshalError
	} else {
		// Strip quotes
		b = b[1 : l-1]
	}

	// Handle timezone-offsets in relation to UTC, not local time
	t, err := time.ParseInLocation(datetimeFormat, string(b), time.UTC)
	if err != nil {
		return datetimeUnmarshalError
	}

	*dt = Datetime(t)
	return nil
}
