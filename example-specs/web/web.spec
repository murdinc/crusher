# WEBSERVER SPEC FILE
# /////////////////////////////////////////////////

NAME = webserver

VERSION = 1
REQUIRES = nginx, php

[PACKAGES]
	# NONE

[CONFIGS]
	# NONE

[CONTENT]
	source = spec
	# source = git
	# git_command = git clone ...
	debian_root = "/var/lib/html/"

[COMMANDS]
	post = "sudo service nginx reload"

