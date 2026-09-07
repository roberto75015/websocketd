---
title: "Environment variables"
weight: 20
description: "Every environment variable websocketd sets on a spawned process, what populates it, and which ones the client controls."
---

A process websocketd spawns for a WebSocket connection receives the
variables below, in CGI style. A process spawned for a `--cgidir` request
receives a different set, built by Go's `net/http/cgi`; the
[differences](#differences-under---cgidir) are listed at the end of this
page.

Client-controlled means the value comes from data the client sent and can
therefore be anything the client chooses.

## RFC 3875 variables

| Variable | Populated from | Client-controlled |
|---|---|---|
| `SERVER_SOFTWARE` | `websocketd/` followed by the version string | No |
| `REMOTE_ADDR` | The client's source IP address. Literally `unix-socket` for a client connected over `--unixsocket` | No |
| `REMOTE_HOST` | The reverse DNS name of the client's address when `--reverselookup` is given, otherwise the same value as `REMOTE_ADDR` | No |
| `SERVER_NAME` | The host part of the request's `Host` header | **Yes** |
| `SERVER_PORT` | The port part of the request's `Host` header, or 80, or 443 under `--ssl`, when the header carries no port | **Yes** |
| `SERVER_PROTOCOL` | The request's HTTP version, for example `HTTP/1.1` | No |
| `GATEWAY_INTERFACE` | The constant `CGI/1.1` | No |
| `REQUEST_METHOD` | The request method, `GET` for a WebSocket upgrade | No |
| `SCRIPT_NAME` | Under `--dir`, the leading path segments that resolved to a file. With a single `COMMAND`, always `/` | Partly: derived from the request path, but only ever a path that resolves to a file under `--dir` |
| `PATH_INFO` | Under `--dir`, whatever path is left after `SCRIPT_NAME`. With a single `COMMAND`, the whole request path | **Yes** |
| `PATH_TRANSLATED` | The whole request path, before the query string | **Yes** |
| `QUERY_STRING` | Everything after `?` in the request URL, raw and still percent-encoded | **Yes** |

`SERVER_NAME` and `SERVER_PORT` come from the `Host` header, not from the
address websocketd is bound to, which matches `net/http/cgi` and
virtual-hosted web servers. This is settled behaviour, not a defect
(issue #475).

## Variables set to an empty string

Each of these is set, and set to an empty string. websocketd sets them
explicitly so that a value cannot leak in from its own environment.

| Variable | Why it is empty |
|---|---|
| `AUTH_TYPE` | websocketd performs no authentication |
| `REMOTE_USER` | websocketd performs no authentication |
| `REMOTE_IDENT` | websocketd performs no authentication |
| `CONTENT_LENGTH` | A WebSocket upgrade carries no request body |
| `CONTENT_TYPE` | A WebSocket upgrade carries no request body |

## Non-standard variables

Not part of RFC 3875, and commonly provided by CGI-style servers.

| Variable | Populated from | Client-controlled |
|---|---|---|
| `UNIQUE_ID` | A random per-connection identifier, modelled on Apache's `mod_unique_id` | No |
| `REMOTE_PORT` | The client's source TCP port. Empty for a `--unixsocket` client, which has none | No |
| `REQUEST_URI` | The full request target, path and query string, for example `/foo/blah?a=b` | **Yes** |
| `HTTPS` | The constant `on`. Set only under `--ssl`; otherwise the variable is absent, not empty | No |

## Request headers

Every request header becomes one variable named `HTTP_` followed by the
header name uppercased with each `-` replaced by `_`. `X-Forwarded-For`
becomes `HTTP_X_FORWARDED_FOR`. All of them are client-controlled.

Repeated headers of the same name are joined with `, ` into one value.
Carriage returns and newlines within a value are replaced with spaces,
and the result is trimmed.

There is no `HTTP_HOST`. Go's HTTP server moves the `Host` header out of
the header map before websocketd sees it; the host reaches the process as
`SERVER_NAME` and `SERVER_PORT` instead.

One header is dropped: a request header named `Proxy` never becomes
`HTTP_PROXY`. Many HTTP client libraries route outbound requests through
whatever `HTTP_PROXY` names, so forwarding it would let a remote caller
redirect a spawned program's own traffic. This is the httpoxy
vulnerability, CVE-2016-5385. The comparison uses the header's canonical
form, so `proxy`, `Proxy`, and `PROXY` are all dropped. Go's
`net/http/cgi` drops it for the same reason.

## Variables forwarded from websocketd's own environment

`--passenv` names, comma-separated, which of websocketd's own environment
variables are copied into every spawned process. Nothing else from
websocketd's environment reaches the process: on every platform except
Windows, websocketd clears its own environment after reading this list.

| Platform | Default `--passenv` |
|---|---|
| Linux | `PATH,LD_LIBRARY_PATH` |
| macOS | `PATH,DYLD_LIBRARY_PATH` |
| Windows | `PATH,SystemRoot,COMSPEC,PATHEXT,WINDIR` |

Four facts govern the list:

- Passing `--passenv` replaces the default rather than adding to it.
  `--passenv=API_KEY` alone leaves the spawned process with no `PATH`.
- A named variable that is unset, or set to an empty string, in
  websocketd's environment is dropped rather than forwarded empty.
- `HTTPS` is skipped even when named, because that variable is
  websocketd's own `--ssl` signal.
- The list has no effect on the request-derived variables above. They are
  built per request and are always present.

## Differences under `--cgidir`

A `--cgidir` request is handed to Go's `net/http/cgi`, which builds its
own environment. `SERVER_SOFTWARE` is overridden to websocketd's value
and the `--passenv` list is added, but the rest comes from Go.

`GATEWAY_INTERFACE`, `SERVER_PROTOCOL`, `REQUEST_METHOD`, `QUERY_STRING`,
`REQUEST_URI`, `SERVER_NAME`, `SERVER_PORT`, `REMOTE_ADDR`,
`REMOTE_PORT`, and the `HTTP_<NAME>` mapping carry the same meaning in
both modes, and `Proxy` is dropped in both. These differ:

| Variable | WebSocket (`COMMAND` or `--dir`) | `--cgidir` |
|---|---|---|
| `SCRIPT_NAME` | The path that resolved to a file, or `/` | Always empty |
| `PATH_INFO` | The path left after `SCRIPT_NAME` | The whole request path |
| `PATH_TRANSLATED` | The whole request path | Not set |
| `UNIQUE_ID` | A random per-connection identifier | Not set |
| `AUTH_TYPE`, `REMOTE_USER`, `REMOTE_IDENT` | Set to an empty string | Not set |
| `CONTENT_LENGTH` | Set to an empty string | Set only when the request has a body |
| `CONTENT_TYPE` | Set to an empty string | Set only when the request carries a `Content-Type` header |
| `REMOTE_HOST` | Honours `--reverselookup` | Always the client IP address |
| `SCRIPT_FILENAME` | Not set | The resolved path of the script on disk |
| `HTTP_HOST` | Not set | The request's `Host` header |
| `HTTP_COOKIE` with repeated headers | Joined with `, ` | Joined with `; ` |
| `PATH` when `--passenv` does not name it | Not set | `/bin:/usr/bin:/usr/ucb:/usr/bsd:/usr/local/bin`, a fallback `net/http/cgi` supplies |

A `--cgidir` request path must name the script file exactly. There is no
trailing path information to split off, which is why `PATH_INFO` is the
whole path and `SCRIPT_NAME` is empty.

## See also

- [The CGI environment](/understanding/cgi-environment/) explains how a
  request becomes an environment, and where the trust boundaries fall.
- [Passing data into your script](/how-to/patterns/pass-arguments/)
  compares the query string with `--passenv`.
- [CLI flags](/reference/cli-flags/) covers `--passenv`, `--reverselookup`,
  `--cgidir`, and `--dir`.
