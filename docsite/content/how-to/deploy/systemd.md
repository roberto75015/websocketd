---
title: "Start at boot with systemd"
weight: 40
description: "A complete systemd unit that runs websocketd as an unprivileged service, starts it at boot, and restarts it on failure."
---

websocketd runs in the foreground and never forks into the background, so a
plain `Type=simple` unit is all it needs. A complete one:

```ini
# /etc/systemd/system/websocketd.service
[Unit]
Description=websocketd wrapping myapp
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=websocketd
Group=websocketd
WorkingDirectory=/opt/myapp
ExecStart=/usr/local/bin/websocketd \
    --port=8080 \
    --address=127.0.0.1 \
    --origin=https://example.com \
    /opt/myapp/myscript.sh
Restart=on-failure
RestartSec=2
NoNewPrivileges=yes
PrivateTmp=yes

[Install]
WantedBy=multi-user.target
```

Create the account it runs as, install the unit, and start it:

```sh
sudo useradd --system --shell /usr/sbin/nologin --home-dir /opt/myapp websocketd
sudo systemctl daemon-reload
sudo systemctl enable --now websocketd
```

Check it came up, and follow its output:

```sh
systemctl status websocketd
journalctl -u websocketd -f
```

## Where the logs go

websocketd writes its access log and its diagnostics to stdout and stderr,
and never to a file. systemd captures both into the journal, so `journalctl
-u websocketd` is the log. There is no log path to configure and no log
rotation to set up; the journal's own retention settings apply.

The startup banner about origin policy goes to stderr, so it lands in the
journal on every restart until you give websocketd an origin policy. The
`--origin` in the unit above is what silences it.

## Running unprivileged, and what that costs you

`User=websocketd` runs the service as an unprivileged account. Do this. A
websocketd endpoint is a way to run a program, so the account it runs as is
the blast radius if a wrapped script has a flaw.

The consequence is that an unprivileged process cannot bind a port below
1024. `--port=80` and `--port=443` will fail with a permission error, and the
unit will restart-loop. Two ways around it, in order of preference, and
one that does not work:

**Put a reverse proxy on 80 and 443.** The proxy already runs as root long
enough to bind them, and you want it there anyway for TLS. Keep websocketd on
a high port bound to `127.0.0.1` as the unit above does. See
[nginx](/how-to/deploy/nginx/), [Apache](/how-to/deploy/apache/) or
[HAProxy](/how-to/deploy/haproxy/).

**Grant the capability instead of the account.** Add
`AmbientCapabilities=CAP_NET_BIND_SERVICE` to the `[Service]` section. systemd
then lets this one unprivileged process bind low ports without giving it any
other privilege. Change `--port` to 80 or 443. Drop the reverse proxy only
if you also do not want TLS, which websocketd can serve but a proxy serves
better.

**Let systemd own the socket.** A separate `websocketd.socket` unit with
`ListenStream=80` will not work: websocketd opens its own listener and does
not accept a pre-opened file descriptor from systemd.

Do not solve it by running as root.

## Restart behaviour

`Restart=on-failure` restarts websocketd when it exits non-zero or is killed
by a signal, and leaves it stopped after a clean `systemctl stop`. That is
what you want for a server.

It also means a configuration mistake restart-loops rather than failing
loudly. websocketd validates its flags before binding and exits with a
non-zero code and a message on stderr, so `journalctl -u websocketd` will
show the same complaint repeating. See [exit codes](/reference/exit-codes/)
for what each one means. `RestartSec=2` keeps that loop slow enough to read.

## Passing environment variables to the script

A `Environment=` line in the unit sets a variable for the websocketd process,
not for the scripts it runs. websocketd starts each script with a controlled
environment and passes through only what you name:

```ini
Environment=MYAPP_TOKEN=s3cret
ExecStart=/usr/local/bin/websocketd --port=8080 --passenv=MYAPP_TOKEN /opt/myapp/myscript.sh
```

See [the CGI environment](/understanding/cgi-environment/) for what a script
receives, and [environment variables](/reference/environment-variables/) for
the full list.

## Next

- [Serve behind nginx](/how-to/deploy/nginx/) to put TLS and a public port in
  front of this.
- [The exposure checklist](/how-to/deploy/public-internet/) before this goes
  on a public address.
- [Exit codes](/reference/exit-codes/) when the unit will not stay up.
