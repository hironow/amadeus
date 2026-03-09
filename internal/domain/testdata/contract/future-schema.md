---
dmail-schema-version: "2"
name: test-future-schema
kind: design-feedback
description: A D-Mail with schema version 2
---

This D-Mail claims schema version 2 which does not exist yet.
A Postel-liberal parser should still parse the frontmatter fields
it recognizes, ignoring the version mismatch.
