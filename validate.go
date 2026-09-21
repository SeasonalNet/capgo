package cap

import (
	"encoding/base64"
	"fmt"
	"math"
	"mime"
	"net/url"
	"regexp"
	"strings"
	"unicode"
)

var languagePattern = regexp.MustCompile(`^[A-Za-z]{1,8}(?:-[A-Za-z0-9]{1,8})*$`)

var (
	validStatuses    = setOf(StatusActual, StatusExercise, StatusSystem, StatusTest, StatusDraft)
	validMsgTypes    = setOf(MsgTypeAlert, MsgTypeUpdate, MsgTypeCancel, MsgTypeAck, MsgTypeError)
	validScopes      = setOf(ScopePublic, ScopeRestricted, ScopePrivate)
	validCategories  = setOf(CategoryGeo, CategoryMet, CategorySafety, CategorySecurity, CategoryRescue, CategoryFire, CategoryHealth, CategoryEnv, CategoryTransport, CategoryInfra, CategoryCBRNE, CategoryOther)
	validResponses   = setOf(ResponseShelter, ResponseEvacuate, ResponsePrepare, ResponseExecute, ResponseAvoid, ResponseMonitor, ResponseAssess, ResponseAllClear, ResponseNone)
	validUrgencies   = setOf(UrgencyImmediate, UrgencyExpected, UrgencyFuture, UrgencyPast, UrgencyUnknown)
	validSeverities  = setOf(SeverityExtreme, SeveritySevere, SeverityModerate, SeverityMinor, SeverityUnknown)
	validCertainties = setOf(CertaintyObserved, CertaintyLikely, CertaintyPossible, CertaintyUnlikely, CertaintyUnknown)
)

// Validate applies the OASIS CAP 1.2 schema value domains and the additional
// normative semantic requirements in the CAP 1.2 data dictionary.
func Validate(alert *Alert) Report {
	var report Report
	if alert == nil {
		report.Add(LevelError, "CAP-ROOT", "/alert", "alert is nil")
		return report
	}
	if alert.XMLName.Local != "" && (alert.XMLName.Space != Namespace || alert.XMLName.Local != "alert") {
		report.Add(LevelError, "CAP-ROOT", "/alert", "root XML name is not the CAP 1.2 alert element")
	}
	validateToken(&report, "CAP-IDENTIFIER", "/alert/identifier", alert.Identifier)
	validateToken(&report, "CAP-SENDER", "/alert/sender", alert.Sender)
	if alert.Sent.IsZero() {
		report.Add(LevelError, "CAP-SENT", "/alert/sent", "sent is required")
	} else {
		validateDateTime(&report, "CAP-SENT", "/alert/sent", &alert.Sent)
	}
	checkEnum(&report, "CAP-STATUS", "/alert/status", alert.Status, validStatuses)
	checkEnum(&report, "CAP-MSGTYPE", "/alert/msgType", alert.MsgType, validMsgTypes)
	checkEnum(&report, "CAP-SCOPE", "/alert/scope", alert.Scope, validScopes)
	if alert.Scope == ScopeRestricted && strings.TrimSpace(alert.Restriction) == "" {
		report.Add(LevelError, "CAP-RESTRICTION", "/alert/restriction", "restriction is required when scope is Restricted")
	}
	if alert.Scope == ScopePrivate && len(ParseAddresses(alert.Addresses)) == 0 {
		report.Add(LevelError, "CAP-ADDRESSES", "/alert/addresses", "addresses are required when scope is Private")
	}
	if alert.MsgType != MsgTypeAlert && strings.TrimSpace(alert.References) == "" {
		report.Add(LevelError, "CAP-REFERENCES", "/alert/references", "references are required for Update, Cancel, Ack, and Error messages")
	}
	if alert.References != "" {
		if _, err := ParseReferences(alert.References); err != nil {
			report.Add(LevelError, "CAP-REFERENCES", "/alert/references", err.Error())
		}
	}
	if alert.Status == StatusExercise && strings.TrimSpace(alert.Note) == "" {
		report.Add(LevelWarning, "CAP-EXERCISE-NOTE", "/alert/note", "Exercise messages should identify the exercise in note")
	}
	if alert.Status == StatusTest && strings.TrimSpace(alert.Note) == "" {
		report.Add(LevelWarning, "CAP-TEST-NOTE", "/alert/note", "Test messages should identify the test in note")
	}
	if alert.MsgType == MsgTypeError && strings.TrimSpace(alert.Note) == "" {
		report.Add(LevelWarning, "CAP-ERROR-NOTE", "/alert/note", "Error messages should explain the error in note")
	}

	for index := range alert.Info {
		validateInfo(&report, alert, &alert.Info[index], index)
	}
	for index, extension := range alert.Extensions {
		if extension.XMLName.Space != XMLDSigNamespace {
			report.Add(LevelError, "CAP-EXTENSION", fmt.Sprintf("/alert/*[%d]", index+1), "CAP 1.2 only permits XML Digital Signature namespace extensions")
		}
	}
	return report
}

