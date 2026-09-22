package cli

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"

	"git.seasonalnet.org/SeasonalNet/capgo"
)

const (
	encodeSchema = "git.seasonalnet.org/SeasonalNet/capgo/encode/v1"
	maxJSONBytes = 8 << 20
)

// encodeDocument is the versioned CLI input contract. It deliberately does
// not decode directly into cap.Alert, so the authoring schema can evolve
// independently from the Go and XML models.
type encodeDocument struct {
	Schema string       `json:"schema"`
	Alert  *encodeAlert `json:"alert"`
}

type encodeAlert struct {
	Identifier  string            `json:"identifier"`
	Sender      string            `json:"sender"`
	Sent        string            `json:"sent"`
	Status      string            `json:"status"`
	MsgType     string            `json:"msg_type"`
	Source      string            `json:"source,omitempty"`
	Scope       string            `json:"scope"`
	Restriction string            `json:"restriction,omitempty"`
	Addresses   string            `json:"addresses,omitempty"`
	Codes       []string          `json:"codes,omitempty"`
	Note        string            `json:"note,omitempty"`
	References  string            `json:"references,omitempty"`
	Incidents   string            `json:"incidents,omitempty"`
	Info        []encodeInfo      `json:"info,omitempty"`
	Extensions  []encodeExtension `json:"extensions,omitempty"`
}

type encodeInfo struct {
	Language      string            `json:"language,omitempty"`
	Categories    []string          `json:"categories"`
	Event         string            `json:"event"`
	ResponseTypes []string          `json:"response_types,omitempty"`
	Urgency       string            `json:"urgency"`
	Severity      string            `json:"severity"`
	Certainty     string            `json:"certainty"`
	Audience      string            `json:"audience,omitempty"`
	EventCodes    []encodeValuePair `json:"event_codes,omitempty"`
	Effective     *string           `json:"effective,omitempty"`
	Onset         *string           `json:"onset,omitempty"`
	Expires       *string           `json:"expires,omitempty"`
	SenderName    string            `json:"sender_name,omitempty"`
	Headline      string            `json:"headline,omitempty"`
	Description   string            `json:"description,omitempty"`
	Instruction   string            `json:"instruction,omitempty"`
	Web           string            `json:"web,omitempty"`
	Contact       string            `json:"contact,omitempty"`
	Parameters    []encodeValuePair `json:"parameters,omitempty"`
	Resources     []encodeResource  `json:"resources,omitempty"`
	Areas         []encodeArea      `json:"areas,omitempty"`
}

type encodeValuePair struct {
	ValueName string `json:"value_name"`
	Value     string `json:"value"`
}

type encodeResource struct {
	Description string `json:"description"`
	MIMEType    string `json:"mime_type"`
	Size        *int64 `json:"size,omitempty"`
	URI         string `json:"uri,omitempty"`
	DerefURI    string `json:"deref_uri,omitempty"`
	Digest      string `json:"digest,omitempty"`
}

type encodeArea struct {
	Description string            `json:"description"`
	Polygons    []string          `json:"polygons,omitempty"`
	Circles     []string          `json:"circles,omitempty"`
	Geocodes    []encodeValuePair `json:"geocodes,omitempty"`
	Altitude    *float64          `json:"altitude,omitempty"`
	Ceiling     *float64          `json:"ceiling,omitempty"`
}

type encodeExtension struct {
	Namespace  string            `json:"namespace"`
	Name       string            `json:"name"`
	Attributes []encodeAttribute `json:"attributes,omitempty"`
	InnerXML   string            `json:"inner_xml,omitempty"`
}

type encodeAttribute struct {
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name"`
	Value     string `json:"value"`
}

