package cap

import (
	"fmt"
	"strings"
	"time"
)

// Reference identifies an earlier CAP message.
type Reference struct {
	Sender     string
	Identifier string
	Sent       DateTime
}

func (r Reference) String() string {
	return r.Sender + "," + r.Identifier + "," + r.Sent.String()
}

// ParseReferences parses CAP's whitespace-separated sender,identifier,sent
// tuples.
func ParseReferences(value string) ([]Reference, error) {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return nil, nil
	}
	references := make([]Reference, 0, len(fields))
	for index, field := range fields {
		parts := strings.Split(field, ",")
		if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
			return nil, fmt.Errorf("reference %d must be sender,identifier,sent", index+1)
		}
		sent, err := ParseDateTime(parts[2])
		if err != nil {
			return nil, fmt.Errorf("reference %d: %w", index+1, err)
		}
		references = append(references, Reference{Sender: parts[0], Identifier: parts[1], Sent: sent})
	}
	return references, nil
}

// FormatReferences formats references for Alert.References.
func FormatReferences(references []Reference) string {
	values := make([]string, len(references))
	for index, reference := range references {
		values[index] = reference.String()
	}
	return strings.Join(values, " ")
}

// ParseAddresses parses CAP's whitespace-delimited recipient addresses.
func ParseAddresses(value string) []string { return strings.Fields(value) }

// ActiveAt reports whether at least one info block has no expiry or expires
// after instant.
func ActiveAt(alert *Alert, instant time.Time) bool {
	if alert == nil {
		return false
	}
	for _, info := range alert.Info {
		if info.Expires == nil || info.Expires.After(instant) {
			return true
		}
	}
	return false
}

// MissingActiveReferences returns active candidate messages not named by the
// current message. Callers are responsible for supplying only related prior
// messages whose status is affected by current.
func MissingActiveReferences(current *Alert, candidates ...*Alert) ([]Reference, error) {
	if current == nil {
		return nil, fmt.Errorf("current alert is nil")
	}
	parsed, err := ParseReferences(current.References)
	if err != nil {
		return nil, err
	}
	referenced := make(map[string]struct{}, len(parsed))
	for _, reference := range parsed {
		referenced[reference.String()] = struct{}{}
	}
	missing := make([]Reference, 0)
	for _, candidate := range candidates {
		if candidate == nil || !ActiveAt(candidate, current.Sent.Time) {
			continue
		}
		reference := Reference{Sender: candidate.Sender, Identifier: candidate.Identifier, Sent: candidate.Sent}
		if _, ok := referenced[reference.String()]; !ok {
			missing = append(missing, reference)
		}
	}
	return missing, nil
}
