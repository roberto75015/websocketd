---
title: "Stream output from a PHP script"
weight: 40
description: "Call flush() after each line, ob_flush() first if output buffering is on, and open the file with the full <?php tag so PHP runs your code instead of printing it."
---

Call `flush()` after each line you want delivered immediately. If your
script or its framework has PHP's own output buffering turned on, call
`ob_flush()` first to empty that layer, then `flush()` to push the bytes
out of the process.

```sh
websocketd --port=8080 php ./count.php
```

```php
<?php
for ($count = 1; $count <= 5; $count++) {
    echo $count . "\n";
    flush();
    usleep(500000);
}
```

Connect a client and the five numbers arrive half a second apart.
[Output buffering](/understanding/output-buffering/) explains why a
runtime holds output back when stdout is a pipe.

## The two layers, and which call empties which

PHP can hold your output in two separate places, and they need different
calls.

The first is PHP's own **output buffer**, an internal store that
`ob_start()` turns on and that some frameworks and some `php.ini`
settings turn on for you. While it is active, `echo` writes into that
store and nothing leaves the process. `ob_flush()` empties it.

The second is the buffer belonging to the layer underneath, which holds
bytes that have left PHP's output buffer but not yet reached the pipe.
`flush()` empties that one.

Calling both, in that order, covers either arrangement:

```php
<?php
echo "ready\n";
ob_flush();
flush();
```

`ob_flush()` emits a notice if no output buffer is active, so guard it
when you are not sure:

```php
<?php
echo "ready\n";
if (ob_get_level() > 0) {
    ob_flush();
}
flush();
```

If you want no buffering at all rather than a flush at every write, turn
it off once at the top instead:

```php
<?php
while (ob_get_level() > 0) {
    ob_end_flush();
}
```

## Open the file with <?php, never <?

A file that begins with `<?` rather than `<?php` is using the **short
open tag**, an abbreviated form that PHP only recognises when the
`short_open_tag` setting is on. That setting is off in a default
installation.

When it is off, PHP does not treat `<?` as the start of code. It treats
the whole file as literal text and prints your source back out. Your
program never runs.

From websocketd's side nothing looks wrong. Your script produced output,
websocketd forwarded it, and the client received it. The symptom is a
browser showing PHP source code, or showing output that never changes,
which is easy to mistake for a buffering problem when in fact no PHP
executed at all.

Always open the file with the full tag:

```php
<?php
echo "this actually runs\n";
```

Check which configuration is in play with `php -i | grep short_open_tag`.
The command-line PHP binary often reads a different `php.ini` from the
one your web server uses, so a script that works under a web server can
still fail here.

## Each line needs a trailing newline

websocketd sends a WebSocket message when it reads a newline, so `echo`
without one leaves the line held rather than sent. `echo $count . "\n"`
above supplies it. This is a separate requirement from flushing, and
either one alone produces the same silence in the browser. [Message
framing](/understanding/message-framing/) has the rule.

## Next

- [Output buffering](/understanding/output-buffering/) is the reason
  flushing is necessary.
- [Message framing](/understanding/message-framing/) covers the newline
  requirement.
- [Debug a script](/how-to/patterns/debug-a-script/) shows how to see
  what is really arriving, which distinguishes a short-tag failure from
  a buffering one immediately.
