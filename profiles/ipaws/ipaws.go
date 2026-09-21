// Package ipaws validates the OASIS CAP v1.2 IPAWS Profile version 1.0.
package ipaws

import (
	"fmt"
	"math"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"git.seasonalnet.org/SeasonalNet/capgo"
)

const ProfileCode = "IPAWSv1.0"

type Channel string

const (
	ChannelCAPExchange Channel = "CAPEXCH"
	ChannelPublic      Channel = "PUBLIC"
	ChannelEAS         Channel = "EAS"
	ChannelCMAS        Channel = "CMAS"
	ChannelNWEM        Channel = "NWEM"
	ChannelNWR         Channel = "NWR"
	ChannelHazCollect  Channel = "HazCollect" // Deprecated: use ChannelNWEM.
)

// Validator selects conditional IPAWS delivery-system rules. With no channels,
// it validates the rules that apply to every IPAWS message. Clock is optional;
// when nil, time.Now is used for the IPAWS five-minute sent-time check.
type Validator struct {
	Channels      []Channel
	Gubernatorial bool
	Clock         func() time.Time
}

func (Validator) Name() string { return "IPAWS 1.0" }

// Validate applies the unconditional IPAWS Profile rules.
func Validate(alert *cap.Alert) cap.Report { return Validator{}.Validate(alert) }

var (
	sameEventPattern    = regexp.MustCompile(`^[A-Z]{3}$`)
	sameLocationPattern = regexp.MustCompile(`^\d{6}$`)
	allowedEASOrg       = map[string]struct{}{"PEP": {}, "EAS": {}, "WXR": {}, "CIV": {}}
	allowedSAMEEvents   = map[string]struct{}{
		"ADR": {}, "AVA": {}, "AVW": {}, "BZW": {}, "BLU": {}, "CAE": {}, "CDW": {}, "CEM": {},
		"CFW": {}, "CFA": {}, "DSW": {}, "DMO": {}, "EQW": {}, "EVI": {}, "EWW": {}, "FRW": {},
		"FFW": {}, "FFA": {}, "FFS": {}, "FLW": {}, "FLA": {}, "FLS": {}, "HMW": {}, "HWW": {},
		"HWA": {}, "HUW": {}, "HUA": {}, "HLS": {}, "LEW": {}, "LAE": {}, "MEP": {}, "NMN": {},
		"TOE": {}, "NUW": {}, "RHW": {}, "RMT": {}, "RWT": {}, "SVR": {}, "SVA": {}, "SVS": {},
		"SQW": {}, "SPW": {}, "SMW": {}, "SPS": {}, "SSA": {}, "SSW": {}, "TOR": {}, "TOA": {},
		"TRW": {}, "TRA": {}, "TSW": {}, "TSA": {}, "VOW": {}, "WSW": {}, "WSA": {}, "EAN": {},
		"NIC": {}, "NPT": {}, "NWS": {},
	}
	allowedBlockChannels = map[string]struct{}{"NWEM": {}, "EAS": {}, "CMAS": {}}
	weaHandlingEvents    = map[string]map[string]struct{}{
		"Imminent Threat": {"AVW": {}, "BLU": {}, "CDW": {}, "CEM": {}, "EQW": {}, "EVI": {}, "FRW": {}, "HMW": {}, "LEW": {}, "NUW": {}, "RHW": {}, "SPW": {}, "VOW": {}},
		"Public Safety":   {"AVW": {}, "BLU": {}, "CDW": {}, "CEM": {}, "EQW": {}, "EVI": {}, "FRW": {}, "HMW": {}, "LAE": {}, "LEW": {}, "MEP": {}, "NUW": {}, "RHW": {}, "SPW": {}, "TOE": {}, "VOW": {}},
		"Amber":           {"CAE": {}},
		"WEA Test":        {"DMO": {}, "NPT": {}, "RMT": {}, "RWT": {}},
		"Presidential":    {"EAN": {}},
		"Earthquake":      {"EQW": {}},
	}
)

