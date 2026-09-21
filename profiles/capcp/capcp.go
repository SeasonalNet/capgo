// Package capcp validates the Canadian Profile of CAP (CAP-CP) Beta 0.4A.
package capcp

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"unicode"

	"git.seasonalnet.org/SeasonalNet/capgo"
)

const (
	ProfileCode          = "profile:CAP-CP:0.4"
	EventValueNamePrefix = "profile:CAP-CP:Event:"
	LocationNamePrefix   = "profile:CAP-CP:Location:"
	MinorChangeName      = "profile:CAP-CP:0.4:MinorChange"
	AutoTranslatedName   = "profile:CAP-CP:0.4:AutoTranslated"
)

// Validator validates CAP-CP rules. Optional managed-list maps make validation
// authoritative for a particular Event References or Location References
// revision. Keys are compared case-insensitively.
type Validator struct {
	EventCodes             map[string]struct{}
	LocationCodes          map[string]struct{}
	DisableRecommendations bool
}

func (Validator) Name() string { return "CAP-CP Beta 0.4A" }

// Validate applies CAP-CP using syntax validation for managed values.
func Validate(alert *cap.Alert) cap.Report { return Validator{}.Validate(alert) }

func (validator Validator) Validate(alert *cap.Alert) cap.Report {
	var report cap.Report
	if alert == nil {
		report.Add(cap.LevelError, "CAPCP-ROOT", "/alert", "alert is nil")
		return report
	}
	if !contains(alert.Codes, ProfileCode) {
		report.Add(cap.LevelError, "CAPCP-03", "/alert/code", "CAP-CP profile code profile:CAP-CP:0.4 is required")
	}
	if alert.Scope == cap.ScopePublic && (alert.MsgType == cap.MsgTypeAlert || alert.MsgType == cap.MsgTypeUpdate || alert.MsgType == cap.MsgTypeCancel) && len(alert.Info) == 0 {
		report.Add(cap.LevelError, "CAPCP-05", "/alert/info", "public Alert, Update, and Cancel messages require an info block")
	}

	var baseline []string
	var baselineNonFree []string
	minorChangeUsed := false
	for index := range alert.Info {
		info := &alert.Info[index]
		path := fmt.Sprintf("/alert/info[%d]", index+1)
		if strings.TrimSpace(info.Language) == "" {
			report.Add(cap.LevelError, "CAPCP-06", path+"/language", "language is required")
		}
		profileCodes := capCPEventCodes(info.EventCodes)
		if len(profileCodes) == 0 {
			report.Add(cap.LevelError, "CAPCP-08", path+"/eventCode", "a CAP-CP Event References code is required")
		}
		values := make([]string, 0, len(profileCodes))
		for item, pair := range profileCodes {
			valuePath := fmt.Sprintf("%s/eventCode[%d]/value", path, item+1)
			if length := len([]rune(pair.Value)); length < 4 || length > 12 || strings.IndexFunc(pair.Value, unicode.IsSpace) >= 0 {
				report.Add(cap.LevelError, "CAPCP-08", valuePath, "CAP-CP event code must be 4 to 12 characters with no spaces")
			}
			if len(validator.EventCodes) != 0 {
				if !inFoldMap(validator.EventCodes, pair.Value) {
					report.Add(cap.LevelError, "CAPCP-08-LIST", valuePath, "event code is not in the configured CAP-CP Event References list")
				}
			}
			values = append(values, strings.ToLower(pair.Value))
		}
		sort.Strings(values)
		values = unique(values)
		if len(values) > 1 {
			report.Add(cap.LevelError, "CAPCP-02", path+"/eventCode", "an alert may describe only one CAP-CP subject event")
		}
		if index == 0 {
			baseline = values
			baselineNonFree = nonFreeValues(info)
		} else {
			if !reflect.DeepEqual(baseline, values) {
				report.Add(cap.LevelError, "CAPCP-02", path+"/eventCode", "all info blocks must describe the same CAP-CP subject event")
			}
			if !reflect.DeepEqual(baselineNonFree, nonFreeValues(info)) {
				report.Add(cap.LevelError, "CAPCP-06", path, "non-free-form values must be repeated verbatim across language info blocks")
			}
		}

		if len(info.Areas) == 0 {
			report.Add(cap.LevelError, "CAPCP-10", path+"/area", "at least one area block is required")
		}
		for areaIndex := range info.Areas {
			area := &info.Areas[areaIndex]
			areaPath := fmt.Sprintf("%s/area[%d]", path, areaIndex+1)
			if len(area.Geocodes) == 0 {
				report.Add(cap.LevelError, "CAPCP-10", areaPath+"/geocode", "at least one geocode is required")
			}
			hasCAPCPLocation := false
			for geocodeIndex, pair := range area.Geocodes {
				if !strings.HasPrefix(pair.ValueName, LocationNamePrefix) {
					continue
				}
				hasCAPCPLocation = true
				if len(validator.LocationCodes) != 0 {
					if !inFoldMap(validator.LocationCodes, pair.Value) {
						report.Add(cap.LevelError, "CAPCP-09-LIST", fmt.Sprintf("%s/geocode[%d]/value", areaPath, geocodeIndex+1), "geocode is not in the configured CAP-CP Location References list")
					}
				}
			}
			if !hasCAPCPLocation {
				report.Add(cap.LevelError, "CAPCP-09", areaPath+"/geocode", "a CAP-CP Location References geocode is required")
			}
			if !validator.DisableRecommendations && len(area.Polygons) == 0 && len(area.Circles) == 0 {
				report.Add(cap.LevelWarning, "CAPCP-18", areaPath, "polygon or circle geometry is recommended when available")
			}
		}

		autoTranslatedCount := len(cap.Values(info.Parameters, AutoTranslatedName))
		for parameterIndex, pair := range info.Parameters {
			parameterPath := fmt.Sprintf("%s/parameter[%d]", path, parameterIndex+1)
			switch pair.ValueName {
			case MinorChangeName:
				minorChangeUsed = true
				if !containsFold([]string{"none", "text", "correction", "resource", "layer", "other"}, pair.Value) {
					report.Add(cap.LevelError, "CAPCP-16", parameterPath+"/value", "unsupported MinorChange value")
				}
				if alert.MsgType != cap.MsgTypeUpdate || strings.TrimSpace(alert.References) == "" {
					report.Add(cap.LevelError, "CAPCP-16", parameterPath, "MinorChange may only be used on an Update with references")
				}
			case AutoTranslatedName:
				if !strings.EqualFold(pair.Value, "yes") && !strings.EqualFold(pair.Value, "no") {
					report.Add(cap.LevelError, "CAPCP-17", parameterPath+"/value", "AutoTranslated must be yes or no")
				}
			}
		}
		if autoTranslatedCount > 1 {
			report.Add(cap.LevelError, "CAPCP-17", path+"/parameter", "AutoTranslated may appear at most once in an info block")
		}

		if !validator.DisableRecommendations {
			if info.Expires == nil {
				report.Add(cap.LevelWarning, "CAPCP-13", path+"/expires", "expires is strongly recommended")
			}
			if strings.TrimSpace(info.SenderName) == "" {
				report.Add(cap.LevelWarning, "CAPCP-14", path+"/senderName", "senderName is strongly recommended")
			}
			if len(info.ResponseTypes) == 0 {
				report.Add(cap.LevelWarning, "CAPCP-15", path+"/responseType", "responseType is recommended when applicable")
			}
		}
	}
	if minorChangeUsed {
		for index, info := range alert.Info {
			if _, ok := cap.Value(info.Parameters, MinorChangeName); !ok {
				report.Add(cap.LevelError, "CAPCP-16", fmt.Sprintf("/alert/info[%d]/parameter", index+1), "MinorChange must appear in every info block when the update is marked non-substantive")
			}
		}
	}
	if strings.IndexFunc(alert.Sender, unicode.IsLetter) < 0 {
		report.Add(cap.LevelError, "CAPCP-11", "/alert/sender", "sender must be human-readable and identify the assembling agency")
	}
	return report
}

