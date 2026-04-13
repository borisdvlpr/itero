# itero

itero is a self-hosted, Kubernetes-native feature flag management backend. It provides a REST API for managing flags, variants, targeting rules, and environments, implementing the OpenFeature Remote Evaluation Protocol (OFREP), enabling any application using the OpenFeature SDK to point to itero as its backend, regardless of programming language.

## What are feature flags?

Feature flags let you control whether a feature is active at runtime without deploying new code. You wrap behaviour in a conditional that checks a flag's value, and control that value externally.
