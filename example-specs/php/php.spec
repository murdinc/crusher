# PHP SPEC FILE
# /////////////////////////////////////////////////

NAME = php

VERSION = 1
REQUIRES =

[PACKAGES]
	apt_get = php7.0-fpm, php7.0-cli, php7.0-curl, php7.0-gd, php7.0-intl php7.0-mysql, php-memcache, php7.0-xml, php7.0-mbstring, php7.0-mcrypt, php7.0-xmlrpc

[CONFIGS]
	debian_root = "/etc/"
	skip_interpolate = true

[COMMANDS]
	pre = "sudo chmod -R 775 ./specs/php/scripts/, sudo add-apt-repository -y ppa:ondrej/php"
	post = "sudo service php7.0-fpm restart"
