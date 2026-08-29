// Package validation defines the typed vocabulary for reporting a problem
// found in a document: the issue itself, its level, and the entity it belongs
// to. Producers across commands build these so that one issue shape reaches
// the response regardless of which check found it.
package validation

type IssueLevel string

const (
	LevelError   IssueLevel = "error"
	LevelWarning IssueLevel = "warning"
)

type Entity struct {
	Type string `json:"type,omitempty"`
	ID   string `json:"id,omitempty"`
	Slug string `json:"slug,omitempty"`
}

type Issue struct {
	Code        string     `json:"code,omitempty"`
	Level       IssueLevel `json:"level,omitempty"`
	Class       string     `json:"class"`
	Message     string     `json:"message"`
	StandardRef string     `json:"standard_ref"`
	Entity      *Entity    `json:"entity,omitempty"`
	Field       string     `json:"field,omitempty"`
}
