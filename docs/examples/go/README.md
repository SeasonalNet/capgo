# Go

Go code can use the CAP object model and validators without starting a subprocess.

```go
package main

import (
	"encoding/json"
	"log"
	"os"

	cap "git.seasonalnet.org/SeasonalNet/capgo"
	"git.seasonalnet.org/SeasonalNet/capgo/profiles/nws"
)

type output struct {
	Identifier string      `json:"identifier"`
	Valid      bool        `json:"valid"`
	Event      []string    `json:"events"`
	Issues     []cap.Issue `json:"issues"`
}

func main() {
	alert, err := cap.Decode(os.Stdin)
	if err != nil {
		log.Fatal(err)
	}

	// Replace nws.Validator{} with capcp.Validator{} or ipaws.Validator{}
	// for the profile this application accepts. Use cap.Validate(alert)
	// separately when only the base CAP rules are needed.
	report := cap.ValidateWith(alert, nws.Validator{})
	events := make([]string, 0, len(alert.Info))
	for _, info := range alert.Info {
		events = append(events, info.Event)
	}

	if err := json.NewEncoder(os.Stdout).Encode(output{
		Identifier: alert.Identifier,
		Valid:      report.Valid(),
		Event:      events,
		Issues:     report,
	}); err != nil {
		log.Fatal(err)
	}
	if !report.Valid() {
		os.Exit(1)
	}
}
```

For IPAWS channel rules, select the channel explicitly:

```go
import "git.seasonalnet.org/SeasonalNet/capgo/profiles/ipaws"

report := cap.ValidateWith(alert, ipaws.Validator{
	Channels: []ipaws.Channel{ipaws.ChannelEAS},
})
```

For update/cancel history checks, call `ipaws.ValidateActiveReferences(current, priorMessages...)` in addition to the normal validation.
