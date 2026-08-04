# itero

itero is a self-hosted, Kubernetes-native feature flag management backend. It provides a REST API for managing flags, variants, targeting rules, and environments, implementing the [OpenFeature Remote Evaluation Protocol (OFREP)](https://github.com/open-feature/protocol), enabling any application using the OpenFeature SDK to point to itero as its backend, regardless of programming language.

itero is built with Go and Chi, backed by PostgreSQL, and designed to run as a first-class citizen inside a Kubernetes cluster.

## What are feature flags?

Feature flags (also called feature toggles) are a software delivery mechanism that lets you control whether a feature is active at runtime — without deploying new code. Instead of shipping code that's immediately live, you wrap it in a conditional that checks a flag's value, and control that value externally.

There are four main categories:

- **Release flags** — hide an unfinished feature in production. When it's ready, flip the flag on. No deployment needed.
- **Experiment flags** — A/B testing. Show variant A to 50% of users, variant B to the others. Measure outcomes.
- **Kill switch flags** — an emergency lever. If a new database query is hammering production, flip a flag to short-circuit it without a rollback.
- **Permission flags** — grant early access to specific user groups, tenants, or roles (beta users, enterprise customers).

In a Kubernetes context, this becomes even more powerful: you can control behaviour per-cluster, per-namespace, per-deployment, or per-pod — without touching manifests or triggering rollouts.
