package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"git.seasonalnet.org/SeasonalNet/capgo"
	"git.seasonalnet.org/SeasonalNet/capgo/profiles/capcp"
	"git.seasonalnet.org/SeasonalNet/capgo/profiles/ipaws"
	"git.seasonalnet.org/SeasonalNet/capgo/profiles/nws"
)

func TestRunReadsStdinAndEmitsActionableJSON(t *testing.T) {
	input := fixture(t)
	var stdout, stderr bytes.Buffer
	if code := Run(nil, bytes.NewReader(input), &stdout, &stderr); code != 0 {
		t.Fatalf("Run exit code = %d, stderr = %s", code, stderr.String())
	}
	var result document
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode JSON output: %v\n%s", err, stdout.String())
	}
	if result.Schema != validationSchema || result.Profile != "cap" || !result.Valid {
		t.Fatalf("unexpected result metadata: %+v", result)
	}
	if result.Alert.Identifier == "" || len(result.Actions) != 1 {
		t.Fatalf("missing actionable CAP data: %+v", result)
	}
	if result.Actions[0].Event == "" || result.Actions[0].Language == "" {
		t.Fatalf("missing action fields: %+v", result.Actions[0])
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestRunSupportsEveryProfile(t *testing.T) {
	input := fixture(t)
	for _, profile := range []string{"cap", "capcp", "ipaws", "nws"} {
		t.Run(profile, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run([]string{"-profile", profile, "-compact"}, bytes.NewReader(input), &stdout, &stderr)
			var result document
			if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
				t.Fatalf("decode JSON output: %v\n%s", err, stdout.String())
			}
			if result.Profile != profile || result.Schema != validationSchema {
				t.Fatalf("unexpected profile metadata: %+v", result)
			}
			if profile == "cap" && code != 0 {
				t.Fatalf("base CAP should validate: code=%d stderr=%s", code, stderr.String())
			}
			if profile != "cap" && code == 0 {
				t.Fatalf("fixture should expose profile findings for %s", profile)
			}
		})
	}
}

func TestRunRawDecodesValidCAPToTypedJSON(t *testing.T) {
	input := fixture(t)
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"-mode", "decode"}, bytes.NewReader(input), &stdout, &stderr); code != 0 {
		t.Fatalf("Run exit code = %d, stderr = %s", code, stderr.String())
	}
	var result decodeDocument
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode JSON output: %v\n%s", err, stdout.String())
	}
	if result.Schema != decodeSchema || result.Profile != "cap" {
		t.Fatalf("unexpected decode metadata: %+v", result)
	}
	if result.Alert.Identifier == "" || len(result.Alert.Info) != 1 || len(result.Alert.Info[0].Categories) == 0 {
		t.Fatalf("raw typed message is incomplete: %+v", result.Alert)
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestRunRawDecodeRequiresSelectedProfileValidity(t *testing.T) {
	input := validEncodeDocument("capcp")
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	var xmlOutput, stderr bytes.Buffer
	if code := Run([]string{"-mode", "encode", "-profile", "cap"}, bytes.NewReader(data), &xmlOutput, &stderr); code != 0 {
		t.Fatalf("prepare CAP-CP XML: code=%d stderr=%s", code, stderr.String())
	}
	var jsonOutput bytes.Buffer
	stderr.Reset()
	if code := Run([]string{"-mode", "decode", "-profile", "capcp"}, bytes.NewReader(xmlOutput.Bytes()), &jsonOutput, &stderr); code != 0 {
		t.Fatalf("valid CAP-CP decode: code=%d stderr=%s", code, stderr.String())
	}
	var result decodeDocument
	if err := json.Unmarshal(jsonOutput.Bytes(), &result); err != nil {
		t.Fatalf("decode CAP-CP JSON output: %v", err)
	}
	if result.Profile != "capcp" {
		t.Fatalf("decode profile = %q, want capcp", result.Profile)
	}

	jsonOutput.Reset()
	stderr.Reset()
	if code := Run([]string{"-mode", "decode", "-profile", "nws"}, bytes.NewReader(xmlOutput.Bytes()), &jsonOutput, &stderr); code != 1 {
		t.Fatalf("invalid NWS profile exit code = %d, want 1", code)
	}
	if jsonOutput.Len() != 0 || stderr.Len() == 0 {
		t.Fatalf("profile-invalid message emitted JSON or no diagnostics: stdout=%s stderr=%s", jsonOutput.String(), stderr.String())
	}
}

func TestRunAcceptsFilePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "alert.xml")
	if err := os.WriteFile(path, fixture(t), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"-profile", "CAP", path}, bytes.NewReader(nil), &stdout, &stderr); code != 0 {
		t.Fatalf("Run exit code = %d, stderr = %s", code, stderr.String())
	}
	var result document
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Profile != "cap" || !result.Valid {
		t.Fatalf("unexpected file result: %+v", result)
	}
}

