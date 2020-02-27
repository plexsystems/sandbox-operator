# sandbox-operator

## Introduction

This is a sandbox operator that creates segregated namespaces and sets up RBAC for authenticated users specified in the CRD.

## Local Testing

Run `make test-unit` to run the operator unit tests

Run `make test-integration` to deploy the operator to a Kind cluster and verify the operator pod enters a running state.

Iterative deployments can be made with `make deploy`. This will rebuild the operator and deploy to it to an existing cluster.

To test with a different version of Kubernetes, pass in `KUBERNETES_VERSION` to the `make` command (e.g. `make test-integration KUBERNETES_VERSION=v1.17.0`)
