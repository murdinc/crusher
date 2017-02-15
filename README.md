# crusher
> Configuration Management Tool

[![Build Status](https://travis-ci.org/murdinc/crusher.svg)](https://travis-ci.org/murdinc/crusher)

## Intro
**crusher** is a minimalist configuration-management tool. It aims to provide much of the flexibility of the available off-the-shelf options, with as little overhead as possible.

## Requirements
To use this tool, you must have the ability to ssh to a host, have permission to write to the servers temp directory, and run commands with elevated privileges.

## Use Cases
**crusher** can be used as both a centralized and distributed tool for setting up new servers.

- Centralized:
**crusher** manages a list of remote servers that it saves in your users home directory (`~/.crusher`). The `remote-configure` command targets remote servers based on name or spec, and runs configuration tasks on all of them asynchronously.

- Distributed:
compile **crusher** and put it at the base of a git repo containing your spec files. New servers can be launched with a script to check out your repo and run crushers `local-configure` command to configure themselves.

## Installation
```
curl -s http://dl.sudoba.sh/get/crusher | sh
```

## Commands
The available commands can be printed by running `$ crusher` or `$ crusher --help`:
```
$ crusher --help
crusher - 1.0

Usage:
   crusher [global options] command [command options] [arguments...]

Commands:
   list-servers, l			List all configured remote servers
   remote-configure, rc		Configure one or many remote servers
   local-configure, lc		Configure this local machine with a given spec
   add-server, a			Add a new remote server to the config
   delete-server, d			Delete a remote server from the config
   available-specs, s		List all available specs
   show-spec, ss			Show what a given spec will build
   help, h					Shows a list of commands or help for one command

Global Options:
   --version, -v			print the version
   --help, -h				show help
```
Help for specific commands can be printed by passing the `--help` flag to the command
```
$ crusher remote-configure --help
Usage:
   remote-configure search [--flags]

Arguments:
   search					The server or spec group to remote configure

Flags:


Example:
   crusher remote-configure hello_world
```

## What's a Spec?
A `.spec` file (short for specification), along with its `config` and `content` folders, contain the building blocks of a server configuration. Specs contain a list of packages to install, configuration and content files along with their destinations, and commands to run during the configuration job.

Specs can require other specs, to link smaller building blocks into more complex configurations. Check out `hello_word.spec` in the [example-specs](https://github.com/murdinc/crusher/tree/master/example-specs) folder for a simple example.

By default, **crusher** will look for Specs in the following directories, in order, overwriting previously found specs with the same name:

1. $GOPATH/src/github.com/murdinc/crusher/example-specs/
2. /etc/crusher/specs/
3. ~/crusher/specs/
4. ./specs/

Running a Spec against a server looks a little like this:

1. Install all required packages from the spec and all of its required specs
2. Transfer/Copy all of the config and content files of the spec and its required specs
3. Run post-config commands of the spec and its required specs

## Hello World Example:
This example spec installs nginx and php5-fpm, and serves "Hello World!" from port 80.

1. Run the `remote-configure` command, passing in `hello_world` as the search option:

  `$ crusher remote-configure hello_world`

2. **crusher** knows you haven't configured any remote servers yet, and asks you to set some up first:

	![setup](screenshots/setup.png)

3. Run the same command again, and this time it will find the servers you have set up, and run the spec configurations against them:

	![remote-configure](screenshots/remote-configure.png)

4. Your servers should now be serving "Hello World!" from port 80.

## Crusher?
This was a code challenge, and I for some reason immediately thought of the scenario of Wesley Crusher adding a set of commands to the ships computers to automate the set-up new warp engines. Also I needed to call it something.

## Tests
There is a very basic test for each of the sub-packages, I hope to expand those after moving the Jobs code into its own sub-package. Running `$ go test github.com/murdinc/crusher/...` will run all tests for this project.

```
$ go test github.com/murdinc/crusher/...
ok  	github.com/murdinc/crusher	0.010s
ok  	github.com/murdinc/crusher/config	0.011s
ok  	github.com/murdinc/crusher/servers	0.011s
ok  	github.com/murdinc/crusher/specr	0.013s
```

## Roadmap / Not yet implemented
- SSH Key Authentication (still needs callback func)
- Support for all flavors of servers (not just Ubuntu)
- Finer control over tasks run / incremental changes
- Check and rollback of config changes
- Pull Jobs into its own package as an interface to help declutter the other controllers
- More Tests!
- Lots of sanity checking still needed
- Tab completion
