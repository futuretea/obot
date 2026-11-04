#!/usr/bin/env bash
# verify-cli-install.sh — verify the /install-cli page install instructions stay
# verbatim-executable against the built distribution archives.
# Keep the command sequence below in sync with ui/user/src/routes/install-cli/+page.svelte.
#
# For every archive it simulates a first-time user in a clean environment:
# download (local copy), checksum verify, unpack, install onto PATH (into a
# temporary stand-in directory), then run the bare `uni` command.
# Host-incompatible binaries are checked for archive structure only.
#
# Usage: tools/verify-cli-install.sh [dist-dir] [version]
set -euo pipefail

DIST_DIR="${1:-dist/v0.26.3-uni}"
VERSION="${2:-v0.26.3-uni}"

fail() {
	echo "FAIL: $*" >&2
	exit 1
}

[ -d "${DIST_DIR}" ] || fail "dist dir not found: ${DIST_DIR}"
DIST_DIR="$(cd "${DIST_DIR}" && pwd)"
[ -f "${DIST_DIR}/checksums.txt" ] || fail "missing ${DIST_DIR}/checksums.txt"

work="$(mktemp -d)"
trap 'rm -rf "${work}"' EXIT

host_os="$(uname -s | tr '[:upper:]' '[:lower:]')"
host_arch="$(uname -m)"
case "${host_arch}" in
	x86_64) host_arch=amd64 ;;
	aarch64|arm64) host_arch=arm64 ;;
esac

check_archive() {
	local os="$1" arch="$2" ext="$3"
	local name="uni_${VERSION}_${os}_${arch}.${ext}"
	local dir="${work}/${os}-${arch}"
	mkdir -p "${dir}/home" "${dir}/bin"
	cd "${dir}"

	# Page step: download (local copy stands in) and verify against checksums.txt.
	cp "${DIST_DIR}/${name}" . || fail "missing archive ${name}"
	cp "${DIST_DIR}/checksums.txt" .
	grep " ${name}\$" checksums.txt | shasum -a 256 -c - > /dev/null \
		|| fail "checksum mismatch for ${name}"

	# Page step: unpack. Archives must contain the bare executable at top level.
	case "${ext}" in
		tar.gz)
			tar -xzf "${name}"
			[ -f ./uni ] || fail "${name} does not contain top-level uni"
			;;
		zip)
			unzip -q "${name}"
			[ -f ./uni.exe ] || fail "${name} does not contain top-level uni.exe"
			;;
	esac

	# Run the page command sequence only for binaries this host can execute.
	if [ "${os}" = "${host_os}" ] && [ "${arch}" = "${host_arch}" ]; then
		# sudo install -m 0755 uni /usr/local/bin/uni (stand-in dir, no sudo here)
		install -m 0755 uni "bin/uni"
		env -i HOME="${dir}/home" PATH="${dir}/bin:/usr/bin:/bin" sh -c \
			'uni --help > /dev/null && uni setup --help > /dev/null' \
			|| fail "bare uni not runnable after install for ${name}"
		# No-sudo alternative: mkdir -p ~/.local/bin && install -m 0755 uni ~/.local/bin/uni
		mkdir -p "${dir}/home/.local/bin"
		install -m 0755 uni "${dir}/home/.local/bin/uni"
		env -i HOME="${dir}/home" PATH="${dir}/home/.local/bin:/usr/bin:/bin" sh -c \
			'uni --help > /dev/null' \
			|| fail "no-sudo install path broken for ${name}"
		echo "ok (run): ${name}"
	else
		echo "ok (structure only): ${name}"
	fi
	cd "${OLDPWD}"
}

check_archive darwin arm64 tar.gz
check_archive darwin amd64 tar.gz
check_archive linux amd64 tar.gz
check_archive linux arm64 tar.gz
check_archive windows amd64 zip
check_archive windows arm64 zip

echo "all install instructions verified against ${DIST_DIR}"
