package ipaws_test

import (
	"testing"
	"time"

	"git.seasonalnet.org/SeasonalNet/capgo"
	"git.seasonalnet.org/SeasonalNet/capgo/profiles/ipaws"
)

func TestValidIPAWSMessage(t *testing.T) {
	alert := validIPAWS(t)
	report := cap.ValidateWith(alert, ipaws.Validator{Channels: []ipaws.Channel{ipaws.ChannelEAS}})
	if !report.Valid() {
		t.Fatalf("unexpected errors: %v", report.Errors())
	}
}

func TestChannelRules(t *testing.T) {
	alert := validIPAWS(t)
	alert.Info[0].Parameters = nil
	alert.Info[0].EventCodes[0].Value = "bad"
	report := ipaws.Validator{Channels: []ipaws.Channel{ipaws.ChannelEAS}}.Validate(alert)
	assertRule(t, report, "IPAWS-EAS-ORG")
	assertRule(t, report, "IPAWS-SAME-EVENT")
}

func TestInfoBlocksMustDescribeSameIncident(t *testing.T) {
	alert := validIPAWS(t)
	other := alert.Info[0]
	other.EventCodes = []cap.ValuePair{{ValueName: "SAME", Value: "TOR"}}
	alert.Info = append(alert.Info, other)
	report := ipaws.Validate(alert)
	assertRule(t, report, "IPAWS-INFO-EVENTCODE")
}

func TestActiveReferenceValidation(t *testing.T) {
	current := validIPAWS(t)
	current.MsgType = cap.MsgTypeUpdate
	prior := validIPAWS(t)
	prior.Identifier = "prior-1"
	report := ipaws.ValidateActiveReferences(current, prior)
	assertRule(t, report, "IPAWS-REFERENCES")
	current.References = cap.FormatReferences([]cap.Reference{{Sender: prior.Sender, Identifier: prior.Identifier, Sent: prior.Sent}})
	if report := ipaws.ValidateActiveReferences(current, prior); !report.Valid() {
		t.Fatalf("unexpected error: %v", report)
	}
	current.MsgType = cap.MsgTypeAck
	current.References = cap.FormatReferences([]cap.Reference{{Sender: prior.Sender, Identifier: prior.Identifier, Sent: prior.Sent}})
	if report := ipaws.ValidateActiveReferences(current, prior); !report.Valid() {
		t.Fatalf("unexpected ACK reference error: %v", report)
	}
	prior.Info[0].Expires = func() *cap.DateTime { value := cap.NewDateTime(prior.Sent.Add(-time.Minute)); return &value }()
	assertRule(t, ipaws.ValidateActiveReferences(current, prior), "IPAWS-REFERENCES")
}

func TestGuideAllowsCancelAndPrivateMessagesWithoutInfo(t *testing.T) {
	alert := validIPAWS(t)
	alert.Info = nil
	alert.MsgType = cap.MsgTypeCancel
	alert.References = "sender.example,ipaws-1," + alert.Sent.String()
	if report := ipaws.Validate(alert); !report.Valid() {
		t.Fatalf("cancel without info rejected: %v", report)
	}
	alert.MsgType = cap.MsgTypeAlert
	alert.Scope = cap.ScopePrivate
	alert.References = ""
	if report := ipaws.Validate(alert); !report.Valid() {
		t.Fatalf("private message without info rejected: %v", report)
	}
}

func TestGuideRejectsRestrictedTokensAndUnknownSAME(t *testing.T) {
	alert := validIPAWS(t)
	alert.Identifier = "bad/identifier"
	alert.Sender = "bad\\sender"
	alert.Info[0].EventCodes[0].Value = "ZZZ"
	report := ipaws.Validate(alert)
	assertRule(t, report, "IPAWS-IDENTIFIER")
	assertRule(t, report, "IPAWS-SENDER")
	assertRule(t, report, "IPAWS-SAME-EVENT")
}

