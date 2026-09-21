# capgo

`capgo` is a dependency-free Go library for the XML representation of OASIS Common Alerting Protocol (CAP) version 1.2. It includes composable validators for the Canadian CAP Profile (CAP-CP), the IPAWS CAP profile, and the National Weather Service CAP producer profile.

The primary repository is [SeasonalForge](https://git.seasonalnet.org/SeasonalNet/capgo). The [SeasonalNet GitHub repository](https://github.com/SeasonalNet/capgo) is a mirror.

## What is included

- The complete CAP 1.2 XML object model: `alert`, `info`, `resource`, `area`, event codes, parameters, geocodes, and XML Digital Signature extension preservation.
- Strict XML structure and ordering checks matching the CAP 1.2 XSD.
- Semantic checks from the CAP 1.2 data dictionary, including lifecycle references, scope-dependent fields, date-times, resources, and WGS 84 polygons/circles.
- Bounded, DTD-free decoding for untrusted alert feeds.
- Structured error/warning reports with stable rule IDs and XML-like paths.
- CAP-CP Beta 0.4A validation, optional exact managed-list validation, and active-message reference-chain checking.
- IPAWS 1.0 validation with guide-backed EAS, NWEM/NWR, CMAS/WEA, and gubernatorial must-carry rules.
- NWS CAP v1.2 producer validation and NWS parameter checks.
- No third-party or C dependencies.

## Install

```sh
go get git.seasonalnet.org/SeasonalNet/capgo
```

The module path is ready for that repository location; change `module` in `go.mod` before publishing elsewhere.

## Decode and validate

```go
file, err := os.Open("alert.xml")
if err != nil {
    return err
}
defer file.Close()

alert, err := cap.Decode(file)
if err != nil {
    return err // malformed, unsafe, oversized, or structurally invalid XML
}

report := cap.ValidateWith(alert, nws.Validator{})
for _, issue := range report {
    log.Printf("%s", issue)
}
if !report.Valid() {
    return report
}
```

Imports for that example:

```go
import (
    "os"
    "git.seasonalnet.org/SeasonalNet/capgo"
    "git.seasonalnet.org/SeasonalNet/capgo/profiles/nws"
)
```

`ValidateWith` always runs base CAP validation first. Error-level issues represent violated mandatory rules. Warning-level issues represent `SHOULD`/recommended practices and do not make `Report.Valid` false.

## Select a profile

```go
baseOnly := cap.Validate(alert)
canadian := cap.ValidateWith(alert, capcp.Validator{})
ipawsEAS := cap.ValidateWith(alert, ipaws.Validator{
    Channels: []ipaws.Channel{ipaws.ChannelEAS},
})
nwsProduced := cap.ValidateWith(alert, nws.Validator{})
```

For exact CAP-CP managed-list membership, inject the revision your system governs:

```go
validator := capcp.Validator{
    EventCodes: map[string]struct{}{"tornado": {}},
    LocationCodes: map[string]struct{}{"3506": {}},
}
```

The CAP-CP documents version their rule set, event references, and location references independently. The library therefore validates the rule-set syntax by default and never hardcodes a potentially stale event or SGC list.

CAP-CP rule 12 depends on earlier messages, so it has an explicit history-aware check:

```go
report := capcp.ValidateActiveReferences(current, priorMessages...)
```

## Encode

Construct messages with the exported types and constants, then call `cap.Validate` before publication and `cap.Encode` to write an XML declaration plus indented CAP XML. See `examples/create`.

## Command-line application

```sh
go run ./cmd/capgo -profile cap testdata/oasis-example.xml
cat message.xml | go run ./cmd/capgo -profile capcp
cat message.xml | go run ./cmd/capgo -profile ipaws -compact
go run ./cmd/capgo -profile nws message.xml
```

The command accepts one XML path or reads XML from standard input when the
path is omitted (or `-`). It writes one JSON document to standard output. The
document contains the normalized alert metadata, one actionable object for
each `info` block, and structured validation issues. A message with
error-level findings still produces JSON and exits non-zero; diagnostics are
also written to standard error. The `examples/validate` command remains as a
compatibility wrapper around the same application.

## Standards target

The implementation targets these published documents:

- [OASIS Common Alerting Protocol Version 1.2](https://docs.oasis-open.org/emergency/cap/v1.2/CAP-v1.2-os.html), including its [normative XML schema](https://docs.oasis-open.org/emergency/cap/v1.2/CAP-v1.2.xsd).
- [Canadian Profile of CAP, Introduction and Rule Set Beta 0.4A](https://www.publicsafety.gc.ca/cnt/rsrcs/pblctns/capcp-ntro-rl-st/index3-en.aspx).
- [CAP v1.2 IPAWS Profile Version 1.0](https://docs.oasis-open.org/emergency/cap/v1.2/ipaws-profile/v1.0/cap-v1.2-ipaws-profile-v1.0.html).
- [IPAWS-OPEN Interface Design Guide v4.07](https://content.govdelivery.com/attachments/USDHSFEMA/2026/08/17/file_attachments/3748713/IPAWS-OPEN-v4-07-00_InterfaceDesignGuide_Final.pdf), for IPAWS dissemination-channel requirements.
- [National Weather Service CAP v1.2 Documentation, 16 May 2017](https://www.weather.gov/media/alert/CAP_v12_guide_05-16-2017.pdf).

See `docs/CONFORMANCE.md` for the validation boundary and rule mapping.

## Deliberate boundaries

- This library implements CAP 1.2 XML, not the optional ASN.1 UPER representation.
- XML Digital Signature elements are accepted and round-tripped as required for a CAP consumer, but cryptographic signature verification is application policy and is not implemented here.
- Profile validation proves message-shape conformance. It does not prove sender authorization, IPAWS-OPEN acceptance, alert truth, delivery eligibility, or NWS lifecycle equivalence beyond what the CAP fields and documented profile parameters express.

## License

GNU General Public License v3.0 (GPL-3.0-only). See [LICENSE](LICENSE).
