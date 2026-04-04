package testdata

// Harness layer rules use paths.include for enforcement.
// They cannot be tested via ruleid annotations in a single test file
// because the rules match on file paths (e.g., /internal/domain/**).
// Verification is done by running `semgrep scan --config .semgrep/ --error`
// against the actual codebase.
