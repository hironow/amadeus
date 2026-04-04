// Package filter defines LLM action spaces: prompt templates,
// response schemas, and variable specifications.
//
// The PromptRegistry loads YAML prompt files from the embedded prompts/
// directory and provides Get/Expand methods for simple {key} substitution.
// This externalizes prompt strings from Go source, enabling versioning
// and auditing without code changes.
package filter
