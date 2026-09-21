package capcp_test

import (
	"testing"

	"git.seasonalnet.org/SeasonalNet/capgo"
	"git.seasonalnet.org/SeasonalNet/capgo/profiles/capcp"
)

func TestValidBilingualMessage(t *testing.T) {
	alert := validCAPCP(t)
	report := cap.ValidateWith(alert, capcp.Validator{})
	if !report.Valid() {
		t.Fatalf("unexpected errors: %v", report.Errors())
	}
}

func TestRejectsDifferentEventsAcrossLanguages(t *testing.T) {
	alert := validCAPCP(t)
	alert.Info[1].EventCodes[0].Value = "tornado"
	report := capcp.Validate(alert)
	assertRule(t, report, "CAPCP-02")
}

func TestManagedListsAndMinorChange(t *testing.T) {
	alert := validCAPCP(t)
	alert.MsgType = cap.MsgTypeUpdate
	alert.References = "sender.example,old-1,2026-09-20T09:00:00+00:00"
	alert.Info[0].Parameters = []cap.ValuePair{{ValueName: capcp.MinorChangeName, Value: "correction"}}
	report := capcp.Validator{
		EventCodes:    map[string]struct{}{"tornado": {}},
		LocationCodes: map[string]struct{}{"3506": {}},
	}.Validate(alert)
	assertRule(t, report, "CAPCP-08-LIST")
	assertRule(t, report, "CAPCP-16")
}

func TestActiveReferenceValidation(t *testing.T) {
	current := validCAPCP(t)
	current.MsgType = cap.MsgTypeUpdate
	prior := validCAPCP(t)
	prior.Identifier = "prior-1"
	current.References = ""
	report := capcp.ValidateActiveReferences(current, prior)
	assertRule(t, report, "CAPCP-12")
	current.References = cap.FormatReferences([]cap.Reference{{Sender: prior.Sender, Identifier: prior.Identifier, Sent: prior.Sent}})
	if report := capcp.ValidateActiveReferences(current, prior); !report.Valid() {
		t.Fatalf("unexpected error: %v", report)
	}
}

func TestPrivateAlertMayOmitInfo(t *testing.T) {
	alert := validCAPCP(t)
	alert.Scope = cap.ScopePrivate
	alert.Addresses = "recipient@example.net"
	alert.Info = nil
	if report := capcp.Validate(alert); !report.Valid() {
		t.Fatalf("private CAP-CP alert without info should be valid: %v", report)
	}
}

func TestAutoTranslatedAppearsAtMostOncePerInfo(t *testing.T) {
	alert := validCAPCP(t)
	alert.Info[0].Parameters = []cap.ValuePair{
		{ValueName: capcp.AutoTranslatedName, Value: "yes"},
		{ValueName: capcp.AutoTranslatedName, Value: "no"},
	}
	assertRule(t, capcp.Validate(alert), "CAPCP-17")
}

func TestSenderMustIdentifyAnAgency(t *testing.T) {
	alert := validCAPCP(t)
	alert.Sender = "12345"
	assertRule(t, capcp.Validate(alert), "CAPCP-11")
}

func TestGeometryRecommendation(t *testing.T) {
	assertRule(t, capcp.Validate(validCAPCP(t)), "CAPCP-18")
}

func validCAPCP(t *testing.T) *cap.Alert {
	t.Helper()
	sent := mustDate(t, "2026-09-20T10:00:00+00:00")
	expires := mustDate(t, "2026-09-20T12:00:00+00:00")
	newInfo := func(language, event, area string) cap.Info {
		return cap.Info{
			Language: language, Categories: []cap.Category{cap.CategoryMet}, Event: event,
			ResponseTypes: []cap.ResponseType{cap.ResponseMonitor}, Urgency: cap.UrgencyExpected,
			Severity: cap.SeveritySevere, Certainty: cap.CertaintyLikely,
			EventCodes: []cap.ValuePair{{ValueName: "profile:CAP-CP:Event:0.3", Value: "thunderstorm"}},
			Expires:    &expires, SenderName: "Environment Canada",
			Areas: []cap.Area{{Description: area, Geocodes: []cap.ValuePair{{ValueName: "profile:CAP-CP:Location:0.3", Value: "3506"}}}},
		}
	}
	return &cap.Alert{
		Identifier: "capcp-1", Sender: "sender.example", Sent: sent,
		Status: cap.StatusActual, MsgType: cap.MsgTypeAlert, Scope: cap.ScopePublic,
		Codes: []string{capcp.ProfileCode},
		Info:  []cap.Info{newInfo("en-CA", "Thunderstorm", "Ottawa"), newInfo("fr-CA", "Orage", "Ottawa")},
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
func assertRule(t *testing.T, report cap.Report, rule string) {
	t.Helper()
	for _, issue := range report {
		if issue.Rule == rule {
			return
		}
	}
	t.Fatalf("rule %s not found: %v", rule, report)
}
