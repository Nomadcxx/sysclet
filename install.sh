#!/bin/sh
# Install sysclet for the current user.

set -eu

release_base_url=https://github.com/Nomadcxx/sysclet/releases/latest/download
tmpdir=

fail() {
	printf 'sysclet installer: %s\n' "$1" >&2
	exit 1
}

fallback_to_go() {
	reason=$1
	if command -v go >/dev/null 2>&1; then
		printf '%s\n' "$reason" >&2
		go run github.com/Nomadcxx/sysclet/cmd/installer@latest </dev/tty
		exit $?
	fi
	printf '%s\n' "$reason" >&2
	printf '%s\n' 'Go 1.21+ is required for this fallback. Install Go, then run this script again.' >&2
	printf '%s\n' 'See https://go.dev/doc/install' >&2
	exit 1
}

if [ "$(id -u)" -eq 0 ]; then
	fail 'run this installer as your user, without sudo'
fi

if [ ! -t 1 ] || [ ! -r /dev/tty ]; then
	fail 'run this installer from an interactive terminal'
fi

case "$(uname -m)" in
	x86_64) arch=amd64 ;;
	aarch64) arch=arm64 ;;
	*) fallback_to_go "No release installer binary is available for $(uname -m)." ;;
esac

if ! command -v curl >/dev/null 2>&1 || ! command -v sha256sum >/dev/null 2>&1; then
	fallback_to_go 'curl and sha256sum are required to install the release binary.'
fi

tmpdir=$(mktemp -d) || fail 'create temporary directory'
trap 'rm -rf "$tmpdir"' 0

manifest=$tmpdir/SHA256SUMS
if ! http_code=$(curl -sSL --max-time 300 -o "$manifest" -w '%{http_code}' "$release_base_url/SHA256SUMS"); then
	fail 'download release checksums'
fi
case "$http_code" in
	200) ;;
	404) fallback_to_go 'This release has no checksum manifest.' ;;
	*) fail "download release checksums: HTTP $http_code" ;;
esac

asset=sysclet-installer-linux-$arch
if checksum=$(awk -v asset="$asset" '
	function valid_digest(value) {
		return length(value) == 64 && value !~ /[^[:xdigit:]]/
	}
	NF == 0 { next }
	{
		entries++
		if (NF != 2 || !valid_digest($1)) {
			invalid = 1
			next
		}
		name = $2
		sub(/^\*/, "", name)
		if (name == asset) {
			matches++
			digest = tolower($1)
		}
	}
	END {
		if (invalid || entries == 0) exit 3
		if (matches == 0) exit 1
		if (matches != 1) exit 2
		print digest
	}' "$manifest"); then
	:
else
	awk_status=$?
	if [ "$awk_status" -eq 1 ]; then
		fallback_to_go 'This release has no installer binary for this architecture.'
	fi
	fail 'invalid SHA256SUMS manifest'
fi

installer=$tmpdir/$asset
if ! http_code=$(curl -sSL --max-time 300 -o "$installer" -w '%{http_code}' "$release_base_url/$asset"); then
	fail "download $asset"
fi
[ "$http_code" = 200 ] || fail "download $asset: HTTP $http_code"

if ! (cd "$tmpdir" && printf '%s  %s\n' "$checksum" "$asset" | sha256sum -c >/dev/null); then
	fail "SHA-256 verification failed for $asset"
fi
chmod 755 "$installer" || fail "make $asset executable"
"$installer" </dev/tty
