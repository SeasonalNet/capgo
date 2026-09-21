package nws_test

import (
	"testing"
	"time"

	"git.seasonalnet.org/SeasonalNet/capgo"
	"git.seasonalnet.org/SeasonalNet/capgo/profiles/ipaws"
	"git.seasonalnet.org/SeasonalNet/capgo/profiles/nws"
)

func TestValidNWSMessage(t *testing.T) {
	alert := validNWS(t)
	report := cap.ValidateWith(alert, nws.Validator{})
	if !report.Valid() {
		t.Fatalf("unexpected errors: %v", report.Errors())
	}
}

func TestNWSIdentifierIsOpaque(t *testing.T) {
	alert := validNWS(t)
	alert.Identifier = "urn:oid:2.49.0.1.840.0.6a5bec09e7660b90d388ed0089000708b4456c34.001.1"
	report := nws.Validate(alert)
	for _, issue := range report {
		if issue.Rule == "NWS-IDENTIFIER" {
			t.Fatalf("opaque NWS identifier was rejected: %v", issue)
		}
	}
}

func TestNWSVTECAllowsPhenomenonSignificance(t *testing.T) {
	alert := validNWS(t)
	alert.Info[0].Parameters = append(alert.Info[0].Parameters, cap.ValuePair{
		ValueName: nws.ParameterVTEC,
		Value:     "/O.CON.KLWX.CF.Y.0043.000000T0000Z-260921T1000Z/",
	})
	report := nws.Validate(alert)
	for _, issue := range report {
		if issue.Rule == "NWS-VTEC" {
			t.Fatalf("valid CF.Y VTEC value was rejected: %v", issue)
		}
	}
}

func TestNWSAcceptsCurrentSAMEEventCodes(t *testing.T) {
	for _, eventCode := range []string{
		"ADR", "AVW", "AVA", "BZW", "BLU", "CAE", "CDW", "CEM", "CFW", "CFA",
		"DSW", "EQW", "EVI", "EWW", "FRW", "FFW", "FFA", "FFS", "FLW", "FLA",
		"FLS", "HMW", "HWW", "HWA", "HUW", "HUA", "HLS", "LEW", "LAE", "MEP", "NMN",
		"TOE", "NUW", "DMO", "RHW", "SVR", "SVA", "SVS", "SQW", "SPW", "SMW", "SPS",
		"SSA", "SSW", "TOR", "TOA", "TRW", "TRA", "TSW", "TSA", "VOW", "WSW",
		"WSA", "EAN", "NIC", "NPT", "RMT", "RWT", "NWS",
	} {
		alert := validNWS(t)
		alert.Info[0].EventCodes[0].Value = eventCode
		report := nws.Validate(alert)
		assertNoRule(t, report, "NWS-EVENTCODE")
	}
}

func TestSAMEEventCodeCatalogIsSortedAndComplete(t *testing.T) {
	codes := nws.SAMEEventCodes()
	if len(codes) != 59 {
		t.Fatalf("SAME event-code catalog has %d entries, want 59", len(codes))
	}
	for index := 1; index < len(codes); index++ {
		if codes[index-1] >= codes[index] {
			t.Fatalf("SAME event-code catalog is not strictly sorted: %q before %q", codes[index-1], codes[index])
		}
	}
	for _, code := range []string{"EAN", "MEP", "NMN", "SQW", "NWS"} {
		if !nws.IsSAMEEventCode(code) {
			t.Fatalf("SAME event-code catalog is missing %s", code)
		}
	}
}

func TestNWSRejectsUnknownSAMEEventCode(t *testing.T) {
	alert := validNWS(t)
	alert.Info[0].EventCodes[0].Value = "ZZZ"
	assertRule(t, nws.Validate(alert), "NWS-EVENTCODE")
}

func TestNWSEASOriginatorFollowsEventCodeSignificance(t *testing.T) {
	alert := validNWS(t)
	alert.Info[0].Parameters = []cap.ValuePair{
		{ValueName: nws.ParameterBlockChannel, Value: "NWEM"},
		{ValueName: nws.ParameterBlockChannel, Value: "EAS"},
		{ValueName: nws.ParameterVTEC, Value: "/O.CON.KLWX.CF.Y.0043.000000T0000Z-260921T1000Z/"},
	}
	alert.Info[0].EventCodes[1].Value = "CFY"
	report := nws.Validate(alert)
	assertNoRule(t, report, "NWS-EAS-ORG")

	alert.Info[0].EventCodes[1].Value = "FAA"
	report = nws.Validate(alert)
	assertRule(t, report, "NWS-EAS-ORG")
}

func TestNWSRequiredFieldsAndParameters(t *testing.T) {
	alert := validNWS(t)
	alert.Sender = "someone@example.org"
	alert.Info[0].Onset = nil
	alert.Info[0].Instruction = ""
	alert.Info[0].Parameters = []cap.ValuePair{{ValueName: nws.ParameterBlockChannel, Value: "PUBLIC"}}
	report := nws.Validate(alert)
	assertRule(t, report, "NWS-SENDER")
	assertRule(t, report, "NWS-ONSET")
	assertRule(t, report, "NWS-INSTRUCTION")
	assertRule(t, report, "NWS-EAS-ORG")
	assertRule(t, report, "NWS-BLOCKCHANNEL")
}

