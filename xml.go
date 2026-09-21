package cap

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	defaultMaxBytes = 8 << 20
	defaultMaxDepth = 64
)

// DecodeOptions bounds untrusted XML input. Zero values use safe defaults.
// Set AllowInvalidStructure only when building diagnostic tooling that needs
// to inspect well-formed but schema-invalid messages.
type DecodeOptions struct {
	MaxBytes              int64
	MaxDepth              int
	AllowInvalidStructure bool
}

// Decode reads one CAP 1.2 XML message using conservative input bounds.
func Decode(r io.Reader) (*Alert, error) {
	return DecodeWithOptions(r, DecodeOptions{})
}

// DecodeWithOptions reads one CAP 1.2 XML message with caller-selected bounds.
func DecodeWithOptions(r io.Reader, options DecodeOptions) (*Alert, error) {
	maxBytes := options.MaxBytes
	if maxBytes == 0 {
		maxBytes = defaultMaxBytes
	}
	if maxBytes < 1 {
		return nil, errors.New("MaxBytes must be positive")
	}
	maxDepth := options.MaxDepth
	if maxDepth == 0 {
		maxDepth = defaultMaxDepth
	}
	if maxDepth < 1 {
		return nil, errors.New("MaxDepth must be positive")
	}

	data, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read CAP XML: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("CAP XML exceeds %d-byte limit", maxBytes)
	}
	if err := inspectXML(data, maxDepth, !options.AllowInvalidStructure); err != nil {
		return nil, err
	}

	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.Strict = true
	var alert Alert
	if err := decoder.Decode(&alert); err != nil {
		return nil, fmt.Errorf("decode CAP XML: %w", err)
	}
	return &alert, nil
}

// Marshal serializes a CAP message as compact XML.
func Marshal(alert *Alert) ([]byte, error) {
	if alert == nil {
		return nil, errors.New("cannot marshal a nil CAP alert")
	}
	data, err := xml.Marshal(alert)
	if err != nil {
		return nil, fmt.Errorf("marshal CAP XML: %w", err)
	}
	return data, nil
}

// MarshalIndent serializes a CAP message as indented XML.
func MarshalIndent(alert *Alert, prefix, indent string) ([]byte, error) {
	if alert == nil {
		return nil, errors.New("cannot marshal a nil CAP alert")
	}
	data, err := xml.MarshalIndent(alert, prefix, indent)
	if err != nil {
		return nil, fmt.Errorf("marshal CAP XML: %w", err)
	}
	return data, nil
}

// Encode writes an indented XML document, including the XML declaration.
func Encode(w io.Writer, alert *Alert) error {
	data, err := MarshalIndent(alert, "", "  ")
	if err != nil {
		return err
	}
	if _, err := io.WriteString(w, xml.Header); err != nil {
		return fmt.Errorf("write CAP XML declaration: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("write CAP XML: %w", err)
	}
	if _, err := io.WriteString(w, "\n"); err != nil {
		return fmt.Errorf("write CAP XML newline: %w", err)
	}
	return nil
}

type structureFrame struct {
	name   xml.Name
	last   int
	counts map[string]int
	opaque bool
}

type childRule struct {
	rank     int
	repeated bool
}

var structureRules = map[string]map[string]childRule{
	"alert": {
		"identifier": {0, false}, "sender": {1, false}, "sent": {2, false},
		"status": {3, false}, "msgType": {4, false}, "source": {5, false},
		"scope": {6, false}, "restriction": {7, false}, "addresses": {8, false},
		"code": {9, true}, "note": {10, false}, "references": {11, false},
		"incidents": {12, false}, "info": {13, true},
	},
	"info": {
		"language": {0, false}, "category": {1, true}, "event": {2, false},
		"responseType": {3, true}, "urgency": {4, false}, "severity": {5, false},
		"certainty": {6, false}, "audience": {7, false}, "eventCode": {8, true},
		"effective": {9, false}, "onset": {10, false}, "expires": {11, false},
		"senderName": {12, false}, "headline": {13, false}, "description": {14, false},
		"instruction": {15, false}, "web": {16, false}, "contact": {17, false},
		"parameter": {18, true}, "resource": {19, true}, "area": {20, true},
	},
	"eventCode": {"valueName": {0, false}, "value": {1, false}},
	"parameter": {"valueName": {0, false}, "value": {1, false}},
	"geocode":   {"valueName": {0, false}, "value": {1, false}},
	"resource": {
		"resourceDesc": {0, false}, "mimeType": {1, false}, "size": {2, false},
		"uri": {3, false}, "derefUri": {4, false}, "digest": {5, false},
	},
	"area": {
		"areaDesc": {0, false}, "polygon": {1, true}, "circle": {2, true},
		"geocode": {3, true}, "altitude": {4, false}, "ceiling": {5, false},
	},
}

func inspectXML(data []byte, maxDepth int, enforceStructure bool) error {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.Strict = true
	stack := make([]structureFrame, 0, 8)
	foundRoot := false
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("parse CAP XML: %w", err)
		}
		switch current := token.(type) {
		case xml.Directive:
			return errors.New("CAP XML directives and DTDs are not allowed")
		case xml.CharData:
			if len(stack) == 0 && strings.TrimSpace(string(current)) != "" {
				return errors.New("non-whitespace content is not allowed outside the CAP alert element")
			}
		case xml.StartElement:
			if len(stack)+1 > maxDepth {
				return fmt.Errorf("CAP XML exceeds maximum element depth %d", maxDepth)
			}
			if !foundRoot {
				foundRoot = true
				if current.Name.Space != Namespace || current.Name.Local != "alert" {
					return fmt.Errorf("root element must be {%s}alert", Namespace)
				}
				stack = append(stack, structureFrame{name: current.Name, last: -1, counts: map[string]int{}})
				continue
			}
			if len(stack) == 0 {
				return errors.New("CAP XML must contain exactly one root element")
			}
			parent := &stack[len(stack)-1]
			opaque := parent.opaque
			if !opaque && enforceStructure {
				if current.Name.Space != Namespace {
					if parent.name.Space != Namespace || parent.name.Local != "alert" {
						return fmt.Errorf("extension element {%s}%s is only allowed below alert", current.Name.Space, current.Name.Local)
					}
					if parent.last > 14 {
						return errors.New("invalid extension element order")
					}
					parent.last = 14
					opaque = true
				} else if err := inspectChild(parent, current.Name.Local); err != nil {
					return err
				}
			}
			stack = append(stack, structureFrame{name: current.Name, last: -1, counts: map[string]int{}, opaque: opaque})
		case xml.EndElement:
			if len(stack) == 0 {
				return errors.New("unexpected XML end element")
			}
			stack = stack[:len(stack)-1]
		}
	}
	if !foundRoot {
		return errors.New("CAP XML document is empty")
	}
	if len(stack) != 0 {
		return errors.New("CAP XML has unclosed elements")
	}
	return nil
}

func inspectChild(parent *structureFrame, local string) error {
	rules, structured := structureRules[parent.name.Local]
	if !structured {
		return fmt.Errorf("element %s may not contain child element %s", parent.name.Local, local)
	}
	rule, ok := rules[local]
	if !ok {
		return fmt.Errorf("unknown CAP element %s below %s", local, parent.name.Local)
	}
	if rule.rank < parent.last {
		return fmt.Errorf("CAP element %s is out of schema order below %s", local, parent.name.Local)
	}
	parent.counts[local]++
	if !rule.repeated && parent.counts[local] > 1 {
		return fmt.Errorf("CAP element %s occurs more than once below %s", local, parent.name.Local)
	}
	parent.last = rule.rank
	return nil
}
