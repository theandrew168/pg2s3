#!/bin/sh
set -e

if [ -d /run/systemd/system ]; then
	systemctl --system daemon-reload >/dev/null || true
fi

if [ "$1" = "remove" ]; then
	if [ -x "/usr/bin/deb-systemd-helper" ]; then
		deb-systemd-helper mask pg2s3.service >/dev/null || true
	fi
fi

if [ "$1" = "purge" ]; then
	if [ -x "/usr/bin/deb-systemd-helper" ]; then
		deb-systemd-helper purge pg2s3.service >/dev/null || true
		deb-systemd-helper unmask pg2s3.service >/dev/null || true
	fi
fi
