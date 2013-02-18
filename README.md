# Overview

Clipper is an OS X "launch agent" that runs in the background providing a
service that exposes the local clipboard to tmux sessions and other processes
running both locally and remotely.

# Problem

You're running tmux, possibly on a remote machine via ssh, and want to copy
something using tmux copy mode into your local system clipboard.

You can hold down Option and click to make a selection, bypassing tmux and
allowing you to copy straight to the system clipboard, but that won't work if
you're using vertical splits (because the selection will operate across the
entire width of the terminal, crossing the splits) or if you want to grab more
than what is currently visible.

As a workaround for the vertical split problem, you can hold down Option +
Command to make a rectangular (non-contiguous) selection, but that will grab
trailing whitespace as well, requiring you to manually clean it up later.
Again, it won't work if you want to grab more than what is currently visible.

As a result, you often find yourself doing a tiresome sequence of:

1. Copy a selection using tmux copy mode on a remote machine
2. Still on the remote machine, open a new, empty buffer in Vim
3. Enter Vim paste mode (`:set paste`)
4. Paste the tmux copy buffer into the Vim buffer
5. Write the file to a temporary location (eg. `:w /tmp/buff`)
6. From the local machine, get the contents of the temporary file into the local
   system clipboard with `ssh user@host cat /tmp/buff | pbcopy` or similar

# Solution

OS X comes with a `pbcopy` tool that allows you to get stuff into the clipboard
from the command-line. We've already seen this at work above. Basically, we can
do things like `echo foo | pbcopy` to place "foo" in the system clipboard.

tmux has a couple of handy commands related to copy mode buffers, namely
`save-buffer` and `show-buffer`. With these, you can dump the contents of a
buffer into a text file, or emit it over standard out.

In theory, combining these two elements, we can add something like this to our
`~/.tmux.conf`:

    bind-key C-y run-shell "tmux save-buffer - | pbcopy"

In practice, this doesn't work because tmux uses the `daemon(3)` system call,
which ends up putting it in a different execution context from which it cannot
interact with the system clipboard. For (much) more detail, see:

- http://developer.apple.com/library/mac/#technotes/tn2083/_index.html

One workaround comes in the form of the `reattach-to-user-space` tool available
here:

- https://github.com/ChrisJohnsen/tmux-MacOSX-pasteboard

This is a wrapper which allows you to launch a process and have it switch from
the daemon execution context to the "user" context where access to the system
clipboard is possible. The suggestion is that you can add a command like the
following to your `~/.tmux.conf` to make this occur transparently whenever you
open a new pane or window:

    set-option -g default-command "reattach-to-user-namespace -l zsh"

Despite the fact that the wrapper tool relies on an undocumented, private API,
it is written quite defensively and appears to work pretty well. While this is a
workable solution when running only the local machine, we'll need something else
if we want things to work transparently both locally and remotely. This is where
Clipper comes in.

Clipper is a server process that you can run on your local machine. It will
listen on the network for connections, and place any content that it receives
into the system clipboard.

It can be set to run automatically as a "launch agent" in the appropriate
execution context, which means you don't have to worry about starting it, and it
will still have access to the system clipboard despite being "daemon"-like.

Through the magic of `ssh -R` it is relatively straightforward to have a shared
tmux configuration that you can use both locally and remotely and which will
provide you with transparent access to the local system clipboard from both
places.

# Setup

## Installing

I'm relatively new to Go so am not yet sure what the best way to distribute a
command-line applicaton is. So, for now, it boils down to:

    git clone git://git.wincent.com/clipper.git
    cd clipper
    go build clipper.go
    sudo cp clipper /usr/bin
    cp com.wincent.clipper.plist ~/Library/LaunchAgents/
    launchctl load -w -S Aqua ~/Library/LaunchAgents/com.wincent.clipper.plist

Alternatively, if you'd like to run Clipper manually, you can do so with:

    ./clipper [--address=IP_ADDR] [--port=PORT] [--logfile=LOGFILE]

