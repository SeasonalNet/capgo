package cap

import (
	"fmt"
	"strings"
)

// Level is the conformance impact of a validation issue.
type Level string

const (
	LevelError   Level = "error"
	LevelWarning Level = "warning"
)

// Issue is one actionable conformance finding.
type Issue struct {
	Level   Level
	Rule    string
	Path    string
	Message string
}

func (i Issue) String() string {
	return fmt.Sprintf("%s %s at %s: %s", i.Level, i.Rule, i.Path, i.Message)
}

// Report is a deterministic list of validation issues.
type Report []Issue

// Add appends an issue.
func (r *Report) Add(level Level, rule, path, message string) {
	*r = append(*r, Issue{Level: level, Rule: rule, Path: path, Message: message})
}

// Append adds all issues from other.
func (r *Report) Append(other Report) { *r = append(*r, other...) }

// Valid reports whether the report has no error-level issues. Warnings do not
// make a message invalid.
func (r Report) Valid() bool { return !r.HasErrors() }

// HasErrors reports whether at least one error-level issue exists.
func (r Report) HasErrors() bool {
	for _, issue := range r {
		if issue.Level == LevelError {
			return true
		}
	}
	return false
}

// Errors returns only error-level issues.
func (r Report) Errors() Report { return r.withLevel(LevelError) }

// Warnings returns only warning-level issues.
func (r Report) Warnings() Report { return r.withLevel(LevelWarning) }

func (r Report) withLevel(level Level) Report {
	filtered := make(Report, 0, len(r))
	for _, issue := range r {
		if issue.Level == level {
			filtered = append(filtered, issue)
		}
	}
	return filtered
}

// Error summarizes the report for use as an error value.
func (r Report) Error() string {
	if len(r) == 0 {
		return ""
	}
	parts := make([]string, len(r))
	for index, issue := range r {
		parts[index] = issue.String()
	}
	return strings.Join(parts, "; ")
}

// Validator is implemented by CAP profile validators.
type Validator interface {
	Name() string
	Validate(*Alert) Report
}

// ValidateWith runs base CAP validation followed by each profile validator.
func ValidateWith(alert *Alert, validators ...Validator) Report {
	report := Validate(alert)
	for _, validator := range validators {
		if validator != nil {
			report.Append(validator.Validate(alert))
		}
	}
	return report
}