func TestNWSParameterFormats(t *testing.T) {
	alert := validNWS(t)
	alert.Info[0].Parameters = append(alert.Info[0].Parameters,
		cap.ValuePair{ValueName: nws.ParameterHailSize, Value: "1.7"},
		cap.ValuePair{ValueName: nws.ParameterTornadoDamageThreat, Value: "LOW"},
		cap.ValuePair{ValueName: nws.ParameterEventEndingTime, Value: "2026-09-20T12:00:00Z"},
	)
	report := nws.Validate(alert)
	assertRule(t, report, "NWS-HAIL-SIZE")
	assertRule(t, report, "NWS-TORNADO-DAMAGE")
	assertRule(t, report, "NWS-EVENT-END")
}

func TestNWSRequiresArea(t *testing.T) {
	alert := validNWS(t)
	alert.Info[0].Areas = nil
	assertRule(t, nws.Validate(alert), "NWS-AREA")
}

func TestNWSEventMotionRequiresNonzeroDirectionAndCanonicalSpeed(t *testing.T) {
	alert := validNWS(t)
	alert.Info[0].Parameters = append(alert.Info[0].Parameters,
		cap.ValuePair{ValueName: nws.ParameterEventMotion, Value: "2026-09-20T10:00:00+00:00...storm...000DEG...01KT...10,20"},
	)
	assertRule(t, nws.Validate(alert), "NWS-EVENT-MOTION")

	alert.Info[0].Parameters[len(alert.Info[0].Parameters)-1].Value = "2026-09-20T10:00:00+00:00...storm...001DEG...0KT...10,20"
	assertNoRule(t, nws.Validate(alert), "NWS-EVENT-MOTION")
}

func TestNWSActiveReferences(t *testing.T) {
	for _, msgType := range []cap.MsgType{cap.MsgTypeUpdate, cap.MsgTypeCancel} {
		current := validNWS(t)
		current.MsgType = msgType
		prior := validNWS(t)
		prior.Identifier = "prior-1"
		assertRule(t, nws.ValidateActiveReferences(current, prior), "NWS-REFERENCES")
		current.References = cap.FormatReferences([]cap.Reference{{Sender: prior.Sender, Identifier: prior.Identifier, Sent: prior.Sent}})
		if report := nws.ValidateActiveReferences(current, prior); !report.Valid() {
			t.Fatalf("unexpected %s reference error: %v", msgType, report)
		}
	}
}

func validNWS(t *testing.T) *cap.Alert {
	t.Helper()
	sent := cap.NewDateTime(time.Now())
	onset := sent
	expires := cap.NewDateTime(sent.Add(time.Hour))
	return &cap.Alert{
		Identifier: "NWS-IDP-PROD-12345-67890", Sender: nws.Sender, Sent: sent,
		Status: cap.StatusActual, MsgType: cap.MsgTypeAlert, Scope: cap.ScopePublic,
		Codes: []string{ipaws.ProfileCode},
		Info: []cap.Info{{
			Categories: []cap.Category{cap.CategoryMet}, Event: "Severe Thunderstorm Warning",
			ResponseTypes: []cap.ResponseType{cap.ResponseShelter}, Urgency: cap.UrgencyImmediate,
			Severity: cap.SeveritySevere, Certainty: cap.CertaintyObserved,
			EventCodes: []cap.ValuePair{{ValueName: nws.EventCodeSAME, Value: "SVR"}, {ValueName: nws.EventCodeNWS, Value: "SVW"}},
			Effective:  &sent, Onset: &onset, Expires: &expires,
			SenderName: "NWS Example Office", Headline: "Severe Thunderstorm Warning issued September 20",
			Description: "A severe thunderstorm is occurring.", Instruction: "Take shelter.", Web: "https://www.weather.gov",
			Parameters: []cap.ValuePair{{ValueName: nws.ParameterEASORG, Value: "WXR"}, {ValueName: nws.ParameterBlockChannel, Value: "NWEM"}},
			Areas:      []cap.Area{{Description: "Example County", Polygons: []string{"10,20 10,21 11,21 10,20"}, Geocodes: []cap.ValuePair{{ValueName: "SAME", Value: "012345"}, {ValueName: "UGC", Value: "MDC001"}}}},
		}},
	}
}

func assertRule(t *testing.T, report cap.Report, rule string) {
	t.Helper()
	for _, issue := range report {
		if issue.Rule == rule {
			return
		}
	}
	t.Fatalf("rule %s not found: %v", rule, report)
}

func assertNoRule(t *testing.T, report cap.Report, rule string) {
	t.Helper()
	for _, issue := range report {
		if issue.Rule == rule {
			t.Fatalf("unexpected rule %s: %v", rule, report)
		}
	}
}
