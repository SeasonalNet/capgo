// Command create writes a small valid CAP 1.2 message.
package main

import (
	"log"
	"os"

	"git.seasonalnet.org/SeasonalNet/capgo"
)

func main() {
	sent, err := cap.ParseDateTime("2026-09-20T18:00:00-04:00")
	if err != nil {
		log.Fatal(err)
	}
	expires, err := cap.ParseDateTime("2026-09-20T19:00:00-04:00")
	if err != nil {
		log.Fatal(err)
	}
	alert := &cap.Alert{
		Identifier: "example-1", Sender: "alerts.example.org", Sent: sent,
		Status: cap.StatusActual, MsgType: cap.MsgTypeAlert, Scope: cap.ScopePublic,
		Info: []cap.Info{{
			Language: "en-US", Categories: []cap.Category{cap.CategorySafety},
			Event: "Example Warning", ResponseTypes: []cap.ResponseType{cap.ResponseMonitor},
			Urgency: cap.UrgencyExpected, Severity: cap.SeverityModerate, Certainty: cap.CertaintyLikely,
			Expires: &expires, Description: "An example warning.", Instruction: "Monitor local information.",
			Areas: []cap.Area{{Description: "Example area", Circles: []string{"38.90,-77.04 10"}}},
		}},
	}
	if report := cap.Validate(alert); !report.Valid() {
		log.Fatal(report)
	}
	if err := cap.Encode(os.Stdout, alert); err != nil {
		log.Fatal(err)
	}
}