func validateInfo(report *Report, alert *Alert, info *Info, index int) {
	path := fmt.Sprintf("/alert/info[%d]", index+1)
	if info.Language != "" && !languagePattern.MatchString(info.Language) {
		report.Add(LevelError, "CAP-LANGUAGE", path+"/language", "language is not a valid xs:language value")
	}
	if len(info.Categories) == 0 {
		report.Add(LevelError, "CAP-CATEGORY", path+"/category", "at least one category is required")
	}
	for item, category := range info.Categories {
		checkEnum(report, "CAP-CATEGORY", fmt.Sprintf("%s/category[%d]", path, item+1), category, validCategories)
	}
	if strings.TrimSpace(info.Event) == "" {
		report.Add(LevelError, "CAP-EVENT", path+"/event", "event is required")
	}
	for item, response := range info.ResponseTypes {
		checkEnum(report, "CAP-RESPONSE", fmt.Sprintf("%s/responseType[%d]", path, item+1), response, validResponses)
	}
	checkEnum(report, "CAP-URGENCY", path+"/urgency", info.Urgency, validUrgencies)
	checkEnum(report, "CAP-SEVERITY", path+"/severity", info.Severity, validSeverities)
	checkEnum(report, "CAP-CERTAINTY", path+"/certainty", info.Certainty, validCertainties)
	validatePairs(report, "CAP-EVENTCODE", path+"/eventCode", info.EventCodes)
	validatePairs(report, "CAP-PARAMETER", path+"/parameter", info.Parameters)
	validateDateTime(report, "CAP-EFFECTIVE", path+"/effective", info.Effective)
	validateDateTime(report, "CAP-ONSET", path+"/onset", info.Onset)
	validateDateTime(report, "CAP-EXPIRES", path+"/expires", info.Expires)
	if info.Expires != nil {
		effective := alert.Sent
		if info.Effective != nil {
			effective = *info.Effective
		}
		if !effective.IsZero() && !info.Expires.After(effective.Time) {
			report.Add(LevelError, "CAP-EXPIRES", path+"/expires", "expires must be later than effective (or sent when effective is absent)")
		}
		if info.Onset != nil && !info.Expires.After(info.Onset.Time) {
			report.Add(LevelError, "CAP-ONSET", path+"/onset", "expires must be later than onset")
		}
	}
	if info.Web != "" && !validURI(info.Web) {
		report.Add(LevelError, "CAP-WEB", path+"/web", "web is not a valid URI")
	}
	for item := range info.Resources {
		validateResource(report, &info.Resources[item], fmt.Sprintf("%s/resource[%d]", path, item+1))
	}
	for item := range info.Areas {
		validateArea(report, &info.Areas[item], fmt.Sprintf("%s/area[%d]", path, item+1))
	}
}

func validateDateTime(report *Report, rule, path string, value *DateTime) {
	if value == nil {
		return
	}
	if _, err := value.MarshalText(); err != nil {
		report.Add(LevelError, rule, path, err.Error())
	}
}

