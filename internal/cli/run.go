// Package cli contains the capgo command-line application.
package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"git.seasonalnet.org/SeasonalNet/capgo"
	"git.seasonalnet.org/SeasonalNet/capgo/profiles/capcp"
	"git.seasonalnet.org/SeasonalNet/capgo/profiles/ipaws"
	"git.seasonalnet.org/SeasonalNet/capgo/profiles/nws"
)

const validationSchema = "git.seasonalnet.org/SeasonalNet/capgo/validation/v1"
const decodeSchema = "git.seasonalnet.org/SeasonalNet/capgo/decode/v1"

// Run executes the capgo CLI and returns a process-style exit code. The
// selected mode reads XML to validate/decode or JSON to encode from an
// optional positional path, or from stdin when the path is omitted or is "-".
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("capgo", flag.ContinueOnError)
	flags.SetOutput(stderr)
	mode := flags.String("mode", "validate", "operation: validate XML, decode XML to JSON, or encode JSON as XML")
	profile := flags.String("profile", "cap", "validation profile: cap, capcp, ipaws, or nws")
	compact := flags.Bool("compact", false, "write compact JSON instead of indented JSON")
	var ipawsChannels stringListFlag
	flags.Var(&ipawsChannels, "ipaws-channel", "IPAWS destination channel (repeatable): capexch, public, eas, nwem, nwr, or cmas")
	gubernatorial := flags.Bool("ipaws-gubernatorial", false, "enforce IPAWS gubernatorial EAS must-carry requirements")
	capcpEventsPath := flags.String("capcp-event-codes", "", "file of governed CAP-CP event codes, one per line")
	capcpLocationsPath := flags.String("capcp-location-codes", "", "file of governed CAP-CP location codes, one per line")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() > 1 {
		_, _ = fmt.Fprintln(stderr, "capgo: at most one input path is allowed")
		return 2
	}
	if *mode != "validate" && *mode != "encode" && *mode != "decode" {
		_, _ = fmt.Fprintf(stderr, "capgo: unknown mode %q (want validate, decode, or encode)\n", *mode)
		return 2
	}
	if *mode == "encode" && *compact {
		_, _ = fmt.Fprintln(stderr, "capgo: -compact applies only to JSON output modes")
		return 2
	}

	validators, err := selectValidators(strings.ToLower(*profile), ipawsChannels, *gubernatorial, *capcpEventsPath, *capcpLocationsPath)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "capgo:", err)
		return 2
	}

	input := stdin
	var file *os.File
	if flags.NArg() == 1 && flags.Arg(0) != "-" {
		file, err = os.Open(flags.Arg(0))
		if err != nil {
			_, _ = fmt.Fprintln(stderr, "capgo: open input:", err)
			return 2
		}
		defer func() { _ = file.Close() }()
		input = file
	}

	if *mode == "encode" {
		return encodeJSON(input, stdout, stderr, validators)
	}
	alert, err := cap.Decode(input)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "capgo: decode:", err)
		return 1
	}
	report := cap.ValidateWith(alert, validators...)
	if *mode == "decode" {
		if !report.Valid() {
			for _, issue := range report {
				_, _ = fmt.Fprintln(stderr, issue)
			}
			return 1
		}
		output := decodeDocument{
			Schema: decodeSchema, Profile: strings.ToLower(*profile), Alert: capMessageOutput(alertFromCAP(alert)),
		}
		return writeJSON(stdout, stderr, output, !*compact)
	}
	document := buildDocument(*profile, alert, report)
	if code := writeJSON(stdout, stderr, document, !*compact); code != 0 {
		return code
	}
	if !report.Valid() {
		for _, issue := range report {
			_, _ = fmt.Fprintln(stderr, issue)
		}
		return 1
	}
	return 0
}

func writeJSON(stdout, stderr io.Writer, value any, indent bool) int {
	encoder := json.NewEncoder(stdout)
	if indent {
		encoder.SetIndent("", "  ")
	}
	if err := encoder.Encode(value); err != nil {
		_, _ = fmt.Fprintln(stderr, "capgo: encode output:", err)
		return 1
	}
	return 0
}

func encodeJSON(input io.Reader, stdout, stderr io.Writer, validators []cap.Validator) int {
	alert, err := decodeEncodeDocument(input)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "capgo: decode JSON:", err)
		return 1
	}
	report := cap.ValidateWith(alert, validators...)
	if !report.Valid() {
		for _, issue := range report {
			_, _ = fmt.Fprintln(stderr, issue)
		}
		return 1
	}
	var encoded bytes.Buffer
	if err := cap.Encode(&encoded, alert); err != nil {
		_, _ = fmt.Fprintln(stderr, "capgo: encode XML:", err)
		return 1
	}
	decoded, err := cap.Decode(bytes.NewReader(encoded.Bytes()))
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "capgo: generated XML failed CAP structure validation:", err)
		return 1
	}
	if report := cap.Validate(decoded); !report.Valid() {
		for _, issue := range report {
			_, _ = fmt.Fprintln(stderr, "capgo: generated XML failed CAP validation:", issue)
		}
		return 1
	}
	if _, err := stdout.Write(encoded.Bytes()); err != nil {
		_, _ = fmt.Fprintln(stderr, "capgo: write XML output:", err)
		return 1
	}
	return 0
}

