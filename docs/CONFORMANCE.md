# Conformance boundary

The library separates XML parsing, base CAP validation, and profile validation. A producer should run `cap.ValidateWith` with the applicable profile immediately before publication. A consumer should decode first, retain the original bytes when signature verification is required, and then apply its selected validators.

## CAP 1.2

`cap.Decode` enforces:

- the `urn:oasis:names:tc:emergency:cap:1.2` root;
- schema element order, cardinality for non-repeating elements, and valid nesting;
- well-formed XML, one document element, a configurable byte limit, and a configurable depth limit;
- no DTD or XML directive processing; and
- opaque acceptance of alert-level extension elements so XML Digital Signature content can be consumed and retained.

`cap.Validate` enforces:

- required fields and all schema enumerations;
- the exact CAP date-time lexical form with an explicit numeric offset;
- `restriction`/`addresses` requirements selected by `scope`;
- lifecycle reference syntax and the reference requirement for non-`Alert` message types;
- `info`, resource, geospatial, altitude, and ceiling constraints; and
- the CAP 1.2 XML Digital Signature extension namespace boundary.

Recommended note content for `Exercise`, `Test`, and `Error` messages is reported as warnings.

## CAP-CP Beta 0.4A

The comparison target is the published [CAP-CP Introduction and Rule Set Beta 0.4A](https://www.publicsafety.gc.ca/cnt/rsrcs/pblctns/capcp-ntro-rl-st/Beta-04a-en.pdf).

The `capcp` validator maps its rule IDs directly to rules 2-18 in the published rule set. It enforces the profile marker, public-distribution info cardinality, a single subject event, explicit languages, event/location reference forms, area/geocode requirements, reference presence, and the `MinorChange` and `AutoTranslated` layers (including the one-marker-per-info constraint). Recommendations for expiry, sender name, response type, and polygon/circle geometry are warnings. Private COG-to-COG messages may omit `info` as permitted by CAP-CP.

Event References and Location References are independently versioned managed lists. Their complete contents are not frozen in this module. Callers can inject the governed revision through `capcp.Validator.EventCodes` and `LocationCodes`. `ValidateActiveReferences` checks rule 12 for Update and Cancel messages when candidate prior messages are available.

## IPAWS 1.0

The `ipaws` validator implements the CAP-message requirements in IPAWS-OPEN Interface Design Guide v4.07. It enforces the profile marker, public `Actual` status, IPAWS identifier/sender restrictions, the five-minute `sent` window, consistent category/event-code values across language blocks, current SAME event-code membership, expiry and area/geocode requirements, CAP reference syntax, geometry limits, `BLOCKCHANNEL`, EAS/NWEM description and expiry rules, the four allowed EAS organization codes, CMAS/WEA language/USC/CMAM/WEAHandling rules, and the four IPAWS EAS broadcast MIME types. It reports guide recommendations such as source, five-minute minimum expiry, precise geometry, and long text as warnings.

Delivery-specific rules are activated with `ipaws.Validator.Channels`: `ChannelEAS`, `ChannelNWEM`/`ChannelNWR`, and `ChannelCMAS`; `ChannelCAPExchange` and `ChannelPublic` are available when a caller wants to label those paths explicitly. `ChannelHazCollect` remains as a deprecated compatibility name for NWEM. The library does not guess a destination channel from alert prose.

Cancel messages and private COG-to-COG messages may omit `info` as permitted by the guide. `ipaws.ValidateActiveReferences` checks the all-related-active-messages reference rule when the caller supplies the affected prior-message set.

IPAWS-OPEN SOAP/WSDL transport, COG authorization, X.509 trust policy, and XML-signature cryptographic verification remain outside this dependency-free CAP message library; those operations must be performed by the submitting service.

## NWS CAP v1.2

The comparison target is the [NWS CAP v1.2 producer guide](https://www.weather.gov/media/alert/CAP_v12_guide_05-16-2017.pdf), supplemented by the current [NWR-SAME event-code catalog](https://www.weather.gov/dsb/eventcodes).

The `nws` validator includes unconditional IPAWS validation, then checks the NWS producer formats for message identity, sender, public scope, required info and area blocks, the current [NWR-SAME event-code catalog](https://www.weather.gov/dsb/eventcodes) and [FCC 47 CFR 11.31 event codes](https://www.govinfo.gov/content/pkg/CFR-2024-title47-vol1/pdf/CFR-2024-title47-vol1-sec11-31.pdf), `NationalWeatherService` event codes, effective/onset/expiry behavior, office/text/web fields, EAS originator, channel blocking, SAME and UGC geography, and the documented NWS parameters. NWS identifiers are treated as opaque CAP identifiers, including API URNs.

The production feed emits `EAS-ORG=WXR` for NWS `NationalWeatherService` event codes whose final significance character is watch (`A`) or warning (`W`) and can activate EAS. It omits that parameter for recognized non-EAS significances such as advisory (`Y`) and statement (`S`) products. The validator uses this CAP event-code representation rather than depending on the optional VTEC parameter, which NWS is considering discontinuing.

NWS `Cancel` is not treated as synonymous with an ordinary VTEC cancellation. The validator checks serialization/profile rules; downstream lifecycle interpretation remains the caller's responsibility.

`nws.ValidateActiveReferences` checks that an NWS Update or Cancel references every active related message supplied by the caller. As with CAP-CP and IPAWS, this history-aware check is separate from single-message validation.
