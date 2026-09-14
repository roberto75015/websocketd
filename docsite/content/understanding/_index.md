---
title: "Understanding websocketd"
weight: 40
description: "Why websocketd behaves the way it does: the process model, framing, buffering, lifecycle, environment, security posture, and the decisions behind them."
---

websocketd is small enough that you can hold all of it in your head, and
these pages are how you get there. They explain the reasoning behind the
behaviour rather than telling you which command to type.

Read this section when something websocketd does surprises you and you
want to know whether it is a bug, a setting, or the design. Read it
before you build anything larger than a demo, because two of the
decisions here (one process per connection, and newline-delimited
framing) shape what your application can be.

If you have a job to finish right now, the [how-to
guides](/how-to/) give steps rather than reasons. If you want an exact
value for a flag, a variable, or an exit code, it is in the
[reference](/reference/).

If you are new to websocketd, start with the
[tutorial](/start/tutorial/) and come back.
