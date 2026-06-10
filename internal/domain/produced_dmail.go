package domain

import "fmt"

// ProducesKinds is the producer subset of D-Mail kinds amadeus is
// allowed to emit, mirroring the dmail-sendable SKILL.md manifest
// (refs issue 0031): the verifier produces corrective feedback and
// convergence alerts.
var ProducesKinds = map[DMailKind]bool{
	KindDesignFeedback: true,
	KindImplFeedback:   true,
	KindConvergence:    true,
}

// NewProducedDMail builds an always-valid D-Mail for emission through
// the transactional outbox (Parse-Don't-Validate): kind must be in the
// producer subset, and the schema-v1 required fields must be present.
func NewProducedDMail(kind DMailKind, name, description, body string, issues []string, severity Severity, priority int, metadata map[string]string) (DMail, error) { // nosemgrep: domain-primitives.multiple-string-params-go -- name/description/body are distinct D-Mail schema fields [permanent]
	if !ProducesKinds[kind] {
		return DMail{}, fmt.Errorf("amadeus does not produce kind %q (produces: design-feedback / implementation-feedback / convergence per the dmail-sendable manifest)", kind)
	}
	if name == "" {
		return DMail{}, fmt.Errorf("dmail: name is required")
	}
	if description == "" {
		return DMail{}, fmt.Errorf("dmail: description is required")
	}
	if body == "" {
		return DMail{}, fmt.Errorf("dmail: body is required")
	}
	return DMail{
		SchemaVersion: DMailSchemaVersion,
		Name:          name,
		Kind:          kind,
		Description:   description,
		Body:          body,
		Issues:        issues,
		Severity:      severity,
		Priority:      priority,
		Metadata:      metadata,
	}, nil
}
