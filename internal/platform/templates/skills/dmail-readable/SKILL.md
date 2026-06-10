---
name: dmail-readable
description: Declares inbound D-Mail kinds for phonewave routing discovery.
license: Apache-2.0
metadata:
  dmail-schema-version: "1"
  consumes:
    - kind: report
      description: expedition / judgment reports feeding the review queue
    - kind: stall-escalation
      description: stalled-loop escalations from the designer
---

D-Mail read capability for amadeus.
