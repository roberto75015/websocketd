---
title: "Deploy to Kubernetes"
weight: 60
description: "A Deployment, a Service, and a WebSocket-aware Ingress for websocketd, with the probe that actually works."
---

Three objects: a Deployment running your websocketd image, a Service in front
of it, and an Ingress that keeps long-lived upgrades open. Build the image
first, following [run in a container](/how-to/deploy/docker/).

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: websocketd
spec:
  replicas: 2
  selector:
    matchLabels:
      app: websocketd
  template:
    metadata:
      labels:
        app: websocketd
    spec:
      containers:
        - name: websocketd
          image: myregistry/myapp-ws:0.5.0
          args:
            - "--port=8080"
            - "--address=0.0.0.0"
            - "--origin=https://ws.example.com"
            - "/app/myscript.sh"
          ports:
            - name: ws
              containerPort: 8080
          readinessProbe:
            tcpSocket:
              port: ws
            initialDelaySeconds: 2
            periodSeconds: 10
---
apiVersion: v1
kind: Service
metadata:
  name: websocketd
spec:
  selector:
    app: websocketd
  ports:
    - port: 80
      targetPort: ws
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: websocketd
  annotations:
    nginx.ingress.kubernetes.io/proxy-read-timeout: "3600"
    nginx.ingress.kubernetes.io/proxy-send-timeout: "3600"
spec:
  ingressClassName: nginx
  rules:
    - host: ws.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: websocketd
                port:
                  number: 80
```

Apply it, and `wss://ws.example.com/` reaches your script.

## The Ingress annotations

The `ingress-nginx` controller proxies WebSocket upgrades without any
opt-in annotation. What it does not do is guess how long you want the
connection held open: its `proxy_read_timeout` and `proxy_send_timeout`
default to 60 seconds, and they are idle timers, so an open but quiet
WebSocket connection is cut at the minute mark. The two annotations above
raise them.

You can instead keep the connection from ever going idle, by adding
`--pingms=30000` to the container `args`. websocketd then sends a ping frame
every 30 seconds, which resets the controller's read timer. Doing both is
reasonable.

The annotation keys above are specific to `ingress-nginx`. Traefik, HAProxy
Ingress and the cloud controllers all proxy upgrades too, but each spells its
timeouts differently. Check the controller you run.

## What a readiness probe can and cannot tell you

Use a `tcpSocket` probe, as above. It answers exactly one question: is
websocketd listening? That is the right question, because websocketd
validates its whole configuration before it binds. A pod that accepts a TCP
connection has a websocketd whose flags were accepted.

An `httpGet` probe does not work. websocketd answers a plain, non-upgrade
`GET` on an endpoint path with `404 Not Found`, and Kubernetes counts any
status outside 200 to 399 as a failure, so the pod never becomes ready.

Neither probe tells you your script works. websocketd does not run your
program until a client completes a WebSocket upgrade, so nothing has executed
it at probe time. A broken interpreter, a missing data file or a script that
exits immediately all pass readiness and fail on first connection. If you
need that covered, add a `--staticdir` health file and probe it, or run a
client-side check outside the cluster.

## Replicas and shared state

Scaling out is safe as far as websocketd is concerned. It keeps nothing
between connections, and each connection gets its own process wherever it
lands. See [the process model](/understanding/process-model/).

It is your wrapped program that decides whether replicas are safe. If two
connections must see each other's state, they now have to do so across pods.
See [sharing state across connections](/how-to/patterns/share-state/).

Two connections from the same browser can land on different pods. Nothing in
the configuration above pins them together, and websocketd offers no session
affinity of its own. If you need it, set it on the Service or on the Ingress
controller.

## Keeping the ports in step

Three numbers have to agree: `--port=8080` in the container args, the
`containerPort`, and the Service's `targetPort`. Naming the port `ws` and
referring to it by name, as above, removes two of the three chances to get
that wrong.

## Next

- [Run in a container](/how-to/deploy/docker/) for the image these objects run.
- [The exposure checklist](/how-to/deploy/public-internet/) for what to settle
  before the Ingress is public.
- [The security model](/understanding/security-model/) for what
  `--origin` does and does not cover.
