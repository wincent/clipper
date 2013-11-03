# Overview

Clipper is an OS X "launch agent" that runs in the background providing a
service that exposes the local clipboard to tmux sessions and other processes
running both locally and remotely.

# At a glance

    # Installation (using Homebrew; for non-Homebrew installs see below)
    brew install clipper # run this outside of a tmux session

    # Configuration for ~/.tmux.conf:
    # tmux < 1.8: bind <prefix>-y to forward to Clipper
    bind-key y run-shell "tmux save-buffer - | nc localhost 8377"

    # tmux >= 1.8: bind "Enter" in copy mode to both copy and forward to Clipper
    bind-key -t vi-copy Enter copy-pipe "nc localhost 8377"

    # Configuration for ~/.vimrc:
    # Bind <leader>y to forward last-yanked text to Clipper
    nnoremap <leader>y :call system('nc localhost 8377', @0)<CR>

    # Configuration for ~/.bash_profile, ~/.zshrc etc:
    # Pipe anything into `clip` to forward it to Clipper
    alias clip="nc localhost 8377"

    # Configuration for ~/.ssh/config:
    # Forward Clipper connection to remote host
    Host host.example.org
      RemoteForward 8377 localhost:8377

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
`save-buffer` and `copy-pipe`. With these, you can dump the contents of a
buffer to standard out.

In theory, combining these two elements, we can add something like this to our
`~/.tmux.conf`:

    bind-key -t vi-copy Enter copy-pipe pbcopy

or, in version of tmux prior to 1.8 (which don't have the `copy-pipe` command):

    bind-key y run-shell "tmux save-buffer - | pbcopy"

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
workable solution when running on the local machine, we'll need something else
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

For Homebrew users, install by running (outside of all tmux sessions):

    brew install clipper # and follow the prompts...

For non-Homebrew users, a 64-bit binary archive prepared on OS X 10.8 Mountain
Lion can be downloaded from:

- https://wincent.com/products/clipper

Alternatively, if you have a working Go environment on your system you can do:

    go get github.com/wincent/clipper

Finally, if you want to do things manually, you can clone from the authoritative
Git repo and build manually (which again requires a working Go environment):

    git clone git://git.wincent.com/clipper.git
    cd clipper
    go build clipper.go

### Additional steps for non-Homebrew installs

If you plan to use Clipper as a launch agent you'll either need to put it
somewhere the system can find it (ie. at a location in the standard PATH, such
as under `/usr/bin/`) or update the included property list file to specify the
full path to the location where you installed the Clipper executable.

The following example shows how you would install the built Clipper executable
to `/usr/bin/` after cloning the repo and performing a build. It also shows how
you would set up Clipper as a launch agent and start it running:

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

A Homebrew installation can be reversed with:

    brew uninstall clipper

A manual launch agent installation can be reversed with:

    launchctl unload ~/Library/LaunchAgents/com.wincent.clipper.plist
    rm ~/Library/LaunchAgents/com.wincent.clipper.plist
    sudo rm /usr/bin/clipper

As before, note that you should only run `launchctl` outside of a tmux session.

To kill a manually-launched instance of Clipper, just hit Control+C in the
terminal where it is running.

## Configuring tmux

Now we can use a slight modification of our command from earlier. Assuming we
kept the standard listen address (127.0.0.1) and port (8377), we can use a
command like this to send the last-copied text whenever we hit our tmux prefix
key followed by `y`:

    bind-key y run-shell "tmux save-buffer - | nc localhost 8377"

In tmux 1.8 or later, we have access to the new `copy-pipe` command and can use
a single key binding to copy text into the tmux copy buffer and send it to
Clipper and therefore the system clipboard at the same time:

    bind-key -t vi-copy Enter copy-pipe "nc localhost 8377"

Here we're using netcat (`nc`) to send the contents of the buffer to the
listening Clipper agent.

## Configuring Vim

There is nothing inherent in Clipper that ties it to tmux. We can use it from
any process, including Vim.

For example, we can add a mapping to our `~/.vimrc` to send the last-yanked text
to Clipper by hitting `<leader>y`:

    nnoremap <leader>y :call system('nc localhost 8377', @0)<CR>

## Configuring Zsh (or Bash)

By setting up an alias:

    alias clip="nc localhost 8377"

you can conveniently get files and other content into your clipboard:

    cat example.txt | clip
    ls /etc | clip

## Configuring SSH

Again, assuming default address and port, we can use `-R` like this:

    ssh -R localhost:8377:localhost:8377 user@host.example.org

With this, a tmux process running on the remote host can use the same
configuration file, and our `run-shell` from above will send the buffer
contents to localhost:8377 on the remote machine, which will then be forwarded
back over the SSH connection to localhost:8377 on the local machine, where
Clipper is listening.

See the "Security" section below for some considerations.

To make this automated, entries can be set up in `.ssh/config`:

    Host host.example.org
      RemoteForward 8377 localhost:8377

With this, forwarding is automatically set up any time you run:

    ssh user@host.example.org

# Troubleshooting

## Fixing `remote port forwarding failed for listen port 8377`

This message can be emitted when the remote host you're connecting to already
has something bound to the requested port. If there is a competing service that
you can't move to another port, Clipper can be configured to use a different
port with the `--port` switch, described above.

Another reason you might see this warning is because an old or stale SSH daemon
is lingering from a prior connection attempt. The following example commands
show how you can detect the PID of such a process (in this example, 29517) and
kill it off:

    $ sudo netstat -antpl | grep 8377 # look for offending PID (29517) in output
    $ ps auxww | grep 29517           # confirm it's your old sshd process
    $ kill 29517                      # kill off old process
    $ ps auxww | grep 29517           # confirm that process is really gone

Consult the netstat man page for more details (supported options may vary
depending on the host operating system).

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

## 0.2 (2 November 2013)

- Documentation updates
- Updated sample plist to use UTF-8 encoding by default

## 0.1 (19 February 2013)

- Initial release
