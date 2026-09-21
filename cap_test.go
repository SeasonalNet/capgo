package cap_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"git.seasonalnet.org/SeasonalNet/capgo"
)

func TestDecodeValidateAndRoundTrip(t *testing.T) {
	file, err := os.Open("testdata/oasis-example.xml")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Errorf("close fixture: %v", err)
		}
	}()

	alert, err := cap.Decode(file)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if report := cap.Validate(alert); !report.Valid() {
		t.Fatalf("Validate: %v", report)
	}
	encoded, err := cap.MarshalIndent(alert, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	decoded, err := cap.Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("round-trip Decode: %v\n%s", err, encoded)
	}
	if decoded.Identifier != alert.Identifier || len(decoded.Info) != 1 {
		t.Fatalf("round-trip changed message: %#v", decoded)
	}
}

func TestDecodePreservesXMLSignatureExtension(t *testing.T) {
	input := strings.Replace(validMinimalXML(), "</alert>", `<ds:Signature xmlns:ds="http://www.w3.org/2000/09/xmldsig#" Id="sig1"><ds:SignedInfo/></ds:Signature></alert>`, 1)
	alert, err := cap.Decode(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(alert.Extensions) != 1 || alert.Extensions[0].XMLName.Space != cap.XMLDSigNamespace {
		t.Fatalf("signature not preserved: %#v", alert.Extensions)
	}
	encoded, err := cap.Marshal(alert)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !bytes.Contains(encoded, []byte("SignedInfo")) || !bytes.Contains(encoded, []byte(`Id="sig1"`)) {
		t.Fatalf("signature content lost: %s", encoded)
	}
}

func TestDecodeRejectsUnsafeOrNonSchemaStructure(t *testing.T) {
	tests := map[string]string{
		"DTD":              `<!DOCTYPE alert [<!ENTITY x "boom">]>` + validMinimalXML(),
		"wrong namespace":  strings.Replace(validMinimalXML(), cap.Namespace, "urn:wrong", 1),
		"out of order":     strings.Replace(validMinimalXML(), "<status>Actual</status><msgType>Alert</msgType>", "<msgType>Alert</msgType><status>Actual</status>", 1),
		"duplicate":        strings.Replace(validMinimalXML(), "<scope>Public</scope>", "<scope>Public</scope><scope>Public</scope>", 1),
		"nested extension": strings.Replace(validMinimalXML(), "<event>Test</event>", `<event>Test</event><x:foo xmlns:x="urn:x"/>`, 1),
		"multiple roots":   validMinimalXML() + validMinimalXML(),
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := cap.Decode(strings.NewReader(input)); err == nil {
				t.Fatal("Decode unexpectedly accepted invalid input")
			}
		})
	}
}

func TestDecodeBounds(t *testing.T) {
	input := validMinimalXML()
	if _, err := cap.DecodeWithOptions(strings.NewReader(input), cap.DecodeOptions{MaxBytes: 10}); err == nil {
		t.Fatal("expected byte-limit error")
	}
	if _, err := cap.DecodeWithOptions(strings.NewReader(input), cap.DecodeOptions{MaxDepth: 1}); err == nil {
		t.Fatal("expected depth-limit error")
	}
}

func TestDateTimeLexicalForm(t *testing.T) {
	valid := []string{"2026-09-20T18:30:00-04:00", "2026-09-20T22:30:00+00:00", "2026-09-20T22:30:00-00:00", "2026-09-20T12:00:00+14:00"}
	for _, value := range valid {
		parsed, err := cap.ParseDateTime(value)
		if err != nil {
			t.Errorf("ParseDateTime(%q): %v", value, err)
		}
		if got := parsed.String(); got != value {
			t.Errorf("round trip = %q, want %q", got, value)
		}
	}
	invalid := []string{"2026-09-20T22:30:00Z", "2026-09-20T22:30:00", "2026-09-20T22:30:00.123+00:00", "2026-09-20T22:30:00+14:01", "2026-02-30T22:30:00+00:00"}
	for _, value := range invalid {
		if _, err := cap.ParseDateTime(value); err == nil {
			t.Errorf("ParseDateTime(%q) unexpectedly succeeded", value)
		}
	}
}

func TestReferences(t *testing.T) {
	value := "a.example,id-1,2026-09-20T10:00:00+00:00 b.example,id-2,2026-09-20T11:00:00+00:00"
	references, err := cap.ParseReferences(value)
	if err != nil {
		t.Fatal(err)
	}
	if len(references) != 2 || cap.FormatReferences(references) != value {
		t.Fatalf("reference round-trip failed: %#v", references)
	}
	if _, err := cap.ParseReferences("broken"); err == nil {
		t.Fatal("invalid reference accepted")
	}
}

func TestGeometry(t *testing.T) {
	if _, err := cap.ParsePolygon("10,20 10,30 20,30 10,20"); err != nil {
		t.Fatal(err)
	}
	if _, err := cap.ParsePolygon("10,20 10,30 20,30 20,20"); err == nil {
		t.Fatal("open polygon accepted")
	}
	if _, _, err := cap.ParseCircle("10,20 25"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := cap.ParseCircle("91,20 25"); err == nil {
		t.Fatal("invalid latitude accepted")
	}
}

func TestBaseSemanticValidation(t *testing.T) {
	alert := minimalAlert(t)
	alert.Scope = cap.ScopeRestricted
	alert.Info[0].Expires = ptrDate(t, "2026-09-20T09:00:00+00:00")
	report := cap.Validate(alert)
	if report.Valid() {
		t.Fatal("expected validation errors")
	}
	assertRule(t, report, "CAP-RESTRICTION")
	assertRule(t, report, "CAP-EXPIRES")
}

func minimalAlert(t *testing.T) *cap.Alert {
	t.Helper()
	sent := mustDate(t, "2026-09-20T10:00:00+00:00")
	return &cap.Alert{
		Identifier: "example-1", Sender: "sender.example", Sent: sent,
		Status: cap.StatusActual, MsgType: cap.MsgTypeAlert, Scope: cap.ScopePublic,
		Info: []cap.Info{{
			Categories: []cap.Category{cap.CategorySafety}, Event: "Test",
			Urgency: cap.UrgencyExpected, Severity: cap.SeverityMinor, Certainty: cap.CertaintyLikely,
		}},
	}
}

func mustDate(t *testing.T, value string) cap.DateTime {
	t.Helper()
	parsed, err := cap.ParseDateTime(value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
func ptrDate(t *testing.T, value string) *cap.DateTime { parsed := mustDate(t, value); return &parsed }

func assertRule(t *testing.T, report cap.Report, rule string) {
	t.Helper()
	for _, issue := range report {
		if issue.Rule == rule {
			return
		}
	}
	t.Fatalf("rule %s not found in report: %v", rule, report)
}

func validMinimalXML() string {
	return `<alert xmlns="` + cap.Namespace + `"><identifier>example-1</identifier><sender>sender.example</sender><sent>2026-09-20T10:00:00+00:00</sent><status>Actual</status><msgType>Alert</msgType><scope>Public</scope><info><category>Safety</category><event>Test</event><urgency>Expected</urgency><severity>Minor</severity><certainty>Likely</certainty></info></alert>`
}

func FuzzDecodeNeverPanics(f *testing.F) {
	f.Add([]byte(validMinimalXML()))
	f.Add([]byte(`<!DOCTYPE alert><alert/>`))
	f.Add([]byte{0xff, 0xfe, 0xfd})
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = cap.DecodeWithOptions(bytes.NewReader(data), cap.DecodeOptions{MaxBytes: 1 << 20, MaxDepth: 32})
	})
}
