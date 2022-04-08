# trellofs

The Trello POSIX Filesystem, powered by FUSE, that everyone always knew they
did not want or need.

## The Main Questions

### Why?

Because I wanted to learn Go. And what better way to do it if not by doing
something fun?

### Does it work?

To some extent, yes. At time of writing, the filesystem is read-only. One is
able to obtain information all the way to the card level. Cards are represented
as directories, and their fields are presented to the user as files. Not all
fields are handled though, including comments and attachments.

### Will it be further developed?

Probably not. I've had my fun with it, and it's unlikely I'll find a particular
purpose for it, or that I'll find a use-case to include it in my day-to-day
life. As such, motivation to continue developing it is pretty low.

Should there be interest on folks though, I'd be more than happy to develop
a few more things, or to review and merge pull requests.


## Running

Admitedly I have little experience with Go projects, so my understanding on
how to run this code is poor. As I understand, it should be a matter of
running `go build -o trellofs <args>` on this repository's `src` directory.
Alternatively, `go run trellofs <args>` on the same directory.

The program will require two arguments: `--mount /path/to/dir` and
`--config /path/to/config.json`. The former is the mount point for the
filesystem; the latter the configuration providing credentials to
access Trello's API.

Once the filesystem is mounted, it should just be a matter of using the
specified mountpoint as any other filesystem.


## Obtaining Credentials & Configuration

Given my purpose was not to figure out how to have a CLI application being
able to authenticate over OAuth 2.0, I went with the easy way: grab an API
key and an API token from Trello's developer tools. That worked fine for
the purposes of this project.

Obtaining these tokens is achieved by heading over to
[Trello](https://trello.com/app-key) and following their instructions.

A sample configuration file has been supplied in this repository. Fill it
with the appropriate values, and it _should_ work.


## Contributing

Given how unlikely it is for anyone to ever contribute to this project, we'll
keep contribution guidelines pretty lax. The only things we'll require is a
[DCO](https://developercertificate.org/), by signing off the work with a
`Signed-Off-By: My Name <me@foo.bar>` line on the work's commits; and having
signed, and verified, commits.

## LICENSE

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

See the LICENSE file for more information.

