package approvals

import (
	"fmt"
)

// NotificationTemplate represents a notification template
type NotificationTemplate struct {
	ID        string                `json:"id"`
	Name      string                `json:"name"`
	Type      string                `json:"type"`
	Subject   string                `json:"subject"`
	Body      string                `json:"body"`
	Variables []string              `json:"variables"`
	Channels  []NotificationChannel `json:"channels"`
}

// TemplateRenderer renders notification templates
type TemplateRenderer struct {
	templates map[string]*NotificationTemplate
}

// NewTemplateRenderer creates a new template renderer
func NewTemplateRenderer() *TemplateRenderer {
	return &TemplateRenderer{
		templates: make(map[string]*NotificationTemplate),
	}
}

// AddTemplate adds a notification template
func (r *TemplateRenderer) AddTemplate(template *NotificationTemplate) {
	r.templates[template.ID] = template
}

// Render renders a notification from a template
func (r *TemplateRenderer) Render(templateID string, variables map[string]string) (*NotificationEvent, error) {
	template, exists := r.templates[templateID]
	if !exists {
		return nil, fmt.Errorf("template not found: %s", templateID)
	}

	// Simple variable substitution
	subject := template.Subject
	body := template.Body

	for key, value := range variables {
		placeholder := fmt.Sprintf("{{%s}}", key)
		subject = replaceAll(subject, placeholder, value)
		body = replaceAll(body, placeholder, value)
	}

	return &NotificationEvent{
		Type:    template.Type,
		Title:   subject,
		Message: body,
	}, nil
}

// Helper function to replace all occurrences
func replaceAll(s, old, new string) string {
	result := ""
	for {
		idx := findSubstring(s, old)
		if idx == -1 {
			return result + s
		}
		result += s[:idx] + new
		s = s[idx+len(old):]
	}
}

func findSubstring(s, substr string) int {
	if len(substr) > len(s) {
		return -1
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
