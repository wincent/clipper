![Clipper](https://raw.github.com/wincent/clipper/master/logo.png)

# Overview

Clipper is a macOS "launch agent" &mdash; or Linux daemon &mdash; that runs in the background providing a service that exposes the local clipboard to tmux sessions and other processes running both locally and remotely.

# At a glance

    # macOS installation (using Homebrew; for non-Homebrew installs see below)
    brew install clipper # run this outside of a tmux session

    # Configuration for ~/.tmux.conf:

    # tmux >= 2.4: bind "Enter" in copy mode to both copy and forward to Clipper
    bind-key -T copy-mode-vi Enter send-keys -X copy-pipe-and-cancel "nc localhost 8377"

    # Or, if you are running Clipper on a UNIX domain socket:
    bind-key -T copy-mode-vi Enter send-keys -X copy-pipe-and-cancel "nc -U ~/.clipper.sock"

    # tmux >= 1.8 and < 2.4: bind "Enter" in copy mode to both copy and forward to Clipper
    bind-key -t vi-copy Enter copy-pipe "nc localhost 8377"

    # Or, if you are running Clipper on a UNIX domain socket:
    bind-key -t vi-copy Enter copy-pipe "nc -U ~/.clipper.sock"

    # tmux < 1.8: bind <prefix>-y to forward to Clipper
    bind-key y run-shell "tmux save-buffer - | nc localhost 8377"

    # Or, if you are running Clipper on a UNIX domain socket:
    bind-key y run-shell "tmux save-buffer - | nc -U ~/.clipper.sock"

    # Configuration for ~/.vimrc:
    # Bind <leader>y to forward last-yanked text to Clipper
    nnoremap <leader>y :call system('nc localhost 8377', @0)<CR>

    # Or, if you are running Clipper on a UNIX domain socket:
    nnoremap <leader>y :call system('nc -U ~/.clipper.sock', @0)<CR>

    # Configuration for ~/.bash_profile, ~/.zshrc etc:
    # Pipe anything into `clip` to forward it to Clipper
    alias clip="nc localhost 8377"

    # Or, if you are running Clipper on a UNIX domain socket:
    alias clip="nc -U ~/.clipper.sock"

    # Configuration for ~/.ssh/config:
    # Forward Clipper connection to remote host
    Host host.example.org
      RemoteForward 8377 localhost:8377

    # Or, if you are running Clipper on a UNIX domain socket:
    Host host.example.org
      RemoteForward /home/me/.clipper.sock /Users/me/.clipper.sock
      StreamLocalBindUnlink yes


# Problem

You're running tmux, possibly on a remote machine via ssh, and want to copy something using tmux copy mode into your local system clipboard.

You can hold down Option and click to make a selection, bypassing tmux and allowing you to copy straight to the system clipboard, but that won't work if you're using vertical splits (because the selection will operate across the entire width of the terminal, crossing the splits) or if you want to grab more than what is currently visible.

As a workaround for the vertical split problem, you can hold down Option + Command to make a rectangular (non-contiguous) selection, but that will grab trailing whitespace as well, requiring you to manually clean it up later. Again, it won't work if you want to grab more than what is currently visible.

As a result, you often find yourself doing a tiresome sequence of:

1. Copy a selection using tmux copy mode on a remote machine
2. Still on the remote machine, open a new, empty buffer in Vim
3. Enter Vim paste mode (`:set paste`)
4. Paste the tmux copy buffer into the Vim buffer
5. Write the file to a temporary location (eg. `:w /tmp/buff`)
6. From the local machine, get the contents of the temporary file into the local system clipboard with `ssh user@host cat /tmp/buff | pbcopy` or similar

# Solution

macOS comes with a `pbcopy` tool that allows you to get stuff into the clipboard from the command-line. `xclip` is an alternative that works on Linux. We've already seen this at work above. Basically, we can do things like `echo foo | pbcopy` to place "foo" in the system clipboard.

tmux has a few handy commands related to copy mode buffers, namely `save-buffer`, `copy-pipe` and `copy-pipe-and-cancel`, the availability of which depends on the version of tmux that you are running. With these, you can dump the contents of a buffer to standard out.

In theory, combining these elements, we can add something like this to our `~/.tmux.conf`:

    bind-key -T copy-mode-vi Enter send-keys -X copy-pipe-and-cancel pbcopy

or, in a version of tmux prior to 2.4 (which use `vi-copy` instead of `copy-mode-vi`, and `copy-pipe` instead of `copy-pipe-and-cancel`):

    bind-key -t vi-copy Enter copy-pipe pbcopy

or, in an even older version of tmux prior to 1.8 (which don't have the `copy-pipe-and-cancel` or `copy-pipe` commands):

    bind-key y run-shell "tmux save-buffer - | pbcopy"

In practice, this won't work on versions of macOS prior to 10.10 "Yosemite" or after 10.11 "El Capitan" &mdash; there was a brief lapse during those versions where it did work &mdash; because tmux uses the `daemon(3)` system call, which ends up putting it in a different execution context from which it cannot interact with the system clipboard. For (much) more detail, see:

- http://developer.apple.com/library/mac/#technotes/tn2083/_index.html

One workaround comes in the form of the `reattach-to-user-space` tool available here:

- https://github.com/ChrisJohnsen/tmux-MacOSX-pasteboard

This is a wrapper which allows you to launch a process and have it switch from the daemon execution context to the "user" context where access to the system clipboard is possible. The suggestion is that you can add a command like the following to your `~/.tmux.conf` to make this occur transparently whenever you open a new pane or window:

    set-option -g default-command "reattach-to-user-namespace -l zsh"

Despite the fact that the wrapper tool relies on an undocumented, private API, it is written quite defensively and appears to work pretty well. While this is a workable solution when running on the local machine, we'll need something else if we want things to work transparently both locally and remotely. This is where Clipper comes in.

Clipper is a server process that you can run on your local machine. It will listen on the network or on a UNIX domain socket for connections, and place any content that it receives into the system clipboard.

It can be set to run automatically as a "launch agent" in the appropriate execution context, which means you don't have to worry about starting it, and it will still have access to the system clipboard despite being "daemon"-like.

Through the magic of `ssh -R` it is relatively straightforward to have a shared tmux configuration that you can use both locally and remotely and which will provide you with transparent access to the local system clipboard from both places.

# Setup

## Installing

For Homebrew users, install by running (outside of all tmux sessions):

    brew install clipper # and follow the prompts...

Alternatively, if you have a working Go environment on your system you can do:

    go get github.com/wincent/clipper

Finally, if you want to do things manually, you can clone from the authoritative Git repo and build manually (which again requires a working Go environment):

    git clone git://git.wincent.com/clipper.git
    cd clipper
    go build

### Additional steps for non-Homebrew installs

If you plan to use Clipper as a launch agent you'll need to put it somewhere the system can find it (ie. at a location in the standard PATH, such as under `/usr/local/bin/`) and update the included property list file to specify the full path to the location where you installed the Clipper executable.

The following examples show how you would install the built Clipper executable to `/usr/local/bin/` after cloning the repo and performing a build. It also shows how you would set up Clipper as a launch agent (on macOS) or systemd service (on Linux) and start it running:

#### macOS example setup

    sudo cp clipper /usr/local/bin
    cp contrib/darwin/tcp-port/com.wincent.clipper.plist ~/Library/LaunchAgents/
    launchctl load -w -S Aqua ~/Library/LaunchAgents/com.wincent.clipper.plist

Note that these commands may fail to do the right thing inside of a tmux session under macOS. Run `launchctl` from outside of tmux, otherwise Clipper may find itself in the wrong execution context. Similarly, when running manually, either run Clipper outside of tmux or use the aforementioned `reattach-to-user-space` as a wrapper.

#### Linux example setup

    sudo cp clipper /usr/local/bin
    cp contrib/linux/systemd-service/clipper.service ~/.config/systemd/user
    systemctl --user daemon-reload
    systemctl --user enable clipper.service
    systemctl --user start clipper.service

#### Manual setup

Alternatively, if you'd like to run Clipper manually, you can do so with:

    ./clipper [--address=IP_ADDR|UNIX_SOCKET] \
              [--port=PORT] \
              [--logfile=LOGFILE] \
              [--config=CONFIG_FILE]

## Uninstalling

A Homebrew installation can be reversed with:

    brew uninstall clipper

A manual launch agent installation can be reversed with the following (and as before, note that you should probably only run `launchctl` outside of a tmux session):

    launchctl unload ~/Library/LaunchAgents/com.wincent.clipper.plist
    rm ~/Library/LaunchAgents/com.wincent.clipper.plist
    sudo rm /usr/local/bin/clipper

On Linux:

    systemctl --user stop clipper.service
    systemctl --user disable clipper.service
    sudo rm /usr/local/bin/clipper

To kill a manually-launched instance of Clipper, just hit Control+C in the terminal where it is running.

## Configuring Clipper

As previously noted, Clipper supports a number of command line options, which you can see by running `clipper -h`:

```
Usage of clipper:
  -a string
        address to bind to (default loopback interface)
  -address string
        address to bind to (default loopback interface)
  -c string
        path to (JSON) config file (default "~/.clipper.json")
  -config string
        path to (JSON) config file (default "~/.clipper.json")
  -e string
        program called to write to clipboard (default "pbcopy")
  -executable string
        program called to write to clipboard (default "pbcopy")
  -f string
        arguments passed to clipboard executable
  -flags string
        arguments passed to clipboard executable
  -h    show usage information
  -help
        show usage information
  -l string
        path to logfile (default "~/Library/Logs/com.wincent.clipper.log")
  -logfile string
        path to logfile (default "~/Library/Logs/com.wincent.clipper.log")
  -p int
        port to listen on (default 8377)
  -port int
        port to listen on (default 8377)
  -v    show version information
  -version
        show version information
```

The defaults shown above apply on macOS. Run `clipper -h` on Linux to see the defaults that apply there.

You can explicitly set these on the command line, or in the plist file if you are using Clipper as a launch agent. Clipper will also look for a configuration file in JSON format at `~/.clipper.json` (this location can be overidden with the `--config`/`-c` options) and read options from that. The following options are supported:

- `address`
- `executable`
- `flags`
- `logfile`
- `port`

Here is a sample `~/.clipper.json` config file:

```
{
  "address": "~/.run/clipper.sock",
  "logfile": "~/Library/Logs/clipper.log"
}
```

Note that explicit command line options — including options supplied via a plist — trump options read from a config file.

### `--address`

Specifies the address on which the Clipper daemon will listen for connections. Defaults to "localhost", and listens on both IPv4 and IPv6 addresses, when available. This is a reasonable default, but you may wish to set it to a filesystem path instead in order to have Clipper create a UNIX domain socket at that location and listen on that instead (for better security: see "Security" below for more). Or perhaps you would like to *only* listen on IPv4 *or* IPv6, in which case you would use "127.0.0.1" or "[::1]" respectively (the square brackets around the IPv6 address are needed by the Go networking libraries in order to disambiguate the colons in the address from colons used to separate it from the port number). Note that if you see an error of the form "too many colons in address", it is likely that you have forgotten to wrap the IPv6 address in surrounding brackets.

### `--port`

The port on which the Clipper daemon will accept TCP connections. Defaults to 8377. You may wish to set this to some other value, to avoid colliding with another service (or Clipper user) on your system using that port, or because you want some tiny extra dose of security through obscurity.

Note that if you use the `--address` option to make Clipper use a UNIX domain socket, then setting the `--port` has no useful effect.

See the "Security" section below for more.

### `--logfile`

Unsurprisingly, controls where the Clipper daemon logs its output. Defaults to  "~/Library/Logs/com.wincent.clipper.log".

As an example, you could disable all logging by setting this to "/dev/null".

### `--executable`

The executable used to place content on the clipboard (defaults to `pbcopy` on macOS and `xclip` on Linux).

### `--flags`

The flags to pass to the `executable` (defaults to `-selection clipboard` on Linux and nothing on macOS).

## Configuring tmux

Now we can use a slight modification of our command from earlier. Assuming we kept the standard listen address (127.0.0.1) and port (8377), we can use a command like this to send the last-copied text whenever we hit our tmux prefix key followed by `y`; here we're using netcat (`nc`) to send the contents of the buffer to the listening Clipper agent:

    bind-key y run-shell "tmux save-buffer - | nc localhost 8377"

If we instead configured Clipper to listen on a UNIX domain socket at `~/.clipper.sock`, then we could do something like:

    bind-key y run-shell "tmux save-buffer - | nc -U ~/.clipper.sock"

In tmux 1.8 to 2.3, we have access to the new `copy-pipe` command and can use a single key binding to copy text into the tmux copy buffer and send it to Clipper and therefore the system clipboard at the same time:

    bind-key -t vi-copy Enter copy-pipe "nc localhost 8377"

Or, for a UNIX domain socket at `~/.clipper.sock`:

    bind-key -t vi-copy Enter copy-pipe "nc -U ~/.clipper.sock"

In tmux 2.4 and above, we would use:

    bind-key -T copy-mode-vi Enter send-keys -X copy-pipe-and-cancel "nc localhost 8377"

Or, for a UNIX domain socket at `~/.clipper.sock`:

    bind-key -T copy-mode-vi Enter send-keys -X copy-pipe-and-cancel "nc -U ~/.clipper.sock"

## Configuring Vim

There is nothing inherent in Clipper that ties it to tmux. We can use it from any process, including Vim.

For example, we can add a mapping to our `~/.vimrc` to send the last-yanked text to Clipper by hitting `<leader>y`:

    nnoremap <leader>y :call system('nc localhost 8377', @0)<CR>

Equivalently, we could do the same for a Clipper daemon listening on a UNIX domain socket at `~/.clipper.sock` with:

    nnoremap <leader>y :call system('nc -U ~/.clipper.sock', @0)<CR>

For the lazy, this functionality plus a `:Clip` command is made available as a [separate Vim plug-in](https://github.com/wincent/vim-clipper) called "vim-clipper".

## Configuring Zsh (or Bash)

By setting up an alias like:

    alias clip="nc localhost 8377"

or (in the case of Clipper listening on a UNIX domain socket at `~/.clipper.sock`):

    alias clip="nc -U ~/.clipper.sock"

you can conveniently get files and other content into your clipboard:

    cat example.txt | clip
    ls /etc | clip

## Configuring SSH

Again, assuming default address and port, we can use `-R` like this:

    ssh -R localhost:8377:localhost:8377 user@host.example.org

Or, in the case of a UNIX domain socket at `~/.clipper.sock` and a sufficiently recent version of OpenSSH (version 6.7 or above):

    # Assuming a local socket on macOS in $HOME at /Users/me/.clipper.sock
    # and a remote Linux machine with $HOME is in /home rather than /Users:
    ssh -R/home/me/.clipper.sock:/Users/me/.clipper.sock \
        -o StreamLocalBindUnlink=yes \
        host.example.org

With this, a tmux process running on the remote host can use the same configuration file, and our `run-shell` from above will send the buffer contents to localhost:8377 (or the UNIX domain socket) on the remote machine, which will then be forwarded back over the SSH connection to localhost:8377 (or the UNIX domain socket) on the local machine, where Clipper is listening.

See the "Security" section below for some considerations.

To make this automated, entries can be set up in `.ssh/config`:

    # TCP forwarding:
    Host host.example.org
      RemoteForward 8377 localhost:8377

    # UNIX domain socket forwarding:
    Host host.example.org
      RemoteForward /home/me/.clipper.sock:/Users/me/.clipper.sock
      StreamLocalBindUnlink yes

With this, forwarding is automatically set up any time you run:

    ssh user@host.example.org

This works particularly well if you use the `ControlMaster`, `ControlPath` and `ControlPersist` settings described in the `ssh_config` man page. These allow you to set up a single forward, and re-use it each time you connect, even if you have multiple concurrent connnections to a given server. An example `~/.ssh/config` configuration that would give you this for all hosts would be something like:

    Host *
      ControlMaster auto
      ControlPath ~/.ssh/%r@%h:%p
      ControlPersist 240

## Configuring Mosh

[Mosh](http://mosh.mit.edu/) is an alternative to SSH that aims to be a superior "mobile shell" than SSH. It is designed to handle intermittent connectivity, and provides relatively intelligent local echoing of line editing commands, but it [doesn't yet support any kind of port forwarding](https://github.com/mobile-shell/mosh/issues/499) (as of the current release, which is version 1.2.5 at the time of writing).

One way to use Clipper with Mosh is to use Mosh for interactive editing but keep using SSH for port forwarding. For example, just say you want to connect to a remote machine with the alias "sandbox", you could have entries like this in your `~/.ssh/config`:

    Host sandbox
      ControlMaster no
      ControlPath none
      Hostname sandbox.example.com

    Host sandbox-clipper
      ControlMaster no
      ControlPath none
      ExitOnForwardFailure yes
      Hostname sandbox.example.com
      RemoteForward 8377 localhost:8377

With this set-up, you can set up the tunnel with:

    ssh -N -f sandbox

SSH will connect to the server, set up the port forwarding and then go into the background.

Then connect using Mosh (it will respect the settings in your `~/.ssh/config` file because it uses SSH to bootstrap new connections):

    mosh sandbox

You could also set up a shell alias to make setting up the Clipper tunnel more convenient; for example:

    alias clip-sandbox='ssh -N -f sandbox'

You should only need to re-run this command if the connection is interrupted for some reason.

# Troubleshooting

## Fixing `remote port forwarding failed for listen port 8377`

This message can be emitted when the remote host you're connecting to already has something bound to the requested port. If there is a competing service that you can't move to another port, Clipper can be configured to use a different port with the `--port` switch, described above.

Another reason you might see this warning is because an old or stale SSH daemon is lingering from a prior connection attempt. The following example commands show how you can detect the PID of such a process (in this example, 29517) and kill it off:

    $ sudo netstat -antpl | grep 8377 # look for offending PID (29517) in output
    $ ps auxww | grep 29517           # confirm it's your old sshd process
    $ kill 29517                      # kill off old process
    $ ps auxww | grep 29517           # confirm that process is really gone

For the bold and lazy, you can simply kill off all `sshd` process belonging to your user with:

    $ killall -u $USER sshd

Do not do this as root, as you will lock yourself out of your server.

Consult the netstat man page for more details (supported options may vary depending on the host operating system).

## Fixing `remote port forwarding failed` when using UNIX domain sockets

Just as with TCP port forwarding, forwarding can fail when using UNIX domain sockets if a stale socket doesn't get automatically cleaned up (or overwritten, as *should* be the case when you use `StreamLocalBindUnlink=yes`).

In this case, the fix is to remove the stale socket on the remote host. For example, assuming a socket in `$HOME/.clipper.sock` on the remote host, `$HOST`:

     $ ssh $HOST rm .clipper.sock

## Fixing delays when sending data to Clipper via `nc`

It's [been reported](https://github.com/wincent/clipper/issues/11) that on some systems sending data to Clipper via `nc` may be affected by an annoying delay. Reportedly, adding the `-N` switch to `nc` may address this issue.

# "Reverse" Clipper

Clipper helps you get content into your local clipboard from other, possibly remote, processes. To send content in the other direction, just paste normally. Note that to make this pleasant in an environment like Vim, you may want to set up bracketed paste mode; see [my dotfiles for an example](https://github.com/wincent/wincent/blob/3b0b2950cdcb09d23c87f0167c207d8c837cb1b2/.vim/plugin/term.vim#L93-114) of how this can be done.

# Security

At the moment, Clipper doesn't employ any authentication. It does, by default, listen only on the loopback interface, which means that random people on your network won't be able to connect to it. People with access to your local machine, however, will have access; they can push content into your clipboard even if they can't read from it.

This may be fine on a single-user machine, but when you start using `ssh -R` to expose your Clipper instance on another machine you're evidently increasing your surface area. In order to mitigate this risk, you can configure Clipper to listen only on a UNIX domain socket, and place that socket in a location where the file-system permissions prevent others from accessing it.

Most SSH systems are configured to use restrictive permissions on forwarded socket files (unless overridden; see the documentation for `StreamLocalBindMask` in `man ssh_config`), but you may wish to place the socket in a non-shared location like `~/.clipper.sock` rather than a shared one like `/tmp/clipper.sock` in any case.

# Authors

Clipper is written and maintained by Greg Hurrell <greg@hurrell.net>.
Other contributors that have submitted patches include, in alphabetical order:

  Jannis Hermanns
  Nelson Fernandez

# Development

The official Clipper source code repo is at:

- http://git.wincent.com/clipper.git

Mirrors exist at:

- https://github.com/wincent/clipper
- https://gitlab.com/wincent/clipper
- https://bitbucket.org/ghurrell/clipper

Patches are welcome via the usual mechanisms (pull requests, email, posting to the project issue tracker etc).

# Website

The official website for Clipper is:

- https://github.com/wincent/clipper

Bug reports should be submitted to the issue tracker at:

- https://github.com/wincent/clipper/issues

# License

Copyright 2013-present Greg Hurrell. All rights reserved.

Redistribution and use in source and binary forms, with or without modification, are permitted provided that the following conditions are met:

1. Redistributions of source code must retain the above copyright notice, this list of conditions and the following disclaimer.
2. Redistributions in binary form must reproduce the above copyright notice, this list of conditions and the following disclaimer in the documentation and/or other materials provided with the distribution.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDERS OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

# History

## 0.4.2 (14 April 2017)

- Fix binding to all interfaces instead of just the loopback interface when no address is provided ([#9](https://github.com/wincent/clipper/issues/9)).

## 0.4.1 (13 December 2016)

- Create socket with user-only permissions, for better security.

## 0.4 (28 November 2016)

- Linux support via `xclip` instead of `pbcopy` (patch from Nelson Fernandez).
- Added `--executable` and `--flags` options.
- On dual-stack systems, listen on both IPv4 and IPv6 loopback interfaces by default.

## 0.3 (3 June 2016)

- Add support for listening over a UNIX domain socket.
- Add support for reading options from a config file (`--config`/`-c` switch).

## 0.2 (2 November 2013)

- Documentation updates.
- Updated sample plist to use UTF-8 encoding by default.

## 0.1 (19 February 2013)

- Initial release.
