---
title: "Language-specific fixes"
weight: 10
description: "One page per language for the problem that dominates websocketd's issue history: a script that streams in a terminal but sends nothing, or everything at once, under websocketd."
---

Nearly every report of "my script sends nothing" has the same cause:
your language's runtime holds output in a private buffer when standard
output is a pipe rather than a terminal, and websocketd always gives it
a pipe. The fix is one flag or one line, and it is different in every
language.

Find yours below. Each page opens with the change to make. If you want
to know why the runtime behaves this way, read
[output buffering](/understanding/output-buffering/) afterwards; you do
not need any of it to apply the fix.