func decodeEncodeDocument(input io.Reader) (*cap.Alert, error) {
	data, err := io.ReadAll(io.LimitReader(input, maxJSONBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read JSON input: %w", err)
	}
	if len(data) > maxJSONBytes {
		return nil, fmt.Errorf("JSON input exceeds %d-byte limit", maxJSONBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var document encodeDocument
	if err := decoder.Decode(&document); err != nil {
		return nil, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			err = errors.New("multiple JSON values are not allowed")
		}
		return nil, err
	}
	if document.Schema != encodeSchema {
		return nil, fmt.Errorf("schema must be %q", encodeSchema)
	}
	if document.Alert == nil {
		return nil, errors.New("alert is required")
	}
	return document.Alert.toCAP()
}

func alertFromCAP(alert *cap.Alert) encodeAlert {
	result := encodeAlert{
		Identifier: alert.Identifier, Sender: alert.Sender, Sent: alert.Sent.String(),
		Status: string(alert.Status), MsgType: string(alert.MsgType), Source: alert.Source,
		Scope: string(alert.Scope), Restriction: alert.Restriction, Addresses: alert.Addresses,
		Codes: append([]string(nil), alert.Codes...), Note: alert.Note,
		References: alert.References, Incidents: alert.Incidents,
	}
	for _, info := range alert.Info {
		converted := encodeInfo{
			Language: info.Language, Categories: make([]string, 0, len(info.Categories)),
			Event: info.Event, ResponseTypes: make([]string, 0, len(info.ResponseTypes)),
			Urgency: string(info.Urgency), Severity: string(info.Severity), Certainty: string(info.Certainty),
			Audience: info.Audience, Effective: dateTimePointerString(info.Effective),
			Onset: dateTimePointerString(info.Onset), Expires: dateTimePointerString(info.Expires),
			SenderName: info.SenderName, Headline: info.Headline, Description: info.Description,
			Instruction: info.Instruction, Web: info.Web, Contact: info.Contact,
			EventCodes: encodePairs(info.EventCodes), Parameters: encodePairs(info.Parameters),
			Resources: make([]encodeResource, 0, len(info.Resources)),
			Areas:     make([]encodeArea, 0, len(info.Areas)),
		}
		for _, category := range info.Categories {
			converted.Categories = append(converted.Categories, string(category))
		}
		for _, response := range info.ResponseTypes {
			converted.ResponseTypes = append(converted.ResponseTypes, string(response))
		}
		for _, resource := range info.Resources {
			converted.Resources = append(converted.Resources, encodeResource{
				Description: resource.Description, MIMEType: resource.MIMEType, Size: resource.Size,
				URI: resource.URI, DerefURI: resource.DerefURI, Digest: resource.Digest,
			})
		}
		for _, area := range info.Areas {
			converted.Areas = append(converted.Areas, encodeArea{
				Description: area.Description, Polygons: append([]string(nil), area.Polygons...),
				Circles: append([]string(nil), area.Circles...), Geocodes: encodePairs(area.Geocodes),
				Altitude: area.Altitude, Ceiling: area.Ceiling,
			})
		}
		result.Info = append(result.Info, converted)
	}
	for _, extension := range alert.Extensions {
		converted := encodeExtension{
			Namespace: extension.XMLName.Space, Name: extension.XMLName.Local,
			InnerXML: extension.InnerXML, Attributes: make([]encodeAttribute, 0, len(extension.Attrs)),
		}
		for _, attribute := range extension.Attrs {
			converted.Attributes = append(converted.Attributes, encodeAttribute{
				Namespace: attribute.Name.Space, Name: attribute.Name.Local, Value: attribute.Value,
			})
		}
		result.Extensions = append(result.Extensions, converted)
	}
	return result
}

func encodePairs(pairs []cap.ValuePair) []encodeValuePair {
	result := make([]encodeValuePair, 0, len(pairs))
	for _, pair := range pairs {
		result = append(result, encodeValuePair{ValueName: pair.ValueName, Value: pair.Value})
	}
	return result
}

func dateTimePointerString(value *cap.DateTime) *string {
	if value == nil {
		return nil
	}
	result := value.String()
	return &result
}

func (input *encodeAlert) toCAP() (*cap.Alert, error) {
	sent, err := cap.ParseDateTime(input.Sent)
	if err != nil {
		return nil, fmt.Errorf("alert.sent: %w", err)
	}
	alert := &cap.Alert{
		Identifier: input.Identifier, Sender: input.Sender, Sent: sent,
		Status: cap.Status(input.Status), MsgType: cap.MsgType(input.MsgType), Source: input.Source,
		Scope: cap.Scope(input.Scope), Restriction: input.Restriction, Addresses: input.Addresses,
		Codes: input.Codes, Note: input.Note, References: input.References, Incidents: input.Incidents,
	}
	for index, info := range input.Info {
		converted := cap.Info{
			Language: info.Language, Event: info.Event, Urgency: cap.Urgency(info.Urgency),
			Severity: cap.Severity(info.Severity), Certainty: cap.Certainty(info.Certainty),
			Audience: info.Audience, SenderName: info.SenderName, Headline: info.Headline,
			Description: info.Description, Instruction: info.Instruction, Web: info.Web, Contact: info.Contact,
		}
		for _, value := range info.Categories {
			converted.Categories = append(converted.Categories, cap.Category(value))
		}
		for _, value := range info.ResponseTypes {
			converted.ResponseTypes = append(converted.ResponseTypes, cap.ResponseType(value))
		}
		var conversionErr error
		converted.Effective, conversionErr = parseOptionalDateTime(info.Effective)
		if conversionErr != nil {
			return nil, fmt.Errorf("alert.info[%d].effective: %w", index, conversionErr)
		}
		converted.Onset, conversionErr = parseOptionalDateTime(info.Onset)
		if conversionErr != nil {
			return nil, fmt.Errorf("alert.info[%d].onset: %w", index, conversionErr)
		}
		converted.Expires, conversionErr = parseOptionalDateTime(info.Expires)
		if conversionErr != nil {
			return nil, fmt.Errorf("alert.info[%d].expires: %w", index, conversionErr)
		}
		converted.EventCodes = convertPairs(info.EventCodes)
		converted.Parameters = convertPairs(info.Parameters)
		for _, resource := range info.Resources {
			converted.Resources = append(converted.Resources, cap.Resource{
				Description: resource.Description, MIMEType: resource.MIMEType, Size: resource.Size,
				URI: resource.URI, DerefURI: resource.DerefURI, Digest: resource.Digest,
			})
		}
		for _, area := range info.Areas {
			converted.Areas = append(converted.Areas, cap.Area{
				Description: area.Description, Polygons: area.Polygons, Circles: area.Circles,
				Geocodes: convertPairs(area.Geocodes), Altitude: area.Altitude, Ceiling: area.Ceiling,
			})
		}
		alert.Info = append(alert.Info, converted)
	}
	for index, extension := range input.Extensions {
		if err := validateExtensionXML(extension.InnerXML); err != nil {
			return nil, fmt.Errorf("alert.extensions[%d].inner_xml: %w", index, err)
		}
		converted := cap.Extension{XMLName: xml.Name{Space: extension.Namespace, Local: extension.Name}, InnerXML: extension.InnerXML}
		for _, attribute := range extension.Attributes {
			converted.Attrs = append(converted.Attrs, xml.Attr{
				Name: xml.Name{Space: attribute.Namespace, Local: attribute.Name}, Value: attribute.Value,
			})
		}
		alert.Extensions = append(alert.Extensions, converted)
	}
	alert.XMLName = xml.Name{Space: cap.Namespace, Local: "alert"}
	return alert, nil
}

func parseOptionalDateTime(value *string) (*cap.DateTime, error) {
	if value == nil {
		return nil, nil
	}
	parsed, err := cap.ParseDateTime(*value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func convertPairs(pairs []encodeValuePair) []cap.ValuePair {
	result := make([]cap.ValuePair, 0, len(pairs))
	for _, pair := range pairs {
		result = append(result, cap.ValuePair{ValueName: pair.ValueName, Value: pair.Value})
	}
	return result
}

func validateExtensionXML(content string) error {
	decoder := xml.NewDecoder(strings.NewReader("<fragment>" + content + "</fragment>"))
	decoder.Strict = true
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("must contain well-formed XML: %w", err)
		}
		if _, ok := token.(xml.Directive); ok {
			return errors.New("XML directives are not allowed")
		}
	}
}
