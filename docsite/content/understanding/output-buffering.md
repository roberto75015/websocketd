---
title: "Output buffering"
weight: 30
description: "Language runtimes withhold output when standard output is a pipe rather than a terminal, which is why a script that streams perfectly in a terminal can appear silent under websocketd."
---

Your program works in a terminal. Under websocketd it sends nothing, or
sends everything at once when it exits. Nothing in websocketd changed
between those two runs, and nothing in your program changed either. What
changed is what your program's stdout is connected to, and your
language's runtime quietly changed its own behaviour in response.

This is the most common problem people have with websocketd, by a
distance. It is also not a websocketd problem, which is exactly why it
is so confusing: every part of the system is behaving as designed.

## The three words you need

A **pipe** is a one-way channel between two processes, provided by the
operating system. One process writes bytes in at one end, another reads
them out at the other. websocketd creates three pipes for every program
it launches, one each for stdin, stdout, and stderr, and reads your
output from the middle one. A pipe is not a terminal and does not
pretend to be one.

**Line-buffered** means the runtime collects the characters you print
until it sees a newline, then writes the whole line to the operating
system in one go. Output appears line by line, at roughly the moment
your code produced it.

**Block-buffered**, sometimes called fully buffered, means the runtime
collects output until it has accumulated a fixed-size block, commonly
four or eight kilobytes, and only then writes it out. Newlines are not
special. Output appears in bursts of several kilobytes, or, for a
program that never produces that much, only when the program exits and
the runtime flushes what is left.

## Why the runtime switches modes

This behaviour predates the web by decades and was, at the time, an
obviously good idea.

Writing to the operating system is comparatively expensive. A program
that prints a million short lines and asks the kernel to handle each one
separately does a million system calls. Buffering in user space
collapses that into a few hundred, which is a large win for anything
whose output is destined for a file or another program.

Interactivity is the exception. If a human is watching the output, the
program has to give up that win, because output that arrives eight
kilobytes at a time is useless to a person waiting for a prompt. So the
C standard library adopted a rule that nearly every language runtime
since has inherited: stdout is line-buffered when it refers to
an interactive device, and block-buffered otherwise. The rule leaves
stderr unbuffered, on the reasoning that error messages must not be lost
in a buffer when the program crashes.

The runtime decides which case it is in exactly once, when the stream is
first set up, by asking the operating system whether the file descriptor
is a terminal. Under websocketd it is a pipe, so the answer is no, so
block buffering it is. Your program was never consulted.

The effect is easy to see. Take a script that prints three lines half a
second apart and run it twice: once with its stdout connected to a pipe,
and once with its stdout connected to a pseudo-terminal. Through the
pipe, all three lines arrive together at 1.64 seconds, when the process
exits. Through the pseudo-terminal, they arrive at 0.01, 0.55, and 1.10
seconds, as they are printed. Same interpreter, same script, no flags,
no configuration. Only the far end of the file descriptor differs.

## Why websocketd cannot fix it for you

The buffer is inside your program's address space. It belongs to your
program's runtime library, not to the operating system and not to
websocketd. From the outside, a program holding eight kilobytes of your
output in a private array is indistinguishable from a program that has
not produced any output yet. There is nothing for websocketd to read,
and no way for it to ask.

There is one thing websocketd could do that would change the answer: it
could allocate a pty, a pseudo-terminal, so that the runtime's
is-this-a-terminal check came back true and line buffering stayed on.
It deliberately does not, and that decision has consequences well beyond
buffering. It is discussed in [design
decisions](/understanding/design-decisions/), and its other effects are
described in [message framing](/understanding/message-framing/).

Since websocketd cannot reach into your program, the fix has to be
inside it. Every language provides a way to say either "flush this now"
or "never block-buffer this stream in the first place", and it is
usually one line or one command-line flag. Which line, in which
language, is the whole content of the how-to pages:
[Python](/how-to/languages/python/),
[Ruby](/how-to/languages/ruby/),
[Node.js](/how-to/languages/nodejs/),
[PHP](/how-to/languages/php/),
[C](/how-to/languages/c/), and
[Windows scripts](/how-to/languages/windows-scripts/).

## Two independent gates

Buffering is the first of two things that must go right before a line
reaches the browser, and it helps to keep them separate in your head.

The first gate is your runtime: has it written the bytes to the pipe?
That is this page.

The second gate is websocketd: has it seen a newline yet? websocketd
sends a WebSocket message only when it reads one, as described in
[message framing](/understanding/message-framing/).

Both gates must open. A line that is flushed but has no trailing newline
is held by websocketd. A line that has a newline but is still sitting in
your runtime's buffer has never reached websocketd at all. The symptom,
silence in the browser, is identical either way, which is why so many
reports of one turn out to be the other.

There is a useful asymmetry when you are diagnosing this: buffering
explains almost all real cases, and it has a distinctive signature.
If everything appears in a burst the moment the program exits, or in
large clumps at irregular intervals, that is a block buffer draining. If
nothing ever appears, not even at exit, look at the newline instead.

## A note on stderr

By convention stderr is unbuffered, so it escapes this problem
entirely. That is useful while you are diagnosing: a program
that prints diagnostics to stderr will show them in websocketd's
log immediately, even while its stdout sits in a buffer. The
contrast between the two streams is itself a strong signal that you are
looking at a buffering problem.

## Next

- [Message framing](/understanding/message-framing/) covers the second
  gate, the newline rule.
- The [language how-to pages](/how-to/languages/python/) have the actual
  fix, per language.
- [Debug a script](/how-to/patterns/debug-a-script/) shows how to watch
  what is really arriving.
