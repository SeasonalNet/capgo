// Package nws validates the National Weather Service CAP v1.2 producer
// profile documented by NWS on 16 May 2017.
package nws

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"git.seasonalnet.org/SeasonalNet/capgo"
	"git.seasonalnet.org/SeasonalNet/capgo/profiles/ipaws"
)

const (
	Sender = "w-nws.webmaster@noaa.gov"

	EventCodeSAME    = "SAME"
	EventCodeNWS     = "NationalWeatherService"
	SAMEEventCodeNWS = "NWS"

	ParameterNWSHeadline         = "NWSheadline"
	ParameterEASORG              = "EAS-ORG"
	ParameterVTEC                = "VTEC"
	ParameterEventEndingTime     = "eventEndingTime"
	ParameterEventMotion         = "eventMotionDescription"
	ParameterWindGust            = "windGust"
	ParameterHailSize            = "hailSize"
	ParameterTornadoDetection    = "tornadoDetection"
	ParameterTornadoDamageThreat = "tornadoDamageThreat"
	ParameterWaterspoutDetection = "waterspoutDetection"
	ParameterBlockChannel        = "BLOCKCHANNEL"
	ParameterCMAMText            = "CMAMText"
)

// sameEventCodes contains the current NWR-SAME/FCC Part 11 event-code
// catalog. NWS also uses SAME=NWS for official CAP alerts that do not map to
// an FCC/NWR event code.
var sameEventCodes = map[string]struct{}{
	"ADR": {}, "AVW": {}, "AVA": {}, "BZW": {}, "BLU": {}, "CAE": {},
	"CDW": {}, "CEM": {}, "CFW": {}, "CFA": {}, "DSW": {}, "EQW": {},
	"EVI": {}, "EWW": {}, "FRW": {}, "FFW": {}, "FFA": {}, "FFS": {},
	"FLW": {}, "FLA": {}, "FLS": {}, "HMW": {}, "HWW": {}, "HWA": {},
	"HUW": {}, "HUA": {}, "HLS": {}, "LEW": {}, "LAE": {}, "MEP": {}, "NMN": {},
	"TOE": {}, "NUW": {}, "DMO": {}, "RHW": {}, "SVR": {}, "SVA": {},
	"SVS": {}, "SQW": {}, "SPW": {}, "SMW": {}, "SPS": {}, "SSA": {}, "SSW": {},
	"TOR": {}, "TOA": {}, "TRW": {}, "TRA": {}, "TSW": {}, "TSA": {},
	"VOW": {}, "WSW": {}, "WSA": {}, "EAN": {}, "NIC": {}, "NPT": {},
	"RMT": {}, "RWT": {},
}

// SAMEEventCodes returns the sorted current NWR-SAME/FCC Part 11 catalog,
// including NWS's special non-FCC value "NWS".
func SAMEEventCodes() []string {
	codes := make([]string, 0, len(sameEventCodes)+1)
	for code := range sameEventCodes {
		codes = append(codes, code)
	}
	codes = append(codes, SAMEEventCodeNWS)
	sort.Strings(codes)
	return codes
}

// IsSAMEEventCode reports whether value is an authorized current SAME event
// code or NWS's special non-FCC value.
func IsSAMEEventCode(value string) bool {
	if value == SAMEEventCodeNWS {
		return true
	}
	_, ok := sameEventCodes[value]
	return ok
}

// Validator validates NWS-produced CAP. IPAWS rules that apply to every
// message are included because NWS documents its feed as IPAWS 1.0 compliant.
// Clock is optional and is passed to the IPAWS sent-time check.
type Validator struct {
	Clock func() time.Time
}

func (Validator) Name() string { return "NWS CAP v1.2" }

// Validate applies the NWS and unconditional IPAWS rules.
func Validate(alert *cap.Alert) cap.Report { return Validator{}.Validate(alert) }

