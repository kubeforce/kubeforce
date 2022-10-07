# Developer Guide

This doc provides instructions about how to test Kubeforce Provider on a local workstation.

## Initial setup for development environment

### Install prerequisites

- Install [go](https://golang.org/doc/install)
- Install [kubectl](https://kubernetes.io/docs/tasks/tools/)
- Install [Kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation)
- Install [Docker](https://docs.docker.com/engine/install/)
- Install [Tilt](https://docs.tilt.dev/install.html)
- Install make

### Get the source

```bash
git clone git@github.com:kubernetes-sigs/cluster-api.git
git clone git@github.com:kubeforce/kubeforce.git
```

### Create a tilt-settings.json file

Create a tilt-settings.json file and place it in your copy of cluster-api:

```bash
cd cluster-api/
cat > tilt-settings.json <<EOF
{
  "default_registry": "",
  "enable_providers": ["docker", "kubeadm-bootstrap", "kubeadm-control-plane", "kubeforce"],
  "provider_repos": ["../kubeforce"],
  "deploy_cert_manager": true,
  "kustomize_substitutions": {
    "EXP_CLUSTER_RESOURCE_SET": "true",
    "EXP_MACHINE_POOL": "true"
  }
}
EOF
```

### Create the management cluster
The Cluster API uses Kind to create a management cluster.

```bash
cd cluster-api/
make kind-cluster
```

### Run Tilt

To start the development environment, run Tilt and continue keep it running in the shell.
Wait for all resources to appear, the status can be checked in the Tilt UI.
```bash
cd cluster-api/
tilt up
```

### Deploy agent repository on the management cluster

```bash
cd kubeforce/agent/
make repository-deploy
```

## Creating a workload cluster

Now that you have your management cluster ready with the Cluster API and Kubeforce provider installed, we can start creating the workload cluster.

### Create docker hosts
```bash
cd kubeforce/test/docker-compose
docker-compose up
```

### Create kubernetes cluster
When the hosts are ready in docker, you can start creating a kubernetes cluster. To do this, you need to create resources in the management cluster.
```bash
cd kubeforce/test/docker-compose
kubectl apply -f kf-cluster.yaml
```

### Wait for the cluster to be ready
You need to wait for the kubernetes cluster to be created. The following commands will help you check:
```bash
kubectl get kfa
kubectl get kfm
kubectl get ma
kubectl get pb
```
