# PHP5 SPEC FILE
# /////////////////////////////////////////////////

NAME = php

VERSION = 1
REQUIRES =

[PACKAGES]
	apt_get = php7.0-fpm

[CONFIGS]
	debian_root = "/etc/"

[COMMANDS]

	post = "sudo service php7.0-fpm start"