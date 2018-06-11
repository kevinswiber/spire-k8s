# Node-attestor

## Introduction

`node-attestor` is an executable that exercises the Kubernetes APIs to fetch
a certificate signed by the Kubernetes CA. This certificate can be used to
establish trust with the SPIRE server, similar to the way an Instance
Identity Document is used in AWS deployments.

The code eventually will be integrated into a SPIRE agent plugin. A corresponding
SPIRE server plugin will be developed to perform validation of the certificates
and check possession of the private key.

This attestation flow takes advantage of the mechanisms that Kubelet uses to
establish trust with Kubernetes ApiServer, so SPIRE should be able to take
advantage of Kubernetes work in that area. See the
[SPIRE-K8s document](https://docs.google.com/document/d/14PFWpKHbXLxJwPn9NYYcUWGyO9d8HE1H_XAZ4Tz5K0E/edit)
for pointers to relevant documents, issues, etc.

## Tutorial

These are the steps required to have `node-attestor` submit a CSR and Kubernetes
ApiServer auto-approve it. The Kubernetes steps need to be performed only once.
After that, `node-attestor` can be run as many times as desired.
`#` indicates a shell prompt.

1. Create a `spire-agent` user

    The simplest way to do this is to sign `keys/node-attestor/spire-agent.csr`
    using the Kubernetes CA private key. If you are using minikube, this should do:

        #cd keys/node-attestor
        #./signcsr.sh

    otherwise locate the Kubernetes CA private key (there is no standard location
    for it, try for example `/etc/kubernetes/pki`) and replace `~/.minikube` in
    `signcsr.sh`

2. Create cluster roles and role-bindings for user `spire-agent`

        #cd k8s-configs
        #kubectl create -f csr-create-role.yml
        #kubectl create -f csr-create-rolebinding.yml
        #kubectl create -f csr-autoapprove-rolebinding.yml

    `csr-create-role.yml` and `csr-create-rolebinding.yml` enable user `spire-agent`
    to submit CSRs, while `csr-autoapprove-rolebinding.yml` informs `kube-controller`
    that they should be auto-approved.

    If auto-approval is not desired, user can skip creating
    `csr-autoapprove-rolebinding.yml` and use the command `kubectl certificate approve`
    to manually approve each CSR.

    User `spire-agent` does not have access to any Kubernetes resource besides CSRs.

3. Build and run `node-attestor`

        #go install ./src/spire-k8s/node-attestor/...

        #./bin/node-attestor -ca-cert ~/.minikube/apiserver.crt -client-cert keys/node-attestor/spire-agent.crt -client-key keys/node-attestor/spire-agent.key

    `ca-cert` flags specify the CA certificate that `node-attestor` uses to
    authenticate the Kubernetes API server. `client-cert` and `client-key`
    flags specify the key and certificate that `node-attestor` uses to authenticate
    itself to the Kubernetes ApiServer

    The private key and the certificate are written (by default) under `/tmp/spire-agent-id/`.

    The host name is used (by default) as identity in the certificate.

    To check out the certificate, run:

        #openssl x509 -in /tmp/spire-agent-id/id-doc.crt -text -noout


