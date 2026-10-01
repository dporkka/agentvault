# Dev Plane repository contract

`devplane.yaml` is the canonical repository-owned verification contract for autonomous development and isolated CI execution.

AgentVault keeps build/test policy in repository-native commands (`make lint`, `make test-ci`, `make contract-check`, and `make ci`) while Dev Plane owns orchestration, isolation, exact-head verification, and evidence.

The contract intentionally keeps AgentVault outside the critical execution authority path: AgentVault provides memory/context/provenance, while Dev Plane and its runtime providers own workspace execution and verification.
