# WEBSERVER SPEC FILE
# /////////////////////////////////////////////////

NAME = hello_world

VERSION = 1
REQUIRES = nginx, php

[PACKAGES]
	# NONE

[CONFIGS]
	debian_root = "/etc/"

[CONTENT]
	source = spec
	# source = git
	# git_command = git clone ...
	debian_root = "/usr/share/nginx/html/"

[COMMANDS]
	pre = "sudo pkill -fx 'nc -k -l 0.0.0.0 80' & sudo lsof -t +L1 /tmp | sudo xargs kill -9  & sudo resolvconf -u"
	# kill netcat on post 80, flush unlinked tmp files, update resolv.conf

	post = "sudo service nginx start & sudo service php5-fpm start & sudo iptables-restore < /etc/iptables.hello_world & sudo service nginx reload"
	# start nginx and php5-fpm incase they arent running, set iptables rules, reload nginx configs

