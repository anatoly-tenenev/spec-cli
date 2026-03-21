---
type: feature
id: FEAT-1
slug: retry-window
created_date: 2026-03-05
updated_date: 2026-03-11
status: active
reviewers:
  - SVC-2
  - SVC-1
score: 9.5
tags:
  - reliability
  - billing
---

## Summary {#summary}
Retry window for outbound requests.

## Implementation {#implementation}
Retry window uses backoff for transient failures.
