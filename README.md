# Plex Sandbox Operator

[![Go Report Card](https://goreportcard.com/badge/github.com/plexsystems/sandbox-operator)](https://goreportcard.com/report/github.com/plexsystems/sandbox-operator)
[![GitHub release](https://img.shields.io/github/release/plexsystems/sandbox-operator.svg)](https://github.com/plexsystems/sandbox-operator/releases)

![sandbox-operator](img/sandbox-operator.png)

## Introduction

The Plex Sandbox Operator is an operator for [Kubernetes](https://kubernetes.io/) that enables authenticated users to create their own isolated environments.

## Installation

### Kustomize

This repository contains a [deploy](deploy) folder which contains all of the manifests required to deploy the operator, as well as a `kustomization.yaml` file.

If you would like to apply your own customizations, reference the `deploy` folder and the version in your `kustomization.yaml`.

```yaml
resources:
- git::https://github.com/plexsystems/sandbox-operator.git//deploy?ref=v0.7.0
```

The [example](example) folder shows one example of how to customize the operator.

### Bundle

A [bundle.yaml](bundle.yaml) is provided in the root of the repository which can then be applied via `kubectl apply`.

### Created ClusterRole and ClusterRoleBinding

A `ClusterRole` resource and a `ClusterRoleBinding` resource will be created to enable authenticated users to create Sandbox resources.

#### ClusterRole (sandbox-users)

|Verbs|API Groups|Resources|
|---|---|---|
|create, list, get|operators.plex.dev|sandboxes|

#### ClusterRoleBinding (sandbox-user)

|API Group|Name|Subjects|
|---|---|---|
|rbac.authorization.k8s.io|sandbox-users|system:authenticated|

### Sandbox CRD

A `CustomResourceDefinition` named `Sandbox` will be created.

An example manifest for the Sandbox CRD is as follows:

```yaml
apiVersion: operators.plex.dev/v1alpha1
kind: Sandbox
metadata:
  name: test
spec:
  size: small
  owners:
  - foo@bar.com
```

## Configuration

### Clients

The Sandbox operator can leverage different clients, depending upon how authenitcation is configured for your cluster.

#### Azure

If Azure credentials are provided to the operators environment, it will perform a lookup of each user in the `owners` field and fetch that users `ObjectID` inside of Azure using the [Microsoft Graph API](https://docs.microsoft.com/en-us/graph/api/resources/azure-ad-overview?view=graph-rest-1.0).

This enables users to create Sandboxes with friendly names in the `owners` field (such as the owners email address) and have the operator itself handle the mapping to the `ObjectID` when creating the Kubernetes resources.

To use the Azure client, include the following environment variables:

- `AZURE_CLIENT_ID`
- `AZURE_TENANT_ID`
- `AZURE_CLIENT_SECRET`

Your Azure Service Principal will need the following _Application_ permission for the **Azure Active Directory Graph API** (00000002-0000-0000-c000-000000000000):

Directory.Read.All (5778995a-e1bf-45b8-affa-663a9f3f4d04)

#### Default

If no credentials are provided, the operator will create the `Role` and `ClusterRole` bindings using the values listed in the `owners` field.

### Docker Pull Secrets

By default, the operator will not create any secrets in the provisioned namespace. 

**If the `PULL_SECRET_NAME` environment variable is set, the operator will copy your clusters pull secret to the provisioned namespace and patch the default service account.**

`PULL_SECRET_NAME` should be the name of the pull secret that exists in your cluster. By default, the operator will look for your secret in the `default` namespace.

To have the operator look in a different namespace for the pull secret, use the `PULL_SECRET_NAMESPACE` environment variable.


## Creating a Sandbox

To create a Sandbox, apply a Sandbox CRD to the target cluster.

The following will create a Sandbox called `foo` (the resulting namespace being `sandbox-foo`), and assign the RBAC policies to user `foo@bar.com`.

### foo.yaml

```yaml
apiVersion: operators.plex.dev/v1alpha1
kind: Sandbox
metadata:
  name: foo
spec:
  size: small
  owners:
  - foo@bar.com
```

```console
$ kubectl apply -f foo.yaml
sandboxes.operators.plex.dev "foo" created
```

## Created Resources

Assuming the name of the created Sandbox is named `foo`, the following resources will be created per Sandbox:

### Namespace (sandbox-foo)

### ClusterRole (sandbox-foo-deleter)

|Verbs|API Groups|Resources|ResourceNames|
|---|---|---|---|
|delete|operators.plex.dev|sandboxes|sandbox-foo|

This is created so that only users defined in the `owners` field can delete their Sandboxes.

### ClusterRoleBinding (sandbox-foo-deleters)

One `ClusterRoleBinding` per name in the `owners` field. Bindings are added and removed as users are added and removed from the `owners` field.

### Role (sandbox-foo-owner)

|Verbs|API Groups|Resources|
|---|---|---|
|*|core|pods, pods/log, pods/portforward, services, services/finalizers, endpoints, persistentvolumeclaims, events, configmaps, replicationcontrollers|
|create|core|secrets|
|*|apps, extensions|deployments, daemonsets, replicasets, statefulsets|
|*|autoscaling|horizontalpodautoscalers|
|*|batch|jobs, cronjobs|
|create, list, get|rbac.authorization.k8s.io|roles, rolebindings|

### RoleBinding (sandbox-foo-owners)

One `RoleBinding` per name in the `owners` field. Bindings are added and removed as users are added and removed from the `owners` field.

### ResourceQuota (sandbox-foo-resourcequota)

The `ResourceQuota` that is applied to the `Namespace` depends on the `size` of the `Sandbox` that was created.

#### Small

|Resource Name|Quantity|
|---|---|
|ResourceRequestsCPU|0.25|
|ResourceLimitsCPU|0.5|
|ResourceRequestsMemory|250Mi|
|ResourceLimitsMemory|500Mi|
|ResourceRequestsStorage|10Gi|
|ResourcePersistentVolumeClaims|2|

#### Large

|Resource Name|Quantity|
|---|---|
|ResourceRequestsCPU|1|
|ResourceLimitsCPU|2|
|ResourceRequestsMemory|2Gi|
|ResourceLimitsMemory|8Gi|
|ResourceRequestsStorage|40Gi|
|ResourcePersistentVolumeClaims|8|

```text
NOTE: If no size is given, small is the default.
```

## Managing Owners of a Sandbox

After the Sandbox has been created, you can add or remove owners that are associated to it.

For example, to add `more@bar.com` as an owner, add their name to the list of owners and apply the changes:

```yaml
apiVersion: operators.plex.dev/v1alpha1
kind: Sandbox
metadata:
  name: foo
spec:
  size: small
  owners:
  - foo@bar.com
  - more@bar.com
```

```console
$ kubectl apply -f sandbox-foo.yaml
sandboxes.operators.plex.dev "foo" configured
```

This will cause the operator to add `ClusterRoleBinding` and `RoleBinding` resources to match the owners list.

When `owners` are removed from the Sandbox, their `ClusterRoleBinding` and `RoleBinding` will also be removed.

## Deleting a Sandbox

To delete a Sandbox, delete the Sandbox resource from the cluster:

```console
$ kubectl delete sandbox foo
sandboxes.operators.plex.dev "foo" deleted
```

Deleting a Sandbox will delete the `Namespace` as well as the `ClusterRole` and `ClusterRoleBinding` resources.

## Metrics

The operator exposes two metric ports for the `/metrics` endpoint:

- Port `8383` exposes metrics for the operator itself
- Port `8686` exposes metrics for the `Sandbox` CRD

Additionally, if [prometheus-operator](https://github.com/coreos/prometheus-operator) is installed into the cluster, a `ServiceMonitor` is created for the operator.

## Development

No external tooling is required to develop and build the operator. However, some tooling is required to run the integration tests:

- [Kind](https://github.com/kubernetes-sigs/kind)
- [Kustomize](https://github.com/kubernetes-sigs/kustomize)

## Testing

The provided `Makefile` contains commands that assist with running the tests for the operator.

### Unit tests

`make test-unit` will use an in-memory kubernetes client to validate and test your changes without the need for an external Kubernetes cluster.

### Integration tests

`make test-integration` will create a Kubernetes cluster for you, using Kind, and deploy the operator to it. The integration tests will then be ran against the newly created cluster.

#### Testing different Kubernetes versions

To test the operator with different versions of Kubernetes, you can use the `KUBERNETES_VERSION` variable when calling `make`.

For example, to test on Kubernetes v1.16.3, run the following command:

`make test-integration KUBERNETES_VERSION=v1.16.3`

## Contributing

We :heart: pull requests. If you have a question, feedback, or would like to contribute — please feel free to create an issue or open a pull request!
