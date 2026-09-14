---
title: "Deploy websocketd"
weight: 20
description: "Run websocketd behind a proxy, under an init system, in a container, and on a public address."
---

Pick the page that matches your environment. Each one gives you a working
configuration first.

The proxy pages solve the same problem three ways: your public server
terminates the connection, and the WebSocket upgrade has to survive the trip
to websocketd. Choose the one you already run.
