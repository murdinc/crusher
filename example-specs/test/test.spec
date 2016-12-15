# TEST SPEC FILE
# /////////////////////////////////////////////////

NAME = test

VERSION = 1
REQUIRES =

[PACKAGES]
	# NONE

[CONFIGS]

[CONTENT]
	source = spec
	# source = git
	# git_command = git clone ...
	debian_root = "/usr/share/nginx/html/"

[COMMANDS]

	pre = "who"

	post = "uname -a"

