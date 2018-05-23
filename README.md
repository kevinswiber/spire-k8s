# Spire-K8s

This repo contains code and artifacts to integrate SPIRE and Kubernetes.

Integration goals include:

* Automatic injection of SPIRE sidecar containers in workloads deployed in the Kubernetes cluster
* Automatic mounting of a hostpath volume in sidecar container with a UDS where the workload API is exposed
* Automatic programming of entries in the SPIRE server for new workloads
* Establishing trust between SPIRE agent and SPIRE server using a Kubernetes-signed identity document

The design is being discussed in this [document](https://docs.google.com/document/d/14PFWpKHbXLxJwPn9NYYcUWGyO9d8HE1H_XAZ4Tz5K0E/edit)

# Content (as of 05/23/2018):

[src/spire-k8s/skbridge](src/spire-k8s/skbridge/) skbridge prototype

[src/spire-k8s/node-attestor/](src/spire-k8s/node-attestor/) node attestor prototype

[k8s-configs](k8s-configs) Kubernetes artifacts (webhook, csr roles, etc.)

[keys](keys) Pre-generated keys, certificates, etc. to ease deployment

[docs](docs) notes and instructions for each component
