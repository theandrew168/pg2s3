#!/bin/sh
set -e

if [ "$1" = "configure" ]; then
	# Add user and group
	if ! getent group pg2s3 >/dev/null; then
		groupadd --system pg2s3
	fi
	if ! getent passwd pg2s3 >/dev/null; then
		useradd --system \
			--gid pg2s3 \
			--create-home \
			--home-dir /var/lib/pg2s3 \
			--shell /usr/sbin/nologin \
			--comment "pg2s3 database backups" \
			pg2s3
	fi
fi

if [ "$1" = "configure" ] || [ "$1" = "abort-upgrade" ] || [ "$1" = "abort-deconfigure" ] || [ "$1" = "abort-remove" ] ; then
	# This will only remove masks created by d-s-h on package removal.
	deb-systemd-helper unmask pg2s3.service >/dev/null || true

	# was-enabled defaults to true, so new installations run enable.
	if deb-systemd-helper --quiet was-enabled pg2s3.service; then
		# Enables the unit on first installation, creates new
		# symlinks on upgrades if the unit file has changed.
		deb-systemd-helper enable pg2s3.service >/dev/null || true
		deb-systemd-invoke start pg2s3.service >/dev/null || true
	else
		# Update the statefile to add new symlinks (if any), which need to be
		# cleaned up on purge. Also remove old symlinks.
		deb-systemd-helper update-state pg2s3.service >/dev/null || true
	fi

	# Restart only if it was already started
	if [ -d /run/systemd/system ]; then
		systemctl --system daemon-reload >/dev/null || true
		if [ -n "$2" ]; then
			deb-systemd-invoke try-restart pg2s3.service >/dev/null || true
		fi
	fi
fi