func validateResource(report *Report, resource *Resource, path string) {
	if strings.TrimSpace(resource.Description) == "" {
		report.Add(LevelError, "CAP-RESOURCE-DESC", path+"/resourceDesc", "resourceDesc is required")
	}
	if _, _, err := mime.ParseMediaType(resource.MIMEType); err != nil {
		report.Add(LevelError, "CAP-MIMETYPE", path+"/mimeType", "mimeType must be a valid media type")
	}
	if resource.Size != nil && *resource.Size < 0 {
		report.Add(LevelError, "CAP-RESOURCE-SIZE", path+"/size", "size must not be negative")
	}
	if resource.URI != "" && !validURI(resource.URI) {
		report.Add(LevelError, "CAP-RESOURCE-URI", path+"/uri", "uri is not a valid URI")
	}
	if resource.DerefURI != "" {
		if _, err := decodeBase64(resource.DerefURI); err != nil {
			report.Add(LevelError, "CAP-DEREFURI", path+"/derefUri", "derefUri must contain base64-encoded data")
		}
	}
	if resource.Digest != "" {
		digest, err := decodeBase64(resource.Digest)
		if err != nil || len(digest) != 20 {
			report.Add(LevelError, "CAP-DIGEST", path+"/digest", "digest must be a base64-encoded 160-bit SHA-1 digest")
		}
	}
}

func validateArea(report *Report, area *Area, path string) {
	if strings.TrimSpace(area.Description) == "" {
		report.Add(LevelError, "CAP-AREA-DESC", path+"/areaDesc", "areaDesc is required")
	}
	for index, polygon := range area.Polygons {
		if _, err := ParsePolygon(polygon); err != nil {
			report.Add(LevelError, "CAP-POLYGON", fmt.Sprintf("%s/polygon[%d]", path, index+1), err.Error())
		}
	}
	for index, circle := range area.Circles {
		if _, _, err := ParseCircle(circle); err != nil {
			report.Add(LevelError, "CAP-CIRCLE", fmt.Sprintf("%s/circle[%d]", path, index+1), err.Error())
		}
	}
	validatePairs(report, "CAP-GEOCODE", path+"/geocode", area.Geocodes)
	if area.Altitude != nil && (math.IsNaN(*area.Altitude) || math.IsInf(*area.Altitude, 0)) {
		report.Add(LevelError, "CAP-ALTITUDE", path+"/altitude", "altitude must be a finite decimal")
	}
	if area.Ceiling != nil && (math.IsNaN(*area.Ceiling) || math.IsInf(*area.Ceiling, 0)) {
		report.Add(LevelError, "CAP-CEILING", path+"/ceiling", "ceiling must be a finite decimal")
	}
	if area.Ceiling != nil && area.Altitude == nil {
		report.Add(LevelError, "CAP-CEILING", path+"/ceiling", "ceiling requires altitude")
	}
	if area.Ceiling != nil && area.Altitude != nil && *area.Ceiling <= *area.Altitude {
		report.Add(LevelError, "CAP-CEILING", path+"/ceiling", "ceiling must be greater than altitude")
	}
}

func validatePairs(report *Report, rule, path string, pairs []ValuePair) {
	for index, pair := range pairs {
		itemPath := fmt.Sprintf("%s[%d]", path, index+1)
		if strings.TrimSpace(pair.ValueName) == "" {
			report.Add(LevelError, rule, itemPath+"/valueName", "valueName must not be empty")
		}
		if strings.TrimSpace(pair.Value) == "" {
			report.Add(LevelError, rule, itemPath+"/value", "value must not be empty")
		}
	}
}

func validateToken(report *Report, rule, path, value string) {
	if value == "" {
		report.Add(LevelError, rule, path, "value is required")
		return
	}
	if strings.ContainsAny(value, ",<>&") || strings.IndexFunc(value, unicode.IsSpace) >= 0 {
		report.Add(LevelError, rule, path, "value must not contain whitespace, commas, or XML-reserved characters")
	}
}

func checkEnum[T comparable](report *Report, rule, path string, value T, allowed map[T]struct{}) {
	if _, ok := allowed[value]; !ok {
		report.Add(LevelError, rule, path, fmt.Sprintf("unsupported value %v", value))
	}
}

func setOf[T comparable](values ...T) map[T]struct{} {
	result := make(map[T]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func validURI(value string) bool {
	if strings.IndexFunc(value, unicode.IsSpace) >= 0 || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return false
	}
	_, err := url.Parse(value)
	return err == nil
}

func decodeBase64(value string) ([]byte, error) {
	value = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, value)
	return base64.StdEncoding.DecodeString(value)
}