func TestRunRejectsUnknownProfile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"-profile", "unknown"}, bytes.NewReader(nil), &stdout, &stderr); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
}

func TestRunEncodesJSONFromStdin(t *testing.T) {
	input := `{"schema":"` + encodeSchema + `","alert":{"identifier":"example-1","sender":"alerts.example.org","sent":"2026-09-20T18:00:00-04:00","status":"Actual","msg_type":"Alert","scope":"Public","codes":["example"],"extensions":[{"namespace":"http://www.w3.org/2000/09/xmldsig#","name":"Signature","inner_xml":"<SignedInfo></SignedInfo>"}],"info":[{"language":"en-US","categories":["Safety"],"event":"Example Warning","response_types":["Monitor"],"urgency":"Expected","severity":"Moderate","certainty":"Likely","expires":"2026-09-20T19:00:00-04:00","sender_name":"Example Alerts","description":"An example warning.","parameters":[{"value_name":"key","value":"value"}],"resources":[{"description":"notice","mime_type":"text/plain","size":4,"uri":"https://example.org/notice.txt"}],"areas":[{"description":"Example area","circles":["38.90,-77.04 10"],"geocodes":[{"value_name":"SAME","value":"TOR"}]}]}]}}`
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"-mode", "encode", "-profile", "cap"}, strings.NewReader(input), &stdout, &stderr); code != 0 {
		t.Fatalf("Run exit code = %d, stderr = %s", code, stderr.String())
	}
	alert, err := cap.Decode(bytes.NewReader(stdout.Bytes()))
	if err != nil {
		t.Fatalf("decode generated XML: %v\n%s", err, stdout.String())
	}
	if alert.Identifier != "example-1" || len(alert.Info) != 1 || alert.Info[0].Parameters[0].Value != "value" || alert.Info[0].Areas[0].Geocodes[0].Value != "TOR" || alert.Info[0].Resources[0].URI != "https://example.org/notice.txt" || len(alert.Extensions) != 1 {
		t.Fatalf("generated CAP message lost input fields: %+v", alert)
	}
	var decodedJSON, decodeStderr bytes.Buffer
	if code := Run([]string{"-mode", "decode"}, bytes.NewReader(stdout.Bytes()), &decodedJSON, &decodeStderr); code != 0 {
		t.Fatalf("raw decode generated XML: code=%d stderr=%s", code, decodeStderr.String())
	}
	var decoded decodeDocument
	if err := json.Unmarshal(decodedJSON.Bytes(), &decoded); err != nil {
		t.Fatalf("decode typed JSON output: %v", err)
	}
	message := decoded.Alert
	if message.Identifier != "example-1" || len(message.Info) != 1 || message.Info[0].Parameters[0].Value != "value" || message.Info[0].Areas[0].Geocodes[0].Value != "TOR" || message.Info[0].Resources[0].URI != "https://example.org/notice.txt" || len(message.Extensions) != 1 || message.Extensions[0].Name != "Signature" {
		t.Fatalf("typed decode lost CAP message fields: %+v", message)
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestRunEncodesUnderEverySupportedProfile(t *testing.T) {
	for _, profile := range []string{"capcp", "ipaws", "nws"} {
		t.Run(profile, func(t *testing.T) {
			input, err := json.Marshal(validEncodeDocument(profile))
			if err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			if code := Run([]string{"-mode", "encode", "-profile", profile}, bytes.NewReader(input), &stdout, &stderr); code != 0 {
				t.Fatalf("Run exit code = %d, stderr = %s", code, stderr.String())
			}
			if _, err := cap.Decode(bytes.NewReader(stdout.Bytes())); err != nil {
				t.Fatalf("decode generated %s XML: %v\n%s", profile, err, stdout.String())
			}
		})
	}
}

func TestRunEncodesIPAWSWithChannelAndGubernatorialRules(t *testing.T) {
	document := validEncodeDocument("ipaws")
	document.Alert.Info[0].Parameters = []encodeValuePair{
		{ValueName: "EAS-ORG", Value: "WXR"},
		{ValueName: "EAS-Must-Carry", Value: "TRUE"},
	}
	input, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	args := []string{"-mode", "encode", "-profile", "ipaws", "-ipaws-channel", "eas", "-ipaws-gubernatorial"}
	if code := Run(args, bytes.NewReader(input), &stdout, &stderr); code != 0 {
		t.Fatalf("Run exit code = %d, stderr = %s", code, stderr.String())
	}
	if _, err := cap.Decode(bytes.NewReader(stdout.Bytes())); err != nil {
		t.Fatalf("decode generated IPAWS EAS XML: %v", err)
	}
}

func TestRunEncodeReadsFileAndValidatesSelectedProfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "alert.json")
	if err := os.WriteFile(path, []byte(`{"schema":"`+encodeSchema+`","alert":{"identifier":"example-1","sender":"alerts.example.org","sent":"2026-09-20T18:00:00-04:00","status":"Actual","msg_type":"Alert","scope":"Public","info":[{"categories":["Safety"],"event":"Example","urgency":"Expected","severity":"Moderate","certainty":"Likely"}]}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"-mode", "encode", "-profile", "capcp", path}, bytes.NewReader(nil), &stdout, &stderr); code != 1 {
		t.Fatalf("exit code = %d, want profile validation failure", code)
	}
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), "CAPCP-03") {
		t.Fatalf("profile-invalid input should not emit XML: stdout=%s stderr=%s", stdout.String(), stderr.String())
	}
}

func TestRunEncodeRejectsUnknownJSONFieldsAndTrailingValues(t *testing.T) {
	for _, input := range []string{
		`{"schema":"` + encodeSchema + `","alert":{"identifier":"x","unknown":true}}`,
		`{"schema":"` + encodeSchema + `","alert":{"identifier":"x"}} {"schema":"` + encodeSchema + `"}`,
		`{"identifier":"x"}`,
		`{"schema":"other","alert":{"identifier":"x"}}`,
	} {
		var stdout, stderr bytes.Buffer
		if code := Run([]string{"-mode", "encode"}, strings.NewReader(input), &stdout, &stderr); code != 1 {
			t.Fatalf("exit code = %d, want 1 for %q", code, input)
		}
		if stdout.Len() != 0 {
			t.Fatalf("invalid JSON emitted output: %s", stdout.String())
		}
	}
}

func TestSelectValidatorsEnablesConfiguredIPAWSChannelRules(t *testing.T) {
	validators, err := selectValidators("ipaws", []string{"eas"}, false, "", "")
	if err != nil {
		t.Fatal(err)
	}
	validator := validators[0].(ipaws.Validator)
	alert := minimumIPAWSAlert()
	var foundEASORG bool
	for _, issue := range validator.Validate(alert) {
		if issue.Rule == "IPAWS-EAS-ORG" && issue.Level == cap.LevelError {
			foundEASORG = true
		}
	}
	if !foundEASORG {
		t.Fatal("EAS channel validation did not require the EAS-ORG parameter")
	}
	if _, err := selectValidators("ipaws", []string{"unknown"}, false, "", ""); err == nil {
		t.Fatal("unknown IPAWS channel was accepted")
	}
	if _, err := selectValidators("ipaws", nil, true, "", ""); err == nil {
		t.Fatal("gubernatorial validation without EAS channel was accepted")
	}
}

func TestSelectValidatorsLoadsGovernedCAPCPLists(t *testing.T) {
	eventsPath := filepath.Join(t.TempDir(), "events.txt")
	locationsPath := filepath.Join(t.TempDir(), "locations.txt")
	if err := os.WriteFile(eventsPath, []byte("# current revision\nthunderstorm\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(locationsPath, []byte("3506\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	validators, err := selectValidators("capcp", nil, false, eventsPath, locationsPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(validators) != 1 {
		t.Fatalf("validator count = %d, want 1", len(validators))
	}
	validator := validators[0].(capcp.Validator)
	if len(validator.EventCodes) != 1 || len(validator.LocationCodes) != 1 {
		t.Fatalf("CAP-CP governed lists were not loaded: %+v", validator)
	}
}

func minimumIPAWSAlert() *cap.Alert {
	sent := cap.NewDateTime(time.Now())
	expires := cap.NewDateTime(sent.Add(time.Hour))
	return &cap.Alert{
		Identifier: "ipaws-1", Sender: "sender.example", Sent: sent,
		Status: cap.StatusActual, MsgType: cap.MsgTypeAlert, Scope: cap.ScopePublic,
		Codes: []string{ipaws.ProfileCode},
		Info: []cap.Info{{
			Categories: []cap.Category{cap.CategorySafety}, Event: "Test Event",
			Urgency: cap.UrgencyExpected, Severity: cap.SeverityModerate, Certainty: cap.CertaintyLikely,
			EventCodes: []cap.ValuePair{{ValueName: "SAME", Value: "SVR"}},
			Expires:    &expires, Description: "Test alert",
			Areas: []cap.Area{{Description: "Test area", Geocodes: []cap.ValuePair{{ValueName: "SAME", Value: "012345"}}}},
		}},
	}
}

func validEncodeDocument(profile string) encodeDocument {
	sent := cap.NewDateTime(time.Now())
	expires := cap.NewDateTime(sent.Add(time.Hour))
	sentValue := sent.String()
	expiresValue := expires.String()
	alert := &encodeAlert{
		Identifier: "example-1", Sender: "alerts.example.org", Sent: sentValue,
		Status: string(cap.StatusActual), MsgType: string(cap.MsgTypeAlert), Scope: string(cap.ScopePublic),
		Info: []encodeInfo{{
			Language: "en-US", Categories: []string{string(cap.CategorySafety)}, Event: "Example Warning",
			ResponseTypes: []string{string(cap.ResponseMonitor)}, Urgency: string(cap.UrgencyExpected),
			Severity: string(cap.SeverityModerate), Certainty: string(cap.CertaintyLikely),
			Effective: &sentValue, Onset: &sentValue, Expires: &expiresValue,
			SenderName: "Example Alerts", Headline: "Example Warning",
			Description: "An example warning.", Instruction: "Monitor local information.",
			Web:   "https://example.org/alerts",
			Areas: []encodeArea{{Description: "Example area", Polygons: []string{"10,20 10,21 11,21 10,20"}}},
		}},
	}
	switch profile {
	case "capcp":
		alert.Codes = []string{capcp.ProfileCode}
		alert.Info[0].Language = "en-CA"
		alert.Info[0].EventCodes = []encodeValuePair{{ValueName: "profile:CAP-CP:Event:0.3", Value: "thunderstorm"}}
		alert.Info[0].Areas[0].Geocodes = []encodeValuePair{{ValueName: "profile:CAP-CP:Location:0.3", Value: "3506"}}
	case "ipaws":
		alert.Codes = []string{ipaws.ProfileCode}
		alert.Info[0].EventCodes = []encodeValuePair{{ValueName: "SAME", Value: "SVR"}}
		alert.Info[0].Areas[0].Geocodes = []encodeValuePair{{ValueName: "SAME", Value: "012345"}}
	case "nws":
		alert.Identifier = "NWS-IDP-PROD-12345-67890"
		alert.Sender = nws.Sender
		alert.Codes = []string{ipaws.ProfileCode}
		alert.Info[0].EventCodes = []encodeValuePair{
			{ValueName: nws.EventCodeSAME, Value: "SVR"},
			{ValueName: nws.EventCodeNWS, Value: "SVW"},
		}
		alert.Info[0].Parameters = []encodeValuePair{
			{ValueName: nws.ParameterEASORG, Value: "WXR"},
			{ValueName: nws.ParameterBlockChannel, Value: "NWEM"},
		}
		alert.Info[0].Areas[0].Geocodes = []encodeValuePair{
			{ValueName: "SAME", Value: "012345"},
			{ValueName: "UGC", Value: "MDC001"},
		}
	}
	return encodeDocument{Schema: encodeSchema, Alert: alert}
}

func fixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "oasis-example.xml"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}