Note that both of these commands will fail to do the right thing inside of a
tmux session. Run `launchctl` from outside of tmux, otherwise Clipper will find
itself in the wrong execution context. Similarly, when running manually, either
run Clipper outside of tmux or use the aforementioned `reattach-to-user-space`
as a wrapper.

## Uninstalling

The launch agent installation can be reversed with:

    launchctl unload ~/Library/LaunchAgents/com.wincent.clipper.plist
    rm ~/Library/LaunchAgents/com.wincent.clipper.plist
    sudo rm /usr/bin/clipper

As before, note that you should only run `launchctl` outside of a tmux session.

To kill a manually-launched instance of Clipper, just hit Control+C in the
terminal where it is running.

## Configuring tmux

Now we can use a slight modification of our command from earlier. Assuming we
kept the standard listen address (127.0.0.1) and port (8377), we can use a
command like this:

    bind-key C-y run-shell "tmux save-buffer - | nc localhost 8377"

Here we're using netcat (`nc`)` to send the contents of the buffer to the
listening Clipper agent.

## Configuring Vim

There is nothing inherent in Clipper that ties it to tmux. We can use it from
any process, including Vim.

For example, we can add a mapping to our `~/.vimrc` to send the last-yanked text
to Clipper:

    nnoremap <leader>y :call system('nc localhost 8377', @0)<CR>

## Configuring Zsh

By setting up an alias:

    alias clip="nc localhost 8377"

you can conveniently get files and other content into your clipboard:

    cat example.txt | clip

## Configuring SSH

Again, assuming default address and port, we can use `-R` like this:

    ssh -R localhost:8337:localhost:8377 user@host.example.org

With this, a tmux process running on the remote host can use the same
configuration file, and our `run-shell` from above will send the buffer
contents to localhost:8377 on the remote machine, which will then be forwarded
back over the SSH connection to localhost:8377 on the local machine, where
Clipper is listening.

See the "Security" section below for some considerations.

To make this automated, entries can be set up in `.ssh/config`:

    Host host.example.org
      RemoteForward 8377 localhost:8377

# Security

At the moment, Clipper doesn't employ any authentication. It does, by default,
listen only on the loopback interface, which means that random people on your
network won't be able to connect to it. People with access to your local
machine, however, will have access; they can push content into your clipboard
even if they can't read from it.

This may be fine on a single-user machine, but when you start using `ssh -R` to
expose your Clipper instance on another machine you're evidently increasing your
surface area. This may be ok, but my intention is to add an authentication
mechanism to the protocol in the near future in any case.

# Author

Clipper is written and maintained by Wincent Colaiuta <win@wincent.com>.

# Development

Development in progress can be inspected via the project's Git web-based
repository browser at:

- https://wincent.com/repos/clipper

the clone URL for which is:

- git://git.wincent.com/clipper.git

Mirrors exist on GitHub and Gitorious; these are automatically updated once
per hour from the authoritative repository:

- https://github.com/wincent/clipper
- https://gitorious.org/clipper/clipper

Patches are welcome via the usual mechanisms (pull requests, email, posting to
the project issue tracker etc).

# Website

The official website for Clipper is:

- https://wincent.com/products/clipper

Bug reports should be submitted to the issue tracker at:

- https://wincent.com/issues

# Donations

Clipper is free software released under the terms of the BSD license. If you
would like to support further development you can make a donation via PayPal to
win@wincent.com:

- https://www.paypal.com/xclick/business=win@wincent.com&item_name=clipper+donation&no_note=1&currency_code=EUR&lc=GB

# License

Copyright 2013 Wincent Colaiuta. All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

1. Redistributions of source code must retain the above copyright notice,
   this list of conditions and the following disclaimer.
2. Redistributions in binary form must reproduce the above copyright notice,
   this list of conditions and the following disclaimer in the documentation
   and/or other materials provided with the distribution.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDERS OR CONTRIBUTORS BE
LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
POSSIBILITY OF SUCH DAMAGE.

# History

## 0.1 (not yet released)

- Initial release
