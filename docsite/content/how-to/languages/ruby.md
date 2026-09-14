---
title: "Stream output from a Ruby script"
weight: 20
description: "Set STDOUT.sync = true at the top of your script so every puts is written immediately instead of collecting in a buffer until the script exits."
---

Set `STDOUT.sync = true` once, at the top of your script, before you
print anything. Ruby then writes every `puts` and `print` straight to
stdout instead of collecting them in a buffer.

```sh
websocketd --port=8080 ruby ./count.rb
```

```ruby
STDOUT.sync = true

(1..5).each do |count|
  puts count
  sleep(0.5)
end
```

Connect a client and the five numbers arrive half a second apart. Remove
the `sync` line and all five arrive together when the script exits.
[Output buffering](/understanding/output-buffering/) explains why.

There is no per-call flush to remember afterwards. Setting `sync`
changes the stream for the life of the process, so every later write
goes out immediately without any further work.

## If you cannot edit the top of the script

Ruby has no command-line switch that turns buffering off, so the change
has to happen in Ruby code. Where the script is not yours to edit, put
the one line in a small wrapper that loads it:

```ruby
STDOUT.sync = true
load File.expand_path('vendored_script.rb', __dir__)
```

Wrap the wrapper, not the original:

```sh
websocketd --port=8080 ruby ./wrapper.rb
```

## Flushing selectively instead

If you have a reason to keep buffering on, `STDOUT.flush` empties the
buffer at a point you choose:

```ruby
(1..5).each do |count|
  puts count
  STDOUT.flush
  sleep(0.5)
end
```

This is more code and one more thing to forget. Use it only when you
write enough output for the buffering to pay for itself.

## Next

- [Output buffering](/understanding/output-buffering/) is the reason all
  of this is necessary.
- [Message framing](/understanding/message-framing/) covers the other
  requirement: each message needs a trailing newline, which `puts` adds
  for you and `print` does not.
- [Debug a script](/how-to/patterns/debug-a-script/) shows the timing of
  what arrives.
