#!/bin/sh
# Install sysclet for the current user. Requires Go 1.20+ and network access.

set -eu

if [ "$(id -u)" -eq 0 ]; then
	printf '%s\n' 'Run this installer as your user, without sudo.' >&2
	exit 1
fi

if ! command -v go >/dev/null 2>&1; then
	printf '%s\n' 'Go 1.20+ is required. Install Go, then run this script again.' >&2
	printf '%s\n' 'See https://go.dev/doc/install' >&2
	exit 1
fi

if [ ! -t 1 ] || [ ! -r /dev/tty ]; then
	printf '%s\n' 'Run this installer from an interactive terminal.' >&2
	exit 1
fi

exec go run github.com/Nomadcxx/sysclet/cmd/installer@latest </dev/tty