// ValidateActiveReferences checks that an NWS Update or Cancel names every
// active related message supplied by the caller. CAP carries the references,
// while the caller supplies the affected-message set from its history.
func ValidateActiveReferences(current *cap.Alert, candidates ...*cap.Alert) cap.Report {
	var report cap.Report
	if current == nil {
		report.Add(cap.LevelError, "NWS-REFERENCES", "/alert", "current alert is nil")
		return report
	}
	if current.MsgType != cap.MsgTypeUpdate && current.MsgType != cap.MsgTypeCancel {
		return report
	}
	missing, err := cap.MissingActiveReferences(current, candidates...)
	if err != nil {
		report.Add(cap.LevelError, "NWS-REFERENCES", "/alert/references", err.Error())
		return report
	}
	for _, reference := range missing {
		report.Add(cap.LevelError, "NWS-REFERENCES", "/alert/references", "active message is not referenced: "+reference.String())
	}
	return report
}

var (
	threeLetterPattern  = regexp.MustCompile(`^[A-Z]{3}$`)
	sameLocationPattern = regexp.MustCompile(`^\d{6}$`)
	ugcPattern          = regexp.MustCompile(`^[A-Z]{2}[CZ](?:\d{3}|ALL)$`)
	hailSizePattern     = regexp.MustCompile(`^\d+\.\d{2}$`)
	eventMotionPattern  = regexp.MustCompile(`(?s)^(.{25})\.\.\.storm\.\.\.((?:00[1-9]|0[1-9]\d|[12]\d\d|3[0-5]\d))DEG\.\.\.(0|[1-9]\d?)KT\.\.\.(.+)$`)
)

func (validator Validator) Validate(alert *cap.Alert) cap.Report {
	report := ipaws.Validator{Clock: validator.Clock}.Validate(alert)
	if alert == nil {
		return report
	}
	if alert.Sender != Sender {
		report.Add(cap.LevelError, "NWS-SENDER", "/alert/sender", "NWS sender must be "+Sender)
	}
	if alert.Scope != cap.ScopePublic {
		report.Add(cap.LevelError, "NWS-SCOPE", "/alert/scope", "NWS messages use Public scope")
	}
	if len(alert.Info) == 0 {
		report.Add(cap.LevelError, "NWS-INFO", "/alert/info", "NWS messages require one or more info blocks")
	}
	for index := range alert.Info {
		info := &alert.Info[index]
		path := fmt.Sprintf("/alert/info[%d]", index+1)
		if len(info.ResponseTypes) == 0 {
			report.Add(cap.LevelError, "NWS-RESPONSE", path+"/responseType", "responseType is required")
		}
		checkSAMECode(&report, info.EventCodes, path)
		checkSingleCode(&report, info.EventCodes, EventCodeNWS, path, threeLetterPattern)
		if info.Effective == nil {
			report.Add(cap.LevelError, "NWS-EFFECTIVE", path+"/effective", "effective is required")
		} else if !info.Effective.Equal(alert.Sent.Time) {
			report.Add(cap.LevelError, "NWS-EFFECTIVE", path+"/effective", "effective must identify the same instant as sent")
		}
		if info.Urgency != cap.UrgencyPast && info.Onset == nil {
			report.Add(cap.LevelError, "NWS-ONSET", path+"/onset", "onset is required unless urgency is Past")
		}
		if info.Expires == nil {
			report.Add(cap.LevelError, "NWS-EXPIRES", path+"/expires", "expires is required")
		}
		requireText(&report, "NWS-SENDERNAME", path+"/senderName", info.SenderName)
		requireText(&report, "NWS-HEADLINE", path+"/headline", info.Headline)
		requireText(&report, "NWS-DESCRIPTION", path+"/description", info.Description)
		if info.Urgency != cap.UrgencyPast {
			requireText(&report, "NWS-INSTRUCTION", path+"/instruction", info.Instruction)
		}
		requireText(&report, "NWS-WEB", path+"/web", info.Web)
		if isEASEvent(info.EventCodes) && !hasPair(info.Parameters, ParameterEASORG, "WXR") {
			report.Add(cap.LevelError, "NWS-EAS-ORG", path+"/parameter", "EAS-ORG=WXR is required for NWS event codes with significance A (watch) or W (warning)")
		}
		if !hasPair(info.Parameters, ParameterBlockChannel, "NWEM") {
			report.Add(cap.LevelError, "NWS-BLOCKCHANNEL", path+"/parameter", "NWS messages must block the NWEM channel")
		}
		if !isEASEvent(info.EventCodes) && !hasPair(info.Parameters, ParameterBlockChannel, "EAS") {
			report.Add(cap.LevelError, "NWS-BLOCKCHANNEL", path+"/parameter", "NWS messages not intended for EAS must block the EAS channel")
		}
		for parameterIndex, parameter := range info.Parameters {
			validateParameter(&report, parameter, fmt.Sprintf("%s/parameter[%d]", path, parameterIndex+1))
		}
		if len(info.Areas) == 0 {
			report.Add(cap.LevelError, "NWS-AREA", path+"/area", "at least one area block is required")
		}
		geocodes := make([]cap.ValuePair, 0)
		for areaIndex := range info.Areas {
			geocodes = append(geocodes, info.Areas[areaIndex].Geocodes...)
		}
		validateGeocodes(&report, geocodes, path+"/area/geocode")
	}
	return report
}

