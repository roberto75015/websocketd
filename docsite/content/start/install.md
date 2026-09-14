---
title: "Install"
weight: 10
description: "Download the websocketd binary for your platform, or build it from source with go build."
---

`websocketd` is a single binary with no runtime and no configuration
file. Installing it means putting that one file somewhere on your `PATH`.

## Download a release

Every release publishes a binary for each supported platform on the
[releases page](https://github.com/joewalnes/websocketd/releases). Open
the latest release, pick the archive matching your operating system and
processor architecture, and download it. Builds are published for Linux
on 32-bit and 64-bit x86 and on 32-bit and 64-bit ARM, for macOS on Intel
and Apple Silicon, and for Windows on 32-bit and 64-bit x86.

Unpack the archive and move the binary onto your `PATH`:

```sh
unzip websocketd-*.zip
sudo mv websocketd /usr/local/bin/
```

Each archive also contains the README, the license, and the changelog.
You do not need any of them to run the program.

Debian and Red Hat packages are published alongside the archives if you
would rather install through your system package manager.

## Build from source

You need Go 1.21 or newer.

```sh
git clone https://github.com/joewalnes/websocketd.git
cd websocketd
go build
```

That is the entire build. It leaves a `websocketd` binary in the current
directory, which you can move onto your `PATH` the same way. There is no
Makefile to run and no dependencies to fetch by hand; Go's module system
fetches the one library `websocketd` uses.

## Check that it worked

```sh
websocketd --version
```

That prints one line: the release number, the Go toolchain the binary was
built with, and your platform.

If your shell says `command not found`, the binary is not on your `PATH`.
Check that the directory you moved it to is listed in `echo $PATH`.

Now go and use it: the [tutorial](/start/tutorial/) takes about fifteen
minutes and ends with a browser talking to a script you wrote.
