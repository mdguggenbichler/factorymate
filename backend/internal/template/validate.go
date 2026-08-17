package template

import (
	"fmt"
	"strings"
)

const (
	maxTitleLen       = 256
	maxDescriptionLen = 4096
	maxFieldCount     = 25
	maxFieldNameLen   = 256
	maxFieldValueLen  = 1024
)

// ValidationError describes a template validation failure at save time (§5.4).
type ValidationError struct {
	Code    string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// Validate checks a template for unknown variables and Discord limits after rendering
// with sampleVars (§5.4, §5.4.1).
func Validate(tmpl Template, allowedVars []string, sampleVars map[string]string) error {
	allowed := make(map[string]struct{}, len(allowedVars))
	for _, v := range allowedVars {
		allowed[v] = struct{}{}
	}

	for _, name := range ExtractVariables(tmpl) {
		if _, ok := allowed[name]; !ok {
			return &ValidationError{
				Code:    "unknown_variable",
				Message: fmt.Sprintf("unknown template variable %q", name),
			}
		}
	}

	rendered := Render(tmpl, sampleVars)
	return validateRendered(rendered)
}

func validateRendered(msg RenderedMessage) error {
	if msg.Embed == nil {
		return nil
	}
	e := msg.Embed

	if len(e.Title) > maxTitleLen {
		return discordLimitError("title", len(e.Title), maxTitleLen)
	}
	if len(e.Description) > maxDescriptionLen {
		return discordLimitError("description", len(e.Description), maxDescriptionLen)
	}
	if len(e.Fields) > maxFieldCount {
		return &ValidationError{
			Code:    "discord_limit",
			Message: fmt.Sprintf("embed has %d fields, maximum is %d", len(e.Fields), maxFieldCount),
		}
	}
	for i, f := range e.Fields {
		if len(f.Name) > maxFieldNameLen {
			return discordLimitError(fmt.Sprintf("fields[%d].name", i), len(f.Name), maxFieldNameLen)
		}
		if len(f.Value) > maxFieldValueLen {
			return discordLimitError(fmt.Sprintf("fields[%d].value", i), len(f.Value), maxFieldValueLen)
		}
	}
	return nil
}

func discordLimitError(field string, got, max int) *ValidationError {
	return &ValidationError{
		Code:    "discord_limit",
		Message: fmt.Sprintf("%s length %d exceeds Discord limit of %d characters", field, got, max),
	}
}

// ValidateShape rejects templates with invalid embed structure.
func ValidateShape(tmpl Template) error {
	if tmpl.Plain == "" && tmpl.Embed == nil {
		return &ValidationError{
			Code:    "invalid_shape",
			Message: "template must include plain and/or embed variant",
		}
	}
	if tmpl.Embed != nil {
		if strings.TrimSpace(tmpl.Embed.Title) == "" &&
			strings.TrimSpace(tmpl.Embed.Description) == "" &&
			len(tmpl.Embed.Fields) == 0 {
			return &ValidationError{
				Code:    "invalid_shape",
				Message: "embed must include title, description, or at least one field",
			}
		}
	}
	return nil
}