func selectValidators(profile string, channels []string, gubernatorial bool, eventsPath, locationsPath string) ([]cap.Validator, error) {
	if profile != "ipaws" && (len(channels) != 0 || gubernatorial) {
		return nil, fmt.Errorf("-ipaws-channel and -ipaws-gubernatorial require -profile ipaws")
	}
	if profile != "capcp" && (eventsPath != "" || locationsPath != "") {
		return nil, fmt.Errorf("-capcp-event-codes and -capcp-location-codes require -profile capcp")
	}
	switch profile {
	case "cap":
		return nil, nil
	case "capcp":
		validator := capcp.Validator{}
		var err error
		if eventsPath != "" {
			validator.EventCodes, err = loadCodeList(eventsPath)
			if err != nil {
				return nil, fmt.Errorf("load CAP-CP event codes: %w", err)
			}
		}
		if locationsPath != "" {
			validator.LocationCodes, err = loadCodeList(locationsPath)
			if err != nil {
				return nil, fmt.Errorf("load CAP-CP location codes: %w", err)
			}
		}
		return []cap.Validator{validator}, nil
	case "ipaws":
		selected := make([]ipaws.Channel, 0, len(channels))
		for _, value := range channels {
			channel, ok := map[string]ipaws.Channel{
				"capexch": ipaws.ChannelCAPExchange, "public": ipaws.ChannelPublic,
				"eas": ipaws.ChannelEAS, "nwem": ipaws.ChannelNWEM,
				"nwr": ipaws.ChannelNWR, "cmas": ipaws.ChannelCMAS,
			}[strings.ToLower(value)]
			if !ok {
				return nil, fmt.Errorf("unknown IPAWS channel %q", value)
			}
			selected = append(selected, channel)
		}
		if gubernatorial && !containsChannel(selected, ipaws.ChannelEAS) {
			return nil, fmt.Errorf("-ipaws-gubernatorial requires -ipaws-channel eas")
		}
		return []cap.Validator{ipaws.Validator{Channels: selected, Gubernatorial: gubernatorial}}, nil
	case "nws":
		return []cap.Validator{nws.Validator{}}, nil
	default:
		return nil, fmt.Errorf("unknown profile %q", profile)
	}
}

type stringListFlag []string

func (values *stringListFlag) String() string { return strings.Join(*values, ",") }

func (values *stringListFlag) Set(value string) error {
	*values = append(*values, value)
	return nil
}

func containsChannel(channels []ipaws.Channel, expected ipaws.Channel) bool {
	for _, channel := range channels {
		if channel == expected {
			return true
		}
	}
	return false
}

func loadCodeList(path string) (map[string]struct{}, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	codes := make(map[string]struct{})
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		code := strings.TrimSpace(scanner.Text())
		if code == "" || strings.HasPrefix(code, "#") {
			continue
		}
		codes[code] = struct{}{}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(codes) == 0 {
		return nil, fmt.Errorf("%s contains no codes", path)
	}
	return codes, nil
}

type document struct {
	Schema  string         `json:"schema"`
	Profile string         `json:"profile"`
	Valid   bool           `json:"valid"`
	Alert   alertOutput    `json:"alert"`
	Actions []actionOutput `json:"actions"`
	Issues  []issueOutput  `json:"issues"`
}

type decodeDocument struct {
	Schema  string           `json:"schema"`
	Profile string           `json:"profile"`
	Alert   capMessageOutput `json:"alert"`
}

// capMessageOutput has the same fields as the encode request while keeping
// the decoder's typed output contract distinct from its input DTO.
type capMessageOutput encodeAlert

type alertOutput struct {
	Identifier  string      `json:"identifier"`
	Sender      string      `json:"sender"`
	Sent        string      `json:"sent"`
	Status      cap.Status  `json:"status"`
	MsgType     cap.MsgType `json:"msg_type"`
	Source      string      `json:"source,omitempty"`
	Scope       cap.Scope   `json:"scope"`
	Restriction string      `json:"restriction,omitempty"`
	Addresses   []string    `json:"addresses,omitempty"`
	Codes       []string    `json:"codes,omitempty"`
	Note        string      `json:"note,omitempty"`
	References  string      `json:"references,omitempty"`
	Incidents   string      `json:"incidents,omitempty"`
}

