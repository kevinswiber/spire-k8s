# SKBridge

## Introduction

`skbridge` is the component that implements the Kubernetes webhook network endpoint and performs the following tasks:

* inject sidecar proxy container in the pod spec
* inject hostpath volume where SPIRE Agent exposes the UDS for the Workload API
* program workload entries in SPIRE server for new workloads

## Notes

*    Kubernetes will only connect to an external webook over TLS.
*    The Webhook configuration includes information on how to reach the endpoint as well
     as a CA certificate used to validate the certificate presented by the webhook process.
*    The endpoint can be an hostname/IP:port, pair or a Kubernetes service name, for the case
     in which `skbridge` itself runs on Kubernetes as deployment, daemonset, ...
*    The Webhook certificate must include the correct hostname/IP in the SAN extension.
*    The CA certificate must be give to Kubernetes in base64 encoding.


## Tutorial

These are the steps to bring up an instance of `skbridge` and have a Kubernetes cluster connect to it.
They should work for any cluster, but they have been tested with Minikube v0.26.
`#` indicates a shell prompt.

1. Build SKBridge as a normal go program

    `#export GOPATH=...`

    `#cd spire-k8s/src/spire-k8s/skbridge`

    `#go get ./...`

    `#go build .`

2. Generate a certificate for skbridge

  Under spire-k8s/keys/skbridge there is a full set of keys that can be used to establish TLS between
  Kubernetes ApiServer and SkBridge.

  However, you may need to regenerate the SKBridge certificate
  to include the correct hostname/IP address:

        #cd spire-k8s/keys/skbridge

  edit `v3ext.cnf` and put the correct IP address in `IP.1` field

  run `./signcsr` file `skbridge.crt` should be regenerated

3. Configure the webhook on Kubernetes

  Assuming your kubectl is already configured to point to the cluster with appropriate credentials:

          #cd spire-k8s/k8s-config

  edit `webhook.yml` and put correct IP:port in `url` field `

  the caBundle field already includes the base64 encoding of the CA cert `spire-k8s/keys/skbridge/skbridge.ca.rt`
  so you should not have to change it

  create the  resource

          #kubectl create -f webhook.yml

4. Start SKBridge

  From `spire-k8s` directory, run `./src/spire-k8s/skbridge/skbridge --wh-cert keys/skbridge/skbridge.crt --wh-key keys/skbridge/skbridge.key --sidecar-image nginx:latest --host-mount /tmp`.

  Replace nginx:latest with the image of the sidecar you want to run.

5. Deploy a test container to verify that everything works

  Start a sample pod with a single container:

        #kubectl run kuard --image=gcr.io/kuar-demo/kuard-amd64:1

  Check that it is running:

        #kubectl get pods

        NAME                    READY     STATUS    RESTARTS   AGE
        kuard-b75468d67-v2ggg   2/2       Running   0          6s

  Any webhook-related error should cause the creation of the pod to fail.

  If you don't see any pod running, check Kubernetes ApiServer logs with:

        kubectl logs kube-apiserver-minikube --namespace kube-system`

  Inspecting the pod should show the injected sidecar container and volume mount

        #kubectl get pod kuard-b75468d67-v2ggg -oyaml

        ...
        spec:
          containers:
          ...
          - image: nginx:latest
            imagePullPolicy: Always
            name: spire-sidecar
            resources: {}
            terminationMessagePath: /dev/termination-log
            terminationMessagePolicy: File
            volumeMounts:
            - mountPath: /spire
              name: spire-wl-api
          volumes:
          ...
          - hostPath:
              path: /tmp
              type: Directory
            name: spire-wl-api
        ...