// ValidateActiveReferences checks CAP-CP rule 12 against candidate prior
// messages. A candidate is required when any info block has no expiry or has
// not expired as of the current message's sent time.
func ValidateActiveReferences(current *cap.Alert, candidates ...*cap.Alert) cap.Report {
	var report cap.Report
	if current == nil {
		report.Add(cap.LevelError, "CAPCP-12", "/alert", "current alert is nil")
		return report
	}
	if current.MsgType != cap.MsgTypeUpdate && current.MsgType != cap.MsgTypeCancel {
		return report
	}
	missing, err := cap.MissingActiveReferences(current, candidates...)
	if err != nil {
		report.Add(cap.LevelError, "CAPCP-12", "/alert/references", err.Error())
		return report
	}
	for _, reference := range missing {
		report.Add(cap.LevelError, "CAPCP-12", "/alert/references", "active message is not referenced: "+reference.String())
	}
	return report
}

func capCPEventCodes(pairs []cap.ValuePair) []cap.ValuePair {
	result := make([]cap.ValuePair, 0, 1)
	for _, pair := range pairs {
		if strings.HasPrefix(pair.ValueName, EventValueNamePrefix) {
			result = append(result, pair)
		}
	}
	return result
}

func nonFreeValues(info *cap.Info) []string {
	values := make([]string, 0, len(info.Categories)+len(info.ResponseTypes)+len(info.EventCodes)+3)
	for _, value := range info.Categories {
		values = append(values, "category="+string(value))
	}
	for _, value := range info.ResponseTypes {
		values = append(values, "response="+string(value))
	}
	values = append(values, "urgency="+string(info.Urgency), "severity="+string(info.Severity), "certainty="+string(info.Certainty))
	for _, pair := range info.EventCodes {
		values = append(values, "eventCode="+pair.ValueName+"="+pair.Value)
	}
	sort.Strings(values)
	return values
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func containsFold(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(value, target) {
			return true
		}
	}
	return false
}

func unique(values []string) []string {
	if len(values) < 2 {
		return values
	}
	result := values[:1]
	for _, value := range values[1:] {
		if value != result[len(result)-1] {
			result = append(result, value)
		}
	}
	return result
}

func inFoldMap(values map[string]struct{}, target string) bool {
	for value := range values {
		if strings.EqualFold(value, target) {
			return true
		}
	}
	return false
}