func TestGuideEASResourceAndOrganizationValues(t *testing.T) {
	alert := validIPAWS(t)
	alert.Info[0].Resources = []cap.Resource{{Description: "EAS Broadcast Content", MIMEType: "audio/x-ipaws-audio-mp3"}}
	if report := (ipaws.Validator{Channels: []ipaws.Channel{ipaws.ChannelEAS}}).Validate(alert); !report.Valid() {
		t.Fatalf("guide-approved EAS resource rejected: %v", report)
	}
	alert.Info[0].Parameters[0].Value = "BAD"
	assertRule(t, ipaws.Validator{Channels: []ipaws.Channel{ipaws.ChannelEAS}}.Validate(alert), "IPAWS-EAS-ORG")
}

func TestGuideCMASRequirementsAndSpanishCharacters(t *testing.T) {
	alert := validIPAWS(t)
	alert.Info[0].Language = "es-US"
	alert.Info[0].EventCodes[0].Value = "AVW"
	alert.Info[0].ResponseTypes = []cap.ResponseType{cap.ResponseMonitor}
	alert.Info[0].Parameters = []cap.ValuePair{
		{ValueName: "CMAMtext", Value: "Árbol de aviso"},
		{ValueName: "CMAMlongtext", Value: "Árbol de aviso prolongado"},
		{ValueName: "WEAHandling", Value: "Imminent Threat"},
	}
	if report := (ipaws.Validator{Channels: []ipaws.Channel{ipaws.ChannelCMAS}}).Validate(alert); !report.Valid() {
		t.Fatalf("guide-approved CMAS message rejected: %v", report)
	}
	alert.Info[0].Parameters = nil
	assertRule(t, ipaws.Validator{Channels: []ipaws.Channel{ipaws.ChannelCMAS}}.Validate(alert), "IPAWS-CMAMTEXT")
}

func TestGuideNWEMRejectsCirclesAndInvalidBlockChannel(t *testing.T) {
	alert := validIPAWS(t)
	alert.Info[0].Areas[0].Circles = []string{"10,20 5"}
	alert.Info[0].Areas[0].Polygons = nil
	alert.Info[0].Parameters = append(alert.Info[0].Parameters, cap.ValuePair{ValueName: "BLOCKCHANNEL", Value: "PUBLIC"})
	report := ipaws.Validator{Channels: []ipaws.Channel{ipaws.ChannelNWEM}}.Validate(alert)
	assertRule(t, report, "IPAWS-NWEM-CIRCLE")
	assertRule(t, report, "IPAWS-BLOCKCHANNEL")
}

func validIPAWS(t *testing.T) *cap.Alert {
	t.Helper()
	sent := cap.NewDateTime(time.Now())
	expires := cap.NewDateTime(sent.Add(time.Hour))
	return &cap.Alert{
		Identifier: "ipaws-1", Sender: "sender.example", Sent: sent,
		Status: cap.StatusActual, MsgType: cap.MsgTypeAlert, Scope: cap.ScopePublic,
		Codes: []string{ipaws.ProfileCode},
		Info: []cap.Info{{
			Categories: []cap.Category{cap.CategoryMet}, Event: "Severe Thunderstorm Warning",
			Urgency: cap.UrgencyImmediate, Severity: cap.SeveritySevere, Certainty: cap.CertaintyObserved,
			EventCodes: []cap.ValuePair{{ValueName: "SAME", Value: "SVR"}}, Expires: &expires,
			Description: "A severe thunderstorm is occurring.", Instruction: "Take shelter.",
			Parameters: []cap.ValuePair{{ValueName: "EAS-ORG", Value: "WXR"}},
			Areas:      []cap.Area{{Description: "Example County", Polygons: []string{"10,20 10,21 11,21 10,20"}, Geocodes: []cap.ValuePair{{ValueName: "SAME", Value: "012345"}}}},
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
