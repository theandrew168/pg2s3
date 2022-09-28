#!/bin/sh
set -e

# Create pg2s3 group (if it doesn't exist)
if ! getent group pg2s3 >/dev/null; then
    groupadd --system pg2s3
fi

# Create pg2s3 user (if it doesn't exist)
if ! getent passwd pg2s3 >/dev/null; then
    useradd                                 \
        --system                            \
        --gid pg2s3                         \
        --shell /usr/sbin/nologin           \
        --comment "pg2s3 database backups"  \
        pg2s3
fi

# Update config file permissions (idempotent)
chown root:pg2s3 /etc/pg2s3.conf
chmod 0640 /etc/pg2s3.conf

# Reload systemd to pickup pg2s3.service
systemctl daemon-reload

# Restart if already running
if systemctl is-active pg2s3 >/dev/null
then
    systemctl restart pg2s3 >/dev/null
fi