// isEASEvent follows the NWS CAP representation: the final character of the
// NationalWeatherService event code is the product significance. A (watch)
// and W (warning) products can activate EAS and require EAS-ORG=WXR. It
// intentionally does not depend on the optional VTEC parameter, which NWS is
// considering discontinuing.
func isEASEvent(eventCodes []cap.ValuePair) bool {
	values := cap.Values(eventCodes, EventCodeNWS)
	if len(values) != 1 || len(values[0]) != 3 {
		return true
	}
	significance := values[0][2]
	return significance == 'A' || significance == 'W'
}

func checkSingleCode(report *cap.Report, pairs []cap.ValuePair, name, path string, pattern *regexp.Regexp) {
	values := cap.Values(pairs, name)
	if len(values) != 1 {
		report.Add(cap.LevelError, "NWS-EVENTCODE", path+"/eventCode", "exactly one "+name+" eventCode is required")
		return
	}
	if !pattern.MatchString(values[0]) {
		report.Add(cap.LevelError, "NWS-EVENTCODE", path+"/eventCode", name+" eventCode must contain three uppercase letters")
	}
}

func checkSAMECode(report *cap.Report, pairs []cap.ValuePair, path string) {
	values := cap.Values(pairs, EventCodeSAME)
	if len(values) != 1 {
		report.Add(cap.LevelError, "NWS-EVENTCODE", path+"/eventCode", "exactly one SAME eventCode is required")
		return
	}
	if !IsSAMEEventCode(values[0]) {
		report.Add(cap.LevelError, "NWS-EVENTCODE", path+"/eventCode", "SAME eventCode must be an authorized NWR-SAME/FCC Part 11 code or NWS")
	}
}

func validateGeocodes(report *cap.Report, pairs []cap.ValuePair, path string) {
	same := cap.Values(pairs, "SAME")
	ugc := cap.Values(pairs, "UGC")
	if len(same) == 0 {
		report.Add(cap.LevelError, "NWS-SAME-GEOCODE", path, "at least one SAME geocode is required")
	}
	if len(ugc) == 0 {
		report.Add(cap.LevelError, "NWS-UGC", path, "at least one UGC geocode is required")
	}
	for _, value := range same {
		if !sameLocationPattern.MatchString(value) {
			report.Add(cap.LevelError, "NWS-SAME-GEOCODE", path, "SAME geocode must contain six digits")
		}
	}
	for _, value := range ugc {
		if !ugcPattern.MatchString(value) {
			report.Add(cap.LevelError, "NWS-UGC", path, "UGC must match SSCNNN, SSZNNN, SSCALL, or SSZALL")
		}
	}
}

