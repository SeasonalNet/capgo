package cap

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

const dateTimeLayout = "2006-01-02T15:04:05-07:00"

var dateTimePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[+-]\d{2}:\d{2}$`)

// DateTime is CAP 1.2's restricted xs:dateTime: whole seconds and an explicit
// numeric UTC offset. The XML form deliberately rejects the "Z" shorthand.
type DateTime struct {
	time.Time
	offset string
}

// NewDateTime constructs a CAP date-time from t.
func NewDateTime(t time.Time) DateTime {
	t = t.Truncate(time.Second)
	return DateTime{Time: t, offset: t.Format("-07:00")}
}

// ParseDateTime parses the exact lexical form required by CAP 1.2.
func ParseDateTime(value string) (DateTime, error) {
	if !dateTimePattern.MatchString(value) {
		return DateTime{}, fmt.Errorf("CAP date-time must match YYYY-MM-DDThh:mm:ss+hh:mm: %q", value)
	}
	offsetHour, _ := strconv.Atoi(value[len(value)-5 : len(value)-3])
	offsetMinute, _ := strconv.Atoi(value[len(value)-2:])
	if offsetHour > 14 || offsetMinute > 59 || (offsetHour == 14 && offsetMinute != 0) {
		return DateTime{}, fmt.Errorf("CAP date-time has invalid UTC offset: %q", value)
	}
	parsed, err := time.Parse(dateTimeLayout, value)
	if err != nil {
		return DateTime{}, fmt.Errorf("invalid CAP date-time %q: %w", value, err)
	}
	return DateTime{Time: parsed, offset: value[len(value)-6:]}, nil
}

func (d DateTime) String() string {
	if d.IsZero() {
		return ""
	}
	if d.offset != "" {
		return d.Format("2006-01-02T15:04:05") + d.offset
	}
	return d.Format(dateTimeLayout)
}

func (d DateTime) MarshalText() ([]byte, error) {
	if d.IsZero() {
		return nil, fmt.Errorf("CAP date-time is zero")
	}
	if d.Nanosecond() != 0 {
		return nil, fmt.Errorf("CAP date-time cannot contain fractional seconds")
	}
	value := d.String()
	parsed, err := ParseDateTime(value)
	if err != nil {
		return nil, err
	}
	_, actualOffset := d.Zone()
	_, parsedOffset := parsed.Zone()
	if actualOffset != parsedOffset {
		return nil, fmt.Errorf("CAP date-time offset does not match its time value")
	}
	return []byte(value), nil
}

func (d *DateTime) UnmarshalText(text []byte) error {
	parsed, err := ParseDateTime(string(text))
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}
