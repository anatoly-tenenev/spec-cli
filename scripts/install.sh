#!/bin/sh

set -eu

OWNER="anatoly-tenenev"
REPO="spec-cli"
BINARY="spec-cli"
INSTALL_DIR="${SPEC_CLI_INSTALL_DIR:-${BINDIR:-$HOME/.local/bin}}"

log() {
	printf '%s\n' "$*" >&2
}

fail() {
	log "error: $*"
	exit 1
}

has_cmd() {
	command -v "$1" >/dev/null 2>&1
}

github_token() {
	if [ -n "${SPEC_CLI_GITHUB_TOKEN:-}" ]; then
		printf '%s' "${SPEC_CLI_GITHUB_TOKEN}"
		return
	fi
	if [ -n "${GITHUB_TOKEN:-}" ]; then
		printf '%s' "${GITHUB_TOKEN}"
		return
	fi
	if [ -n "${GH_TOKEN:-}" ]; then
		printf '%s' "${GH_TOKEN}"
		return
	fi
	printf '%s' ""
}

download_to_file() {
	url="$1"
	output="$2"
	token="$(github_token)"

	if has_cmd curl; then
		if [ -n "$token" ]; then
			curl -fsSL -H "Accept: application/vnd.github+json" -H "Authorization: Bearer $token" -o "$output" "$url"
			return
		fi
		curl -fsSL -H "Accept: application/vnd.github+json" -o "$output" "$url"
		return
	fi

	if has_cmd wget; then
		if [ -n "$token" ]; then
			wget -q --header="Accept: application/vnd.github+json" --header="Authorization: Bearer $token" -O "$output" "$url"
			return
		fi
		wget -q --header="Accept: application/vnd.github+json" -O "$output" "$url"
		return
	fi

	fail "curl or wget is required"
}

download_to_stdout() {
	url="$1"
	token="$(github_token)"

	if has_cmd curl; then
		if [ -n "$token" ]; then
			curl -fsSL -H "Accept: application/vnd.github+json" -H "Authorization: Bearer $token" "$url"
			return
		fi
		curl -fsSL -H "Accept: application/vnd.github+json" "$url"
		return
	fi

	if has_cmd wget; then
		if [ -n "$token" ]; then
			wget -q --header="Accept: application/vnd.github+json" --header="Authorization: Bearer $token" -O - "$url"
			return
		fi
		wget -q --header="Accept: application/vnd.github+json" -O - "$url"
		return
	fi

	fail "curl or wget is required"
}

detect_os() {
	uname_s="$(uname -s)"
	case "$uname_s" in
		Linux)
			printf '%s' "linux"
			;;
		Darwin)
			printf '%s' "darwin"
			;;
		*)
			fail "unsupported operating system: $uname_s"
			;;
	esac
}

detect_arch() {
	uname_m="$(uname -m)"
	case "$uname_m" in
		x86_64|amd64)
			printf '%s' "amd64"
			;;
		aarch64|arm64)
			printf '%s' "arm64"
			;;
		*)
			fail "unsupported architecture: $uname_m"
			;;
	esac
}

normalize_tag() {
	raw="${1:-latest}"
	trimmed="$(printf '%s' "$raw" | tr -d '[:space:]')"
	if [ -z "$trimmed" ] || [ "$trimmed" = "latest" ]; then
		printf '%s' "latest"
		return
	fi
	case "$trimmed" in
		v*)
			printf '%s' "$trimmed"
			;;
		*)
			printf 'v%s' "$trimmed"
			;;
	esac
}

resolve_latest_tag() {
	response="$(download_to_stdout "https://api.github.com/repos/$OWNER/$REPO/releases/latest")"
	tag="$(printf '%s\n' "$response" | sed -n 's/^[[:space:]]*"tag_name":[[:space:]]*"\([^"]*\)",\{0,1\}[[:space:]]*$/\1/p' | sed -n '1p')"
	if [ -z "$tag" ]; then
		fail "failed to resolve the latest release tag from GitHub API"
	fi
	printf '%s' "$tag"
}

checksum_value() {
	file="$1"
	if has_cmd shasum; then
		shasum -a 256 "$file" | awk '{print $1}'
		return
	fi
	if has_cmd sha256sum; then
		sha256sum "$file" | awk '{print $1}'
		return
	fi
	if has_cmd openssl; then
		openssl dgst -sha256 "$file" | awk '{print $NF}'
		return
	fi
	fail "shasum, sha256sum, or openssl is required for checksum verification"
}

warn_if_path_missing() {
	case ":$PATH:" in
		*":$INSTALL_DIR:"*)
			return
			;;
	esac
	log "warning: $INSTALL_DIR is not in PATH"
	log "add it to your shell profile, for example:"
	log "  export PATH=\"$INSTALL_DIR:\$PATH\""
}

REQUESTED_TAG="$(normalize_tag "${1:-latest}")"
OS="$(detect_os)"
ARCH="$(detect_arch)"

if [ "$REQUESTED_TAG" = "latest" ]; then
	TAG="$(resolve_latest_tag)"
else
	TAG="$REQUESTED_TAG"
fi

VERSION="${TAG#v}"
ARCHIVE="${BINARY}_${VERSION}_${OS}_${ARCH}.tar.gz"
CHECKSUMS_URL="https://github.com/$OWNER/$REPO/releases/download/$TAG/checksums.txt"
ARCHIVE_URL="https://github.com/$OWNER/$REPO/releases/download/$TAG/$ARCHIVE"

TMPDIR="$(mktemp -d 2>/dev/null || mktemp -d -t spec-cli-install)"
cleanup() {
	rm -rf "$TMPDIR"
}
trap cleanup EXIT INT TERM

log "installing $BINARY $TAG for $OS/$ARCH"
download_to_file "$ARCHIVE_URL" "$TMPDIR/$ARCHIVE"
download_to_file "$CHECKSUMS_URL" "$TMPDIR/checksums.txt"

EXPECTED_SUM="$(awk -v artifact="$ARCHIVE" '$2 == artifact { print $1 }' "$TMPDIR/checksums.txt" | sed -n '1p')"
if [ -z "$EXPECTED_SUM" ]; then
	fail "checksum entry for $ARCHIVE not found"
fi

ACTUAL_SUM="$(checksum_value "$TMPDIR/$ARCHIVE")"
if [ "$EXPECTED_SUM" != "$ACTUAL_SUM" ]; then
	fail "checksum mismatch for $ARCHIVE"
fi

mkdir -p "$TMPDIR/extract"
tar -xzf "$TMPDIR/$ARCHIVE" -C "$TMPDIR/extract"

SOURCE_BIN="$(find "$TMPDIR/extract" -type f -name "$BINARY" | sed -n '1p')"
if [ -z "$SOURCE_BIN" ]; then
	fail "failed to locate $BINARY inside $ARCHIVE"
fi

mkdir -p "$INSTALL_DIR"
TARGET="$INSTALL_DIR/$BINARY"
TEMP_TARGET="$TARGET.tmp.$$"
cp "$SOURCE_BIN" "$TEMP_TARGET"
chmod 0755 "$TEMP_TARGET"
mv "$TEMP_TARGET" "$TARGET"

VERSION_OUTPUT="$("$TARGET" version 2>/dev/null)" || fail "installed binary failed self-check"

log "installed to $TARGET"
warn_if_path_missing
log "spec-cli version output:"
printf '%s\n' "$VERSION_OUTPUT"
