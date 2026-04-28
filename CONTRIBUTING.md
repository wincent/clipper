# Contributing

## Cutting a new release

First:

1. Set the `version` string in `clipper.go`.
2. Update the "History" section in the `README.md`.
2. Commit this using a message like `chore: prepare for $VERSION release`.

Then, tag, build, and upload.

As a convenience, there's a `Makefile` target for this:

```
make all VERSION=3.0.0
```

Which is roughly equivalent to:

```
git tag -s -m '3.0.0 release' 3.0.0
make build
make archive
make upload
```

After preparing the release, push the code and update the release notes:

```
git push all --follow-tags # "all" is a remote I have that pushes to all my mirrors at once.

# Opens https://github.com/wincent/clipper so that you can publish release notes.
git hub open
```

Finally, reset the `version` string in `clipper.go` back to "main", and start a new section under "History" in the docs, commit that, and push.

## Updating the Homebrew formula

This used to be a manual process, but now a bot takes care of it. I discovered this with my last attempt to bump the formula; there was a `make brew` target for this, but running manually I observed the following:

```console
$ brew bump --open-pr clipper
Warning: bump is a developer command, so Homebrew's
developer mode has been automatically turned on.
To turn developer mode off, run:
  brew developer off

Fetching gem metadata from https://rubygems.org/.......
...
Bundle complete! 44 Gemfile dependencies, 14 gems now installed.
Bundled gems are installed into `../../../../opt/homebrew/Library/Homebrew/vendor/bundle`
1 installed gem you directly depend on is looking for funding.
  Run `bundle fund` for details
Error: These formulae are not in any locally installed taps!

  clipper

You may need to run `brew tap` to install additional taps.
```

So, I tried tapping `homebrew/core`, was told that I was holding it wrong, so tried again and pulled down a rather large repo, only to find it was unnecessary:

```console
$ brew tap homebrew/core
Error: Tapping homebrew/core is no longer typically necessary.
Add --force if you are sure you need it for contributing to Homebrew.
$ brew tap homebrew/core --force
==> Tapping homebrew/core
Cloning into '/opt/homebrew/Library/Taps/homebrew/homebrew-core'...
remote: Enumerating objects: 3399066, done.
remote: Counting objects: 100% (84/84), done.
remote: Compressing objects: 100% (65/65), done.
remote: Total 3399066 (delta 59), reused 20 (delta 19), pack-reused 3398982 (from 3)
Receiving objects: 100% (3399066/3399066), 1.08 GiB | 34.31 MiB/s, done.
Resolving deltas: 100% (2624569/2624569), done.
Tapped 5 commands and 8318 formulae (8,857 files, 1.3GB).
$ brew bump --open-pr clipper
==> clipper
Formula is autobumped so will have bump PRs opened by BrewTestBot every ~3 hours.
```

Cleaning up the mess:

```console
$ brew untap homebrew/core
Untapping homebrew/core...
Untapped 5 commands and 8318 formulae (8,857 files, 1.3GB).
$ brew developer
Developer mode is enabled because a developer command or `brew developer on` was run.
`brew update` will update to the latest commit on the `main` branch.
$ brew developer off
```