type actionOutput struct {
	Language      string              `json:"language"`
	Event         string              `json:"event"`
	Categories    []cap.Category      `json:"categories"`
	ResponseTypes []cap.ResponseType  `json:"response_types,omitempty"`
	Urgency       cap.Urgency         `json:"urgency"`
	Severity      cap.Severity        `json:"severity"`
	Certainty     cap.Certainty       `json:"certainty"`
	Audience      string              `json:"audience,omitempty"`
	Effective     string              `json:"effective,omitempty"`
	Onset         string              `json:"onset,omitempty"`
	Expires       string              `json:"expires,omitempty"`
	SenderName    string              `json:"sender_name,omitempty"`
	Headline      string              `json:"headline,omitempty"`
	Description   string              `json:"description,omitempty"`
	Instruction   string              `json:"instruction,omitempty"`
	Web           string              `json:"web,omitempty"`
	Contact       string              `json:"contact,omitempty"`
	EventCodes    map[string][]string `json:"event_codes,omitempty"`
	Parameters    map[string][]string `json:"parameters,omitempty"`
	Areas         []areaOutput        `json:"areas,omitempty"`
	Resources     []resourceOutput    `json:"resources,omitempty"`
}

type areaOutput struct {
	Description string              `json:"description"`
	Polygons    []string            `json:"polygons,omitempty"`
	Circles     []string            `json:"circles,omitempty"`
	Geocodes    map[string][]string `json:"geocodes,omitempty"`
	Altitude    *float64            `json:"altitude,omitempty"`
	Ceiling     *float64            `json:"ceiling,omitempty"`
}

type resourceOutput struct {
	Description string `json:"description"`
	MIMEType    string `json:"mime_type"`
	Size        *int64 `json:"size,omitempty"`
	URI         string `json:"uri,omitempty"`
	DerefURI    string `json:"deref_uri,omitempty"`
	Digest      string `json:"digest,omitempty"`
}

type issueOutput struct {
	Level   cap.Level `json:"level"`
	Rule    string    `json:"rule"`
	Path    string    `json:"path"`
	Message string    `json:"message"`
}

func buildDocument(profile string, alert *cap.Alert, report cap.Report) document {
	document := document{
		Schema:  validationSchema,
		Profile: strings.ToLower(profile),
		Valid:   report.Valid(),
		Alert: alertOutput{
			Identifier: alert.Identifier, Sender: alert.Sender, Sent: alert.Sent.String(),
			Status: alert.Status, MsgType: alert.MsgType, Source: alert.Source,
			Scope: alert.Scope, Restriction: alert.Restriction,
			Addresses: cap.ParseAddresses(alert.Addresses), Codes: append([]string(nil), alert.Codes...),
			Note: alert.Note, References: alert.References, Incidents: alert.Incidents,
		},
		Actions: make([]actionOutput, 0, len(alert.Info)),
		Issues:  make([]issueOutput, 0, len(report)),
	}
	for _, issue := range report {
		document.Issues = append(document.Issues, issueOutput{
			Level: issue.Level, Rule: issue.Rule, Path: issue.Path, Message: issue.Message,
		})
	}
	for _, info := range alert.Info {
		document.Actions = append(document.Actions, actionOutput{
			Language: info.EffectiveLanguage(), Event: info.Event,
			Categories:    append([]cap.Category(nil), info.Categories...),
			ResponseTypes: append([]cap.ResponseType(nil), info.ResponseTypes...),
			Urgency:       info.Urgency, Severity: info.Severity, Certainty: info.Certainty,
			Audience: info.Audience, Effective: info.EffectiveTime(alert).String(),
			Onset: dateTimeString(info.Onset), Expires: dateTimeString(info.Expires),
			SenderName: info.SenderName, Headline: info.Headline,
			Description: info.Description, Instruction: info.Instruction,
			Web: info.Web, Contact: info.Contact,
			EventCodes: pairMap(info.EventCodes), Parameters: pairMap(info.Parameters),
			Areas: areas(info.Areas), Resources: resources(info.Resources),
		})
	}
	return document
}

func dateTimeString(value *cap.DateTime) string {
	if value == nil {
		return ""
	}
	return value.String()
}

func pairMap(pairs []cap.ValuePair) map[string][]string {
	if len(pairs) == 0 {
		return nil
	}
	values := make(map[string][]string, len(pairs))
	for _, pair := range pairs {
		values[pair.ValueName] = append(values[pair.ValueName], pair.Value)
	}
	return values
}

func areas(values []cap.Area) []areaOutput {
	result := make([]areaOutput, 0, len(values))
	for _, area := range values {
		result = append(result, areaOutput{
			Description: area.Description, Polygons: append([]string(nil), area.Polygons...),
			Circles: append([]string(nil), area.Circles...), Geocodes: pairMap(area.Geocodes),
			Altitude: area.Altitude, Ceiling: area.Ceiling,
		})
	}
	return result
}

func resources(values []cap.Resource) []resourceOutput {
	result := make([]resourceOutput, 0, len(values))
	for _, resource := range values {
		result = append(result, resourceOutput{
			Description: resource.Description, MIMEType: resource.MIMEType,
			Size: resource.Size, URI: resource.URI, DerefURI: resource.DerefURI,
			Digest: resource.Digest,
		})
	}
	return result
}
