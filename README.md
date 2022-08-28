# Kubernetes Cluster API Provider Kubeforce

---

## What is Cluster API Provider Kubeforce

[Cluster API](https://github.com/kubernetes-sigs/cluster-api) brings
declarative, Kubernetes-style APIs to cluster creation, configuration and
management.

__Kubeforce__ is a Cluster API Infrastructure Provider for pre-provisioned Linux hosts.
This provider helps install all the necessary Kubernetes components (kubelet, kubeadm, containerd, etc.) on the host and configure it.


## Features

- Native Kubernetes manifests and API
- Support for single and multi-node control plane clusters
- Support for pre-provisioned Linux hosts
- Support for processor architectures: amd64, arm64, arm

## Getting Started
A getting started guide will be available soon.

## Community, discussion, contribution, and support

The Kubeforce provider is developed in the open, and is constantly being improved by our users, contributors, and maintainers.

Pull Requests and feedback on issues are very welcome!
See the [issue tracker](https://github.com/kubeforce/kubeforce/issues).

See also our [contributor guide](CONTRIBUTING.md) and the Kubernetes [community page](https://kubernetes.io/community) for more details on how to get involved.

## Project Status

This project is currently a work-in-progress, in an Alpha state, so it may not be production ready. There is no backwards-compatibility guarantee at this point. For more details on the roadmap and upcoming features, check out [the project's issue tracker on GitHub][issue].


## Getting involved and contributing

### Launching a Kubernetes cluster using Kubeforce source code

Check out the developer guide for launching a Kubeforce cluster consisting of Docker containers as hosts.

More about development and contributing practices can be found in [`CONTRIBUTING.md`](./CONTRIBUTING.md).

------

## Compatibility with Cluster API

- Kubeforce is currently compatible wth Cluster API (v1.2.x)
