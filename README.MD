kubernetes-auto-ingress
=======================

[![Build Status](https://travis-ci.org/hxquangnhat/kubernetes-auto-ingress.svg?branch=master)](https://travis-ci.org/hxquangnhat/kubernetes-auto-ingress)
[![Go Report Card](https://goreportcard.com/badge/github.com/hxquangnhat/kubernetes-auto-ingress)](https://goreportcard.com/report/github.com/hxquangnhat/kubernetes-auto-ingress)

Description
-----------

Dynamically create an ingress for a service which specifies "autoingress: true" label, for associated namespace of the service.

Use `TLS/SSL` and backend is `/` by default.

Works with [nginx-ingress-controller](https://github.com/kubernetes/ingress/tree/master/controllers/nginx).

How to use it
-------------

Require nginx-ingress-controller running.

Below describes how to run auto-ingress with your `secret` for domain `example.com`

```
openssl req -x509 -nodes -days 365 -newkey rsa:2048 -keyout tls.key -out tls.crt -subj "/CN=*.example.com"
kubectl create secret tls secret --key tls.key --cert tls.crt
kubectl create -f hack/deployment.yaml
```

If you want to use it outside of the cluster, provide the `kubeconfig` with config file:

`./kubernetes-auto-ingress -kubeconfig=./config`

Then, for every services you want to be ingressed need to put label **auto-ingress/enabled: enabled**.

Service which is already ingressed can be un-ingressed by setting **auto-ingress/enabled: disabled**.

```yaml
apiVersion: v1
kind: Service
metadata:
  name: myapp-service
  labels:
    app: myapp
    auto-ingress/enabled: 'enabled'
spec:
  type: LoadBalancer
  ports:
  - name: http
    port: 80
    targetPort: http
  selector:
    app: myapp
```

The ingress for this `myapp-service` will have the same name `myapp-service`. The servername to access this service would be `myapp-service.example.com`.

How to build it
---------------

Clone the repository and build the container

```
$ go get github.com/hxquangnhat/kubernetes-auto-ingress
$ cd $GOPATH/github.com/hxquangnhat/kubernetes-auto-ingress
$ make container
```
