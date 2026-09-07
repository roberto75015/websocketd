---
title: "Stream output from a Python script"
weight: 10
description: "Launch the interpreter with python3 -u, or flush after each print, so your script's lines reach the browser as they are produced instead of arriving together at exit."
---

Run your script with `python3 -u`. The `-u` flag tells Python to write
each line to stdout the moment your code produces it, rather
than collecting lines in a buffer and writing them out in one block.

```sh
websocketd --port=8080 python3 -u ./count.py
```

```python
import time

for count in range(1, 6):
    print(count)
    time.sleep(0.5)
```

Connect a client and the five numbers arrive half a second apart. Remove
the `-u` and nothing arrives until the script exits, at which point all
five appear together. The output is identical either way. Only its
timing changes. [Output buffering](/understanding/output-buffering/)
explains why the runtime does this.

## If you do not control the command line

Sometimes you cannot add `-u`. The script may be launched by its shebang
line, or through `--dir`, or by a wrapper you did not write. In that case
flush from inside the script.

Pass `flush=True` to each `print` you want delivered immediately:

```python
import time

for count in range(1, 6):
    print(count, flush=True)
    time.sleep(0.5)
```

Or call `sys.stdout.flush()` after the writes that matter, which also
works if you are producing output with `sys.stdout.write` rather than
`print`:

```python
import sys
import time

for count in range(1, 6):
    print(count)
    sys.stdout.flush()
    time.sleep(0.5)
```

Both are equivalent to `-u` for the lines you apply them to. `-u` is
safer, because it cannot be forgotten on the one `print` that mattered.

## Setting PYTHONUNBUFFERED takes two steps

Python also stops buffering when the environment variable
`PYTHONUNBUFFERED` holds any non-empty value. Exporting it in your shell
is not enough on its own.

websocketd builds a fresh environment for every process it launches. It
copies across only the variables named by `--passenv`, so a variable you
did not name never reaches your script. Export the variable *and* name
it:

```sh
export PYTHONUNBUFFERED=1
websocketd --port=8080 --passenv=PATH,PYTHONUNBUFFERED ./count.py
```

`--passenv` **replaces** the default list rather than adding to it. The
default is `PATH,LD_LIBRARY_PATH` on Linux and `PATH,DYLD_LIBRARY_PATH`
on macOS, so writing `--passenv=PYTHONUNBUFFERED` on its own hands your
script an environment with no `PATH` in it at all. Name every variable
you need, including `PATH`, on the one flag.

A script that never launches another program may not notice a missing
`PATH`. One that does will: any lookup by bare command name fails with a
file-not-found error, which looks nothing like an environment problem
from the inside.

Prefer `-u` unless you specifically need the environment variable, for
example because the same script has to behave the same way under a
container runtime that sets it.

## Next

- [Output buffering](/understanding/output-buffering/) is the reason all
  of this is necessary.
- [Pass data into your script](/how-to/patterns/pass-arguments/) covers
  the rest of what `--passenv` is for.
- [Environment variables](/reference/environment-variables/) lists
  everything websocketd sets for your process.
- [Debug a script](/how-to/patterns/debug-a-script/) shows the timing of
  what arrives.
