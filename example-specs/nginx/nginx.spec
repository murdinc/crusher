# NGINX SPEC FILE
# /////////////////////////////////////////////////

NAME = nginx

VERSION = 1
REQUIRES =

[PACKAGES]
	apt_get = nginx

[CONFIGS]
	debian_root = "/etc/"

[COMMANDS]

	post = "sudo service nginx start, sudo service nginx reload"