// SAMEEventCodes returns the sorted current SAME/FCC event-code values
// accepted by the IPAWS profile validator, including NWS's special value.
func SAMEEventCodes() []string {
	result := make([]string, 0, len(allowedSAMEEvents))
	for value := range allowedSAMEEvents {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

// IsSAMEEventCode reports whether value is an accepted SAME event code.
func IsSAMEEventCode(value string) bool {
	_, ok := allowedSAMEEvents[value]
	return ok
}

func (validator Validator) Validate(alert *cap.Alert) cap.Report {
	var report cap.Report
	if alert == nil {
		report.Add(cap.LevelError, "IPAWS-ROOT", "/alert", "alert is nil")
		return report
	}

	if !hasString(alert.Codes, ProfileCode) {
		report.Add(cap.LevelError, "IPAWS-CODE", "/alert/code", "IPAWSv1.0 profile code is required")
	}
	if alert.Scope == cap.ScopePublic && alert.Status != cap.StatusActual {
		report.Add(cap.LevelError, "IPAWS-STATUS", "/alert/status", "messages for public dissemination must use status Actual")
	}
	validateAlertTokens(&report, alert)

	noInfoAllowed := alert.MsgType == cap.MsgTypeCancel || alert.Scope == cap.ScopePrivate
	if len(alert.Info) == 0 {
		if !noInfoAllowed {
			report.Add(cap.LevelError, "IPAWS-INFO", "/alert/info", "at least one info block is required")
		}
		return report
	}

	if !alert.Sent.IsZero() {
		clock := time.Now
		if validator.Clock != nil {
			clock = validator.Clock
		}
		delta := alert.Sent.Sub(clock())
		if delta > 5*time.Minute || delta < -5*time.Minute {
			report.Add(cap.LevelError, "IPAWS-SENT", "/alert/sent", "sent must be within five minutes of the current time")
		}
	}

	var baseCategories []string
	var baseCodes []string
	for index := range alert.Info {
		info := &alert.Info[index]
		path := fmt.Sprintf("/alert/info[%d]", index+1)
		categories := normalizedCategories(info.Categories)
		codes := normalizedPairs(info.EventCodes)
		if index == 0 {
			baseCategories, baseCodes = categories, codes
		} else {
			if !reflect.DeepEqual(baseCategories, categories) {
				report.Add(cap.LevelError, "IPAWS-INFO-CATEGORY", path+"/category", "all info blocks must have the same category values")
			}
			if !reflect.DeepEqual(baseCodes, codes) {
				report.Add(cap.LevelError, "IPAWS-INFO-EVENTCODE", path+"/eventCode", "all info blocks must have the same eventCode values")
			}
		}

		if len(info.EventCodes) == 0 {
			report.Add(cap.LevelError, "IPAWS-EVENTCODE", path+"/eventCode", "eventCode is required")
		}
		sameEvents := cap.Values(info.EventCodes, "SAME")
		for _, value := range sameEvents {
			if !sameEventPattern.MatchString(value) || !IsSAMEEventCode(value) {
				report.Add(cap.LevelError, "IPAWS-SAME-EVENT", path+"/eventCode", "SAME eventCode must be an accepted three-letter code")
			}
		}
		if len(sameEvents) > 1 {
			report.Add(cap.LevelWarning, "IPAWS-SAME-EVENT", path+"/eventCode", "an info block should contain only one SAME eventCode")
		}

		eas := validator.channelEnabled(info, ChannelEAS)
		nwem := validator.channelEnabled(info, ChannelNWEM) || validator.channelEnabled(info, ChannelNWR) || validator.channelEnabled(info, ChannelHazCollect)
		cmas := validator.channelEnabled(info, ChannelCMAS)
		if eas || nwem || cmas {
			if len(sameEvents) != 1 {
				report.Add(cap.LevelError, "IPAWS-SAME-EVENT", path+"/eventCode", "dissemination messages require exactly one SAME eventCode")
			}
		}

		if info.Expires == nil {
			report.Add(cap.LevelError, "IPAWS-EXPIRES", path+"/expires", "expires is required")
		} else if !alert.Sent.IsZero() && !info.Expires.After(alert.Sent.Time) {
			report.Add(cap.LevelError, "IPAWS-EXPIRES", path+"/expires", "expires must be later than sent")
		} else if !alert.Sent.IsZero() && info.Expires.Sub(alert.Sent.Time) < 5*time.Minute {
			report.Add(cap.LevelWarning, "IPAWS-EXPIRES", path+"/expires", "expires should be at least five minutes after sent")
		}
		if eas || nwem {
			if info.Expires != nil && !alert.Sent.IsZero() && info.Expires.Sub(alert.Sent.Time) > 99*time.Hour+30*time.Minute {
				report.Add(cap.LevelError, "IPAWS-EAS-EXPIRES", path+"/expires", "EAS and NWEM expires must not exceed 99.5 hours after sent")
			}
			if strings.TrimSpace(info.Description) == "" {
				report.Add(cap.LevelError, "IPAWS-DESCRIPTION", path+"/description", "EAS and NWEM messages require description")
			}
		}
		if cmas && info.Expires != nil && !alert.Sent.IsZero() && info.Expires.Sub(alert.Sent.Time) > 24*time.Hour {
			report.Add(cap.LevelError, "IPAWS-CMAS-EXPIRES", path+"/expires", "CMAS expires must not exceed 24 hours after sent")
		}
		if strings.TrimSpace(info.Description) == "" && !eas && !nwem {
			report.Add(cap.LevelWarning, "IPAWS-DESCRIPTION", path+"/description", "a meaningful description is recommended")
		}
		if strings.TrimSpace(info.Headline) == "" {
			report.Add(cap.LevelWarning, "IPAWS-HEADLINE", path+"/headline", "a headline is recommended")
		}
		if strings.TrimSpace(info.Instruction) == "" {
			report.Add(cap.LevelWarning, "IPAWS-INSTRUCTION", path+"/instruction", "a meaningful instruction is recommended")
		}
		if info.Web != "" {
			validateIPAWSURI(&report, info.Web, path+"/web", "web")
		}
		if eas || nwem {
			if nwem && info.SenderName != "" && strings.Count(info.SenderName, ",") != 2 {
				report.Add(cap.LevelWarning, "IPAWS-SENDERNAME", path+"/senderName", "NWEM senderName is recommended in COGID,CogName,Requesting Agency form")
			}
			if utf8.RuneCountInString(info.Description)+utf8.RuneCountInString(info.Instruction) > 1800 {
				report.Add(cap.LevelWarning, "IPAWS-EAS-TEXT", path, "description and instruction together exceed 1800 characters")
			}
		}

		if len(info.Areas) == 0 {
			report.Add(cap.LevelError, "IPAWS-AREA", path+"/area", "at least one area block is required")
		}
		hasGeocode := false
		hasSAMEGeocode := false
		for areaIndex, area := range info.Areas {
			areaPath := fmt.Sprintf("%s/area[%d]", path, areaIndex+1)
			if len(area.Geocodes) > 0 {
				hasGeocode = true
			}
			sameLocations := cap.Values(area.Geocodes, "SAME")
			if len(sameLocations) > 0 {
				hasSAMEGeocode = true
			}
			for _, value := range sameLocations {
				if !sameLocationPattern.MatchString(value) {
					report.Add(cap.LevelError, "IPAWS-SAME-GEOCODE", areaPath+"/geocode", "SAME location values must contain six digits")
				}
			}
			if len(area.Polygons) == 0 && len(area.Circles) == 0 {
				report.Add(cap.LevelWarning, "IPAWS-GEOSPATIAL", areaPath, "polygon or circle targeting is recommended when possible")
			}
			validateAreaGeometry(&report, area, areaPath, nwem, cmas)
			if (eas || nwem) && len(area.Geocodes) > 31 {
				report.Add(cap.LevelWarning, "IPAWS-GEOCODE-LIMIT", areaPath+"/geocode", "EAS and NWEM process at most 31 geocodes")
			}
		}
		if !hasGeocode {
			report.Add(cap.LevelError, "IPAWS-GEOCODE", path+"/area", "at least one geocode is required")
		}
		if !hasSAMEGeocode {
			if eas || nwem || cmas {
				report.Add(cap.LevelError, "IPAWS-SAME-GEOCODE", path+"/area/geocode", "dissemination messages require a SAME six-digit geocode")
			} else {
				report.Add(cap.LevelWarning, "IPAWS-SAME-GEOCODE", path+"/area/geocode", "a SAME six-digit location geocode is recommended")
			}
		}

		if eas || nwem || len(cap.Values(info.Parameters, "EAS-ORG")) > 0 {
			validateEASOrg(&report, info, path)
		}
		if validator.Gubernatorial && eas && !hasPair(info.Parameters, "EAS-Must-Carry", "TRUE") {
			report.Add(cap.LevelError, "IPAWS-MUST-CARRY", path+"/parameter", "gubernatorial must-carry messages require EAS-Must-Carry=TRUE")
		}
		validateBlockChannels(&report, info, path)
		if cmas {
			validateCMAS(&report, info, path, sameEvents)
		}
		for parameterIndex, pair := range info.Parameters {
			if pair.ValueName == "DBGFBYPASS" && !strings.EqualFold(pair.Value, "TRUE") && !strings.EqualFold(pair.Value, "FALSE") {
				report.Add(cap.LevelError, "IPAWS-DBGFBYPASS", fmt.Sprintf("%s/parameter[%d]/value", path, parameterIndex+1), "DBGFBYPASS must be TRUE or FALSE")
			}
		}
		validateResources(&report, info, path, eas)
	}
	return report
}

func validateAlertTokens(report *cap.Report, alert *cap.Alert) {
	if strings.TrimSpace(alert.Source) == "" {
		report.Add(cap.LevelWarning, "IPAWS-SOURCE", "/alert/source", "source is recommended and missing source produces IPAWS advisory 225")
	} else if utf8.RuneCountInString(alert.Source) > 30 {
		report.Add(cap.LevelWarning, "IPAWS-SOURCE", "/alert/source", "source is recommended to be no more than 30 characters")
	}
	if strings.ContainsAny(alert.Identifier, " ,'<>&{}|\\/^~[]()") {
		report.Add(cap.LevelError, "IPAWS-IDENTIFIER", "/alert/identifier", "identifier contains an IPAWS-restricted character")
	}
	if strings.ContainsAny(alert.Sender, " ,\"<>&{}|\\^~") {
		report.Add(cap.LevelError, "IPAWS-SENDER", "/alert/sender", "sender contains an IPAWS-restricted character")
	}
}

func validateEASOrg(report *cap.Report, info *cap.Info, path string) {
	values := cap.Values(info.Parameters, "EAS-ORG")
	if len(values) == 0 {
		report.Add(cap.LevelError, "IPAWS-EAS-ORG", path+"/parameter", "EAS and NWEM messages require EAS-ORG")
		return
	}
	for _, value := range values {
		if _, ok := allowedEASOrg[value]; !ok {
			report.Add(cap.LevelError, "IPAWS-EAS-ORG", path+"/parameter", "EAS-ORG must be PEP, EAS, WXR, or CIV")
		}
	}
}

func validateBlockChannels(report *cap.Report, info *cap.Info, path string) {
	for index, pair := range info.Parameters {
		if pair.ValueName != "BLOCKCHANNEL" {
			continue
		}
		if _, ok := allowedBlockChannels[pair.Value]; !ok {
			report.Add(cap.LevelError, "IPAWS-BLOCKCHANNEL", fmt.Sprintf("%s/parameter[%d]/value", path, index+1), "BLOCKCHANNEL must be NWEM, EAS, or CMAS")
		}
	}
}

func validateCMAS(report *cap.Report, info *cap.Info, path string, sameEvents []string) {
	if info.Language != "en-US" && info.Language != "es-US" {
		report.Add(cap.LevelError, "IPAWS-CMAS-LANGUAGE", path+"/language", "CMAS language must be en-US or es-US")
	}
	responseAllowed := map[cap.ResponseType]struct{}{
		cap.ResponseShelter: {}, cap.ResponseEvacuate: {}, cap.ResponsePrepare: {}, cap.ResponseExecute: {},
		cap.ResponseAvoid: {}, cap.ResponseMonitor: {},
	}
	for index, response := range info.ResponseTypes {
		if _, ok := responseAllowed[response]; !ok {
			report.Add(cap.LevelError, "IPAWS-CMAS-RESPONSE", fmt.Sprintf("%s/responseType[%d]", path, index+1), "responseType is not allowed for CMAS")
		}
	}
	if info.Urgency != cap.UrgencyImmediate && info.Urgency != cap.UrgencyExpected {
		report.Add(cap.LevelError, "IPAWS-CMAS-USC", path+"/urgency", "CMAS urgency must be Immediate or Expected")
	}
	if info.Severity != cap.SeverityExtreme && info.Severity != cap.SeveritySevere {
		report.Add(cap.LevelError, "IPAWS-CMAS-USC", path+"/severity", "CMAS severity must be Extreme or Severe")
	}
	if info.Certainty != cap.CertaintyObserved && info.Certainty != cap.CertaintyLikely {
		report.Add(cap.LevelError, "IPAWS-CMAS-USC", path+"/certainty", "CMAS certainty must be Observed or Likely")
	}

	textValues := cap.Values(info.Parameters, "CMAMtext")
	if len(textValues) == 0 || strings.TrimSpace(textValues[0]) == "" {
		report.Add(cap.LevelError, "IPAWS-CMAMTEXT", path+"/parameter", "CMAMtext is required for CMAS")
	} else {
		validateCMAMText(report, textValues[0], path+"/parameter")
	}
	for index, value := range cap.Values(info.Parameters, "CMAMlongtext") {
		if utf8.RuneCountInString(value) > 360 {
			report.Add(cap.LevelError, "IPAWS-CMAMLONGTEXT", fmt.Sprintf("%s/parameter[%d]/value", path, index+1), "CMAMlongtext must not exceed 360 characters")
		}
		if containsCMASRestricted(value) {
			report.Add(cap.LevelError, "IPAWS-CMAMTEXT-CHARACTERS", fmt.Sprintf("%s/parameter[%d]/value", path, index+1), "CMAMlongtext contains an IPAWS-restricted character")
		}
	}

	handling := cap.Values(info.Parameters, "WEAHandling")
	if len(handling) != 1 {
		report.Add(cap.LevelError, "IPAWS-WEA-HANDLING", path+"/parameter", "exactly one WEAHandling parameter is required for CMAS")
	} else if len(sameEvents) == 1 {
		allowed, ok := weaHandlingEvents[handling[0]]
		if !ok {
			report.Add(cap.LevelError, "IPAWS-WEA-HANDLING", path+"/parameter", "unsupported WEAHandling value")
		} else if _, allowedForEvent := allowed[sameEvents[0]]; !allowedForEvent {
			report.Add(cap.LevelError, "IPAWS-WEA-HANDLING", path+"/parameter", "WEAHandling does not match the SAME event code")
		}
	}
}

func validateCMAMText(report *cap.Report, value, path string) {
	if utf8.RuneCountInString(value) > 90 {
		report.Add(cap.LevelError, "IPAWS-CMAMTEXT", path+"/value", "CMAMtext must not exceed 90 characters")
	}
	if containsCMASRestricted(value) {
		report.Add(cap.LevelError, "IPAWS-CMAMTEXT-CHARACTERS", path+"/value", "CMAMtext contains an IPAWS-restricted character")
	}
}

func containsCMASRestricted(value string) bool { return strings.ContainsAny(value, "{}|\\^~[]") }

func validateResources(report *cap.Report, info *cap.Info, path string, eas bool) {
	for resourceIndex, resource := range info.Resources {
		if resource.Description != "EAS Broadcast Content" || !eas {
			continue
		}
		if !hasString([]string{"audio/x-ipaws-audio-mp3", "audio/x-ipaws-streaming-audio", "video/x-ipaws-video", "video/x-ipaws-streaming-video"}, resource.MIMEType) {
			report.Add(cap.LevelError, "IPAWS-EAS-RESOURCE", fmt.Sprintf("%s/resource[%d]/mimeType", path, resourceIndex+1), "EAS Broadcast Content requires an IPAWS broadcast MIME type")
		}
		if resource.URI != "" {
			validateIPAWSURI(report, resource.URI, fmt.Sprintf("%s/resource[%d]/uri", path, resourceIndex+1), "uri")
		}
	}
}

func validateIPAWSURI(report *cap.Report, value, path, name string) {
	if len(value) > 2083 || strings.ContainsAny(value, " ?") || (!strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://")) || !strings.Contains(strings.TrimPrefix(strings.TrimPrefix(value, "http://"), "https://"), ".") {
		report.Add(cap.LevelError, "IPAWS-URI", path, name+" must be an http(s) URI without spaces, '?' or an overlong value")
	}
}

func (validator Validator) channelEnabled(info *cap.Info, channel Channel) bool {
	if !validator.hasAnyChannel(channel) {
		return false
	}
	name := string(channel)
	if channel == ChannelNWEM || channel == ChannelNWR || channel == ChannelHazCollect {
		name = "NWEM"
	}
	for _, pair := range info.Parameters {
		if pair.ValueName == "BLOCKCHANNEL" && pair.Value == name {
			return false
		}
	}
	return true
}

func validateAreaGeometry(report *cap.Report, area cap.Area, path string, nwem, cmas bool) {
	shapeCount := len(area.Circles)
	nodeCount := len(area.Circles)
	for index, polygon := range area.Polygons {
		polygonPath := fmt.Sprintf("%s/polygon[%d]", path, index+1)
		if polygon != strings.TrimSpace(polygon) {
			report.Add(cap.LevelError, "IPAWS-POLYGON", polygonPath, "polygon must not have leading or trailing whitespace")
		}
		points, err := cap.ParsePolygon(polygon)
		if err != nil {
			continue
		}
		shapeCount++
		nodeCount += len(points)
		if len(points)-1 > 200 {
			report.Add(cap.LevelError, "IPAWS-POLYGON", polygonPath, "polygon exceeds the 200-vertex IPAWS limit")
		}
		if nwem && len(points)-1 > 20 {
			report.Add(cap.LevelError, "IPAWS-NWEM-POLYGON", polygonPath, "NWEM polygons must not exceed 20 discrete vertices")
		}
		if cmas && len(points)-1 > 100 {
			report.Add(cap.LevelError, "IPAWS-CMAS-POLYGON", polygonPath, "CMAS polygons must not exceed 100 discrete vertices")
		}
		if polygonHasInvalidGeometry(points) {
			report.Add(cap.LevelError, "IPAWS-POLYGON", polygonPath, "polygon has duplicate vertices, self-intersection, or zero area")
		}
		if cmas && coordinatePrecision(polygon) > 4 {
			report.Add(cap.LevelWarning, "IPAWS-GEOMETRY-PRECISION", polygonPath, "CMAS polygon coordinates exceed four decimal places")
		}
	}
	for index, circle := range area.Circles {
		circlePath := fmt.Sprintf("%s/circle[%d]", path, index+1)
		if circle != strings.TrimSpace(circle) {
			report.Add(cap.LevelError, "IPAWS-CIRCLE", circlePath, "circle must not have leading or trailing whitespace")
		}
		if nwem {
			report.Add(cap.LevelError, "IPAWS-NWEM-CIRCLE", circlePath, "NWR/NWEM does not accept circle areas")
		}
		if cmas && coordinatePrecision(circle) > 4 {
			report.Add(cap.LevelWarning, "IPAWS-GEOMETRY-PRECISION", circlePath, "CMAS circle coordinates exceed four decimal places")
		}
	}
	if cmas && (shapeCount > 10 || nodeCount > 100) {
		report.Add(cap.LevelError, "IPAWS-CMAS-SHAPES", path, "CMAS areas must not exceed 10 shapes or 100 coordinate pairs")
	}
}

func polygonHasInvalidGeometry(points []cap.Point) bool {
	if len(points) < 4 {
		return true
	}
	area := 0.0
	for index := 0; index < len(points)-1; index++ {
		area += points[index].Longitude*points[index+1].Latitude - points[index+1].Longitude*points[index].Latitude
		for other := index + 1; other < len(points)-1; other++ {
			if points[index] == points[other] && (index != 0 || other != len(points)-2) {
				return true
			}
		}
	}
	if math.Abs(area) < 1e-12 {
		return true
	}
	segments := len(points) - 1
	for first := 0; first < segments; first++ {
		for second := first + 1; second < segments; second++ {
			if second == first+1 || (first == 0 && second == segments-1) {
				continue
			}
			if segmentsIntersect(points[first], points[first+1], points[second], points[second+1]) {
				return true
			}
		}
	}
	return false
}

func segmentsIntersect(a, b, c, d cap.Point) bool {
	orientation := func(p, q, r cap.Point) float64 {
		return (q.Longitude-p.Longitude)*(r.Latitude-p.Latitude) - (q.Latitude-p.Latitude)*(r.Longitude-p.Longitude)
	}
	o1, o2 := orientation(a, b, c), orientation(a, b, d)
	o3, o4 := orientation(c, d, a), orientation(c, d, b)
	return (o1 > 0 && o2 < 0 || o1 < 0 && o2 > 0) && (o3 > 0 && o4 < 0 || o3 < 0 && o4 > 0)
}

func coordinatePrecision(value string) int {
	maxPrecision := 0
	for _, field := range strings.Fields(value) {
		for _, coordinate := range strings.Split(field, ",") {
			if point := strings.IndexByte(coordinate, '.'); point >= 0 {
				if precision := len(coordinate) - point - 1; precision > maxPrecision {
					maxPrecision = precision
				}
			}
		}
	}
	return maxPrecision
}

// ValidateActiveReferences checks the IPAWS requirement that Update and
// Cancel messages reference all related, unexpired messages. Callers supply
// the set of related prior messages affected by current.
func ValidateActiveReferences(current *cap.Alert, candidates ...*cap.Alert) cap.Report {
	var report cap.Report
	if current == nil {
		report.Add(cap.LevelError, "IPAWS-REFERENCES", "/alert", "current alert is nil")
		return report
	}
	if current.MsgType != cap.MsgTypeUpdate && current.MsgType != cap.MsgTypeCancel && current.MsgType != cap.MsgTypeAck && current.MsgType != cap.MsgTypeError {
		return report
	}
	if current.MsgType == cap.MsgTypeAck || current.MsgType == cap.MsgTypeError {
		references, err := cap.ParseReferences(current.References)
		if err != nil {
			report.Add(cap.LevelError, "IPAWS-REFERENCES", "/alert/references", err.Error())
			return report
		}
		for _, reference := range references {
			found := false
			for _, candidate := range candidates {
				if candidate == nil || candidate.Sender != reference.Sender || candidate.Identifier != reference.Identifier || !candidate.Sent.Equal(reference.Sent.Time) {
					continue
				}
				found = true
				if !cap.ActiveAt(candidate, current.Sent.Time) {
					report.Add(cap.LevelError, "IPAWS-REFERENCES", "/alert/references", "referenced message is expired: "+reference.String())
				}
				break
			}
			if !found {
				report.Add(cap.LevelError, "IPAWS-REFERENCES", "/alert/references", "referenced message was not found: "+reference.String())
			}
		}
		return report
	}
	missing, err := cap.MissingActiveReferences(current, candidates...)
	if err != nil {
		report.Add(cap.LevelError, "IPAWS-REFERENCES", "/alert/references", err.Error())
		return report
	}
	for _, reference := range missing {
		report.Add(cap.LevelError, "IPAWS-REFERENCES", "/alert/references", "active message is not referenced: "+reference.String())
	}
	return report
}

func (validator Validator) hasAnyChannel(channels ...Channel) bool {
	for _, selected := range validator.Channels {
		for _, candidate := range channels {
			if selected == candidate {
				return true
			}
		}
	}
	return false
}

func normalizedCategories(values []cap.Category) []string {
	result := make([]string, len(values))
	for i, value := range values {
		result[i] = string(value)
	}
	sort.Strings(result)
	return result
}

func normalizedPairs(values []cap.ValuePair) []string {
	result := make([]string, len(values))
	for i, value := range values {
		result[i] = value.ValueName + "=" + value.Value
	}
	sort.Strings(result)
	return result
}

func hasString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func hasPair(values []cap.ValuePair, name, value string) bool {
	for _, pair := range values {
		if pair.ValueName == name && pair.Value == value {
			return true
		}
	}
	return false
}
