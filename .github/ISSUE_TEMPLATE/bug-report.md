---
name: Bug Report
about: Report a problem to help improve itero
title: "[Bug]: "
labels: bug
assignees: ''
---

## Description

A clear and concise description of what the bug is.

## Steps to Reproduce

1. Deploy itero via '...' <!-- e.g. Helm chart, raw manifests, Docker Compose, binary -->
2. Configure flag/environment/targeting rule '...'
3. Call '...' <!-- REST API endpoint or OFREP evaluation request -->
4. See error

## Expected Behavior

What you expected to happen.

## Actual Behavior

What actually happened. Include any error output, logs, or OFREP response payloads
(please redact any secrets, API keys, or connection strings).

## Environment

- itero version: <!-- image tag or release version -->
- Deployment method: <!-- Helm chart, raw K8s manifests, Docker Compose -->
- Kubernetes version: <!-- if applicable -->
- Postgres version:
- OpenFeature SDK/language: <!-- if the bug is evaluation-related, e.g. openfeature-go v1.x -->

## Additional Context

Anything else that might help — your Helm values, flag/targeting rule configuration,
ingress or network setup, or relevant OFREP request/response payloads.
