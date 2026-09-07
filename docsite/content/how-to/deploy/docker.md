---
title: "Run in a container"
weight: 50
description: "Package websocketd and your script into a container image, with the no-pty limitation and PID 1 signal handling."
---

websocketd never allocates a pseudo-terminal. A
pseudo-terminal, or pty, is the kernel object that makes a program believe it
is talking to a real terminal. websocketd connects your program's stdin and
stdout to ordinary pipes instead, in a container or anywhere else.

So a program that needs a terminal does not work through websocketd, and no
Docker flag changes that. That rules out `docker run -it`, `screen`, `watch`,
anything that calls `isatty()` and behaves differently, and anything that
draws with cursor positioning or expects job control. This is a permanent
design property, not a gap; see [design decisions](/understanding/design-decisions/).
Everything below assumes a non-interactive program.

## A working image

```dockerfile
FROM golang:1.24-alpine AS build
WORKDIR /src
RUN apk add --no-cache git \
 && git clone --depth 1 --branch v0.5.0 https://github.com/joewalnes/websocketd.git . \
 && go build -o /out/websocketd .

FROM alpine:3.21
RUN adduser -S -D -H websocketd
COPY --from=build /out/websocketd /usr/local/bin/websocketd
COPY myscript.sh /app/myscript.sh
RUN chmod +x /app/myscript.sh
USER websocketd
EXPOSE 8080
ENTRYPOINT ["websocketd", "--port=8080", "--address=0.0.0.0", "--origin=https://example.com", "/app/myscript.sh"]
```

Build it and run it:

```sh
docker build -t myapp-ws .
docker run --init -p 8080:8080 myapp-ws
```

`ws://localhost:8080/` now reaches your script.

## `--address=0.0.0.0`

Bind all interfaces inside the container. This is websocketd's default, so
the flag only documents the intent. The opposite is a trap:
`--address=127.0.0.1` inside a container binds the container's own loopback,
which is a different loopback from the host's. The `-p 8080:8080` mapping
forwards to the container's external interface, so a loopback-bound
websocketd is unreachable through it.

The other deployment pages say the reverse: bind loopback and front it
with a proxy. Inside a container the network namespace is already
the isolation boundary.

## `--init`, and why websocketd should not be PID 1

`docker run --init` puts a small init process at PID 1 and runs websocketd as
its child. Use it.

websocketd starts one process per connection and waits on each one it started.
It does not reap arbitrary orphans. If a script spawns a background child and
exits, that grandchild is re-parented to PID 1, and if PID 1 is websocketd it
is never reaped. Zombie entries then accumulate for the life of the container.

websocketd also installs no signal handler of its own. An init at PID 1
forwards `SIGTERM` from `docker stop` to it and gives the stop a predictable
shape rather than a ten-second wait followed by `SIGKILL`.

If you would rather not pass `--init` at every `docker run`, put an init
binary in the image and make it the entrypoint:

```dockerfile
RUN apk add --no-cache tini
ENTRYPOINT ["/sbin/tini", "--", "websocketd", "--port=8080", "--address=0.0.0.0", "/app/myscript.sh"]
```

## Your script's runtime must be in the image

`alpine` has a shell and little else. A Python script needs `apk add
python3`, a Node script needs `nodejs`, and so on.

A missing runtime fails in one of two places. If the `ENTRYPOINT` command
itself is missing, websocketd resolves it on `PATH` at startup and exits
before binding, with `unable to locate specified COMMAND` on stderr. If the
command is a script whose interpreter is missing, websocketd starts normally
and each connection fails instead.

Alpine uses musl rather than glibc. If your script's runtime or a compiled
helper needs glibc, base the runtime stage on `debian:bookworm-slim` instead.

## Logs

websocketd writes to stdout and stderr and never to a file, so `docker logs`
is the log. The origin-policy startup banner goes to stderr and appears there
on every start until you give websocketd an origin policy, which the
`ENTRYPOINT` above does with `--origin`.

## Next

- [Deploy to Kubernetes](/how-to/deploy/kubernetes/) to run this image in a
  cluster.
- [Serve behind nginx](/how-to/deploy/nginx/) to put TLS and a public port in
  front of the container.
- [The exposure checklist](/how-to/deploy/public-internet/) before publishing
  the port beyond your own machine.
- [Why there is no pty](/understanding/design-decisions/).
