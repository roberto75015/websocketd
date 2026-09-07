---
title: "Stream output from a C program"
weight: 50
description: "Call setbuf(stdout, NULL) at the start of main so every printf reaches websocketd immediately instead of collecting in the C library's buffer."
---

Call `setbuf(stdout, NULL)` as the first statement of `main`, before any
output. That turns off the C standard library's buffering on stdout for
the rest of the process, so every `printf` goes straight down the pipe.

```sh
cc -o count count.c
websocketd --port=8080 ./count
```

```c
#include <stdio.h>
#include <unistd.h>

int main(void) {
    setbuf(stdout, NULL);

    for (int i = 1; i <= 5; i++) {
        printf("%d\n", i);
        usleep(500000);
    }

    return 0;
}
```

Connect a client and the five numbers arrive half a second apart. Remove
the `setbuf` line and all five arrive together when the program exits.
[Output buffering](/understanding/output-buffering/) explains why.

Call it before you write anything. `setbuf` must be applied to a stream
that has had no input or output performed on it yet, so the top of
`main` is the place for it.

## setvbuf, if you prefer the explicit form

`setvbuf` does the same job with the mode spelled out, and returns a
value you can check:

```c
setvbuf(stdout, NULL, _IONBF, 0);
```

`_IONBF` means unbuffered. The two other modes name the behaviour this
page is working around: `_IOLBF` is line-buffered, which flushes on each
newline, and `_IOFBF` is fully buffered, which is what your program gets
by default when stdout is a pipe.

`_IOLBF` is a reasonable middle choice under websocketd, since a
WebSocket message boundary is a newline anyway:

```c
setvbuf(stdout, NULL, _IOLBF, 0);
```

## fflush, if you want to keep the buffer

Where the program produces a lot of output and you want to keep
buffering for the bulk of it, leave the stream alone and call
`fflush(stdout)` after the writes that must go out now:

```c
for (int i = 1; i <= 5; i++) {
    printf("%d\n", i);
    fflush(stdout);
    usleep(500000);
}
```

This streams identically. The cost is one call to remember at every
site that matters. Forget one and that message silently never arrives.

## For a program you cannot recompile

If the binary is not yours, wrap it in `stdbuf`, which sets the buffering
mode through the loader before the program starts:

```sh
websocketd --port=8080 stdbuf -oL ./legacy-binary
```

`-oL` makes stdout line-buffered. This works only for programs that use
the C standard library's `stdio` and have not set their own buffering
explicitly. `stdbuf` ships with GNU coreutils, so it is present on
typical Linux systems and not on macOS by default.

## Next

- [Output buffering](/understanding/output-buffering/) is the reason all
  of this is necessary.
- [Message framing](/understanding/message-framing/) covers the trailing
  newline that `printf("%d\n", i)` supplies here.
- [Debug a script](/how-to/patterns/debug-a-script/) shows the timing of
  what arrives.