func validateParameter(report *cap.Report, parameter cap.ValuePair, path string) {
	switch parameter.ValueName {
	case ParameterEventEndingTime:
		if _, err := cap.ParseDateTime(parameter.Value); err != nil {
			report.Add(cap.LevelError, "NWS-EVENT-END", path+"/value", err.Error())
		}
	case ParameterEventMotion:
		validateEventMotion(report, parameter.Value, path+"/value")
	case ParameterWindGust:
		value, err := strconv.Atoi(parameter.Value)
		if err != nil || value < 0 {
			report.Add(cap.LevelError, "NWS-WIND-GUST", path+"/value", "windGust must be a non-negative integer in miles per hour")
		}
	case ParameterHailSize:
		if !hailSizePattern.MatchString(parameter.Value) {
			report.Add(cap.LevelError, "NWS-HAIL-SIZE", path+"/value", "hailSize must be inches with two decimal places")
		}
	case ParameterTornadoDetection:
		if !oneOf(parameter.Value, "RADAR INDICATED", "OBSERVED", "POSSIBLE") {
			report.Add(cap.LevelError, "NWS-TORNADO-DETECTION", path+"/value", "unsupported tornadoDetection value")
		}
	case ParameterTornadoDamageThreat:
		if !oneOf(parameter.Value, "CONSIDERABLE", "CATASTROPHIC") {
			report.Add(cap.LevelError, "NWS-TORNADO-DAMAGE", path+"/value", "unsupported tornadoDamageThreat value")
		}
	case ParameterWaterspoutDetection:
		if !oneOf(parameter.Value, "OBSERVED", "POSSIBLE") {
			report.Add(cap.LevelError, "NWS-WATERSPOUT", path+"/value", "unsupported waterspoutDetection value")
		}
	case ParameterBlockChannel:
		if !oneOf(parameter.Value, "CMAS", "EAS", "NWEM", "PUBLIC") {
			report.Add(cap.LevelError, "NWS-BLOCKCHANNEL", path+"/value", "unsupported BLOCKCHANNEL value")
		}
		if parameter.Value == "PUBLIC" {
			report.Add(cap.LevelError, "NWS-BLOCKCHANNEL", path+"/value", "NWS messages must never block the PUBLIC channel")
		}
	case ParameterCMAMText:
		if utf8.RuneCountInString(parameter.Value) > 90 {
			report.Add(cap.LevelError, "NWS-CMAMTEXT", path+"/value", "CMAMText must not exceed 90 characters")
		}
	case ParameterVTEC:
		for _, line := range strings.Fields(parameter.Value) {
			if !strings.HasPrefix(line, "/") || !strings.HasSuffix(line, "/") {
				report.Add(cap.LevelError, "NWS-VTEC", path+"/value", "each VTEC string must be slash-delimited")
				break
			}
		}
	}
}

func validateEventMotion(report *cap.Report, value, path string) {
	matches := eventMotionPattern.FindStringSubmatch(value)
	if matches == nil {
		report.Add(cap.LevelError, "NWS-EVENT-MOTION", path, "eventMotionDescription has an invalid format")
		return
	}
	if _, err := cap.ParseDateTime(matches[1]); err != nil {
		report.Add(cap.LevelError, "NWS-EVENT-MOTION", path, "eventMotionDescription has an invalid date-time")
	}
	direction, _ := strconv.Atoi(matches[2])
	speed, _ := strconv.Atoi(matches[3])
	if direction > 359 || speed > 99 {
		report.Add(cap.LevelError, "NWS-EVENT-MOTION", path, "event motion direction or speed is out of range")
	}
	for index, coordinate := range strings.Fields(matches[4]) {
		if _, err := cap.ParsePoint(coordinate); err != nil {
			report.Add(cap.LevelError, "NWS-EVENT-MOTION", path, fmt.Sprintf("coordinate %d is invalid: %v", index+1, err))
		}
	}
}

func requireText(report *cap.Report, rule, path, value string) {
	if strings.TrimSpace(value) == "" {
		report.Add(cap.LevelError, rule, path, "value is required by the NWS producer profile")
	}
}

func hasPair(pairs []cap.ValuePair, name, value string) bool {
	for _, pair := range pairs {
		if pair.ValueName == name && pair.Value == value {
			return true
		}
	}
	return false
}
func oneOf(value string, choices ...string) bool {
	for _, choice := range choices {
		if value == choice {
			return true
		}
	}
	return false
}
