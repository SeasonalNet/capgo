package cap

import "encoding/xml"

// Alert is an OASIS CAP 1.2 alert message. Fields are ordered to match the
// normative XML schema, so encoding/xml produces schema-ordered documents.
type Alert struct {
	XMLName     xml.Name    `xml:"urn:oasis:names:tc:emergency:cap:1.2 alert"`
	Identifier  string      `xml:"identifier"`
	Sender      string      `xml:"sender"`
	Sent        DateTime    `xml:"sent"`
	Status      Status      `xml:"status"`
	MsgType     MsgType     `xml:"msgType"`
	Source      string      `xml:"source,omitempty"`
	Scope       Scope       `xml:"scope"`
	Restriction string      `xml:"restriction,omitempty"`
	Addresses   string      `xml:"addresses,omitempty"`
	Codes       []string    `xml:"code,omitempty"`
	Note        string      `xml:"note,omitempty"`
	References  string      `xml:"references,omitempty"`
	Incidents   string      `xml:"incidents,omitempty"`
	Info        []Info      `xml:"info,omitempty"`
	Extensions  []Extension `xml:",any"`
}

// Info describes one language/audience-specific information block.
type Info struct {
	Language      string         `xml:"language,omitempty"`
	Categories    []Category     `xml:"category"`
	Event         string         `xml:"event"`
	ResponseTypes []ResponseType `xml:"responseType,omitempty"`
	Urgency       Urgency        `xml:"urgency"`
	Severity      Severity       `xml:"severity"`
	Certainty     Certainty      `xml:"certainty"`
	Audience      string         `xml:"audience,omitempty"`
	EventCodes    []ValuePair    `xml:"eventCode,omitempty"`
	Effective     *DateTime      `xml:"effective,omitempty"`
	Onset         *DateTime      `xml:"onset,omitempty"`
	Expires       *DateTime      `xml:"expires,omitempty"`
	SenderName    string         `xml:"senderName,omitempty"`
	Headline      string         `xml:"headline,omitempty"`
	Description   string         `xml:"description,omitempty"`
	Instruction   string         `xml:"instruction,omitempty"`
	Web           string         `xml:"web,omitempty"`
	Contact       string         `xml:"contact,omitempty"`
	Parameters    []ValuePair    `xml:"parameter,omitempty"`
	Resources     []Resource     `xml:"resource,omitempty"`
	Areas         []Area         `xml:"area,omitempty"`
}

// EffectiveLanguage returns Language, or CAP's en-US default when Language is
// absent.
func (i Info) EffectiveLanguage() string {
	if i.Language == "" {
		return "en-US"
	}
	return i.Language
}

// EffectiveTime returns Effective, or the enclosing alert's Sent time when
// Effective is absent as specified by CAP 1.2.
func (i Info) EffectiveTime(alert *Alert) DateTime {
	if i.Effective != nil {
		return *i.Effective
	}
	if alert == nil {
		return DateTime{}
	}
	return alert.Sent
}

// ValuePair represents CAP eventCode, parameter, and geocode pairs.
type ValuePair struct {
	ValueName string `xml:"valueName"`
	Value     string `xml:"value"`
}

// Resource identifies supplemental content associated with an Info block.
type Resource struct {
	Description string `xml:"resourceDesc"`
	MIMEType    string `xml:"mimeType"`
	Size        *int64 `xml:"size,omitempty"`
	URI         string `xml:"uri,omitempty"`
	DerefURI    string `xml:"derefUri,omitempty"`
	Digest      string `xml:"digest,omitempty"`
}

// Area identifies the geographic target of an Info block.
type Area struct {
	Description string      `xml:"areaDesc"`
	Polygons    []string    `xml:"polygon,omitempty"`
	Circles     []string    `xml:"circle,omitempty"`
	Geocodes    []ValuePair `xml:"geocode,omitempty"`
	Altitude    *float64    `xml:"altitude,omitempty"`
	Ceiling     *float64    `xml:"ceiling,omitempty"`
}

// Extension preserves an XML child after the final info element. CAP 1.2
// permits XML Digital Signature elements at this location. Unknown extensions
// are retained during decoding but reported by validation.
type Extension struct {
	XMLName  xml.Name
	Attrs    []xml.Attr `xml:",any,attr"`
	InnerXML string     `xml:",innerxml"`
}

type Status string

const (
	StatusActual   Status = "Actual"
	StatusExercise Status = "Exercise"
	StatusSystem   Status = "System"
	StatusTest     Status = "Test"
	StatusDraft    Status = "Draft"
)

type MsgType string

const (
	MsgTypeAlert  MsgType = "Alert"
	MsgTypeUpdate MsgType = "Update"
	MsgTypeCancel MsgType = "Cancel"
	MsgTypeAck    MsgType = "Ack"
	MsgTypeError  MsgType = "Error"
)

type Scope string

const (
	ScopePublic     Scope = "Public"
	ScopeRestricted Scope = "Restricted"
	ScopePrivate    Scope = "Private"
)

type Category string

const (
	CategoryGeo       Category = "Geo"
	CategoryMet       Category = "Met"
	CategorySafety    Category = "Safety"
	CategorySecurity  Category = "Security"
	CategoryRescue    Category = "Rescue"
	CategoryFire      Category = "Fire"
	CategoryHealth    Category = "Health"
	CategoryEnv       Category = "Env"
	CategoryTransport Category = "Transport"
	CategoryInfra     Category = "Infra"
	CategoryCBRNE     Category = "CBRNE"
	CategoryOther     Category = "Other"
)

type ResponseType string

const (
	ResponseShelter  ResponseType = "Shelter"
	ResponseEvacuate ResponseType = "Evacuate"
	ResponsePrepare  ResponseType = "Prepare"
	ResponseExecute  ResponseType = "Execute"
	ResponseAvoid    ResponseType = "Avoid"
	ResponseMonitor  ResponseType = "Monitor"
	ResponseAssess   ResponseType = "Assess"
	ResponseAllClear ResponseType = "AllClear"
	ResponseNone     ResponseType = "None"
)

type Urgency string

const (
	UrgencyImmediate Urgency = "Immediate"
	UrgencyExpected  Urgency = "Expected"
	UrgencyFuture    Urgency = "Future"
	UrgencyPast      Urgency = "Past"
	UrgencyUnknown   Urgency = "Unknown"
)

type Severity string

const (
	SeverityExtreme  Severity = "Extreme"
	SeveritySevere   Severity = "Severe"
	SeverityModerate Severity = "Moderate"
	SeverityMinor    Severity = "Minor"
	SeverityUnknown  Severity = "Unknown"
)

type Certainty string

const (
	CertaintyObserved Certainty = "Observed"
	CertaintyLikely   Certainty = "Likely"
	CertaintyPossible Certainty = "Possible"
	CertaintyUnlikely Certainty = "Unlikely"
	CertaintyUnknown  Certainty = "Unknown"
)

// Values returns all values associated with name, preserving message order.
func Values(pairs []ValuePair, name string) []string {
	values := make([]string, 0, 1)
	for _, pair := range pairs {
		if pair.ValueName == name {
			values = append(values, pair.Value)
		}
	}
	return values
}

// Value returns the first value associated with name.
func Value(pairs []ValuePair, name string) (string, bool) {
	for _, pair := range pairs {
		if pair.ValueName == name {
			return pair.Value, true
		}
	}
	return "", false
}
