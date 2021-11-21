#!/bin/sh
set -e

if [ -d /run/systemd/system ] && [ "$1" = remove ]; then
	deb-systemd-invoke stop pg2s3.service >/dev/null || true
fi
