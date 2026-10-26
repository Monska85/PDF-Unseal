#!/bin/sh
# Writes the license texts of everything compiled into the Windows binary to the given file.
set -eu

out=${1:-cmd/pdf-unseal/notices/THIRD_PARTY_LICENSES.txt}
pkg=./cmd/pdf-unseal
line='================================================================================'

# Some distributions move the Go license out of GOROOT.
go_license=$(go env GOROOT)/LICENSE
[ -f "$go_license" ] || go_license=/usr/share/licenses/go/LICENSE

{
	echo "PDF Unseal includes the following third-party software."
	echo
	echo "$line"
	echo "Go standard library"
	echo "$line"
	cat "$go_license"
	echo
	GOOS=windows GOARCH=amd64 go list -deps -f '{{with .Module}}{{if not .Main}}{{.Path}} {{.Version}} {{.Dir}}{{end}}{{end}}' "$pkg" |
		sort -u |
		while read -r path version dir; do
			found=
			for f in "$dir"/*; do
				case $(basename "$f") in
				LICENSE | LICENSE.* | LICENCE | LICENCE.* | COPYING | COPYING.* | NOTICE | NOTICE.*)
					found=1
					echo "$line"
					echo "$path $version ($(basename "$f"))"
					echo "$line"
					cat "$f"
					echo
					;;
				esac
			done
			if [ -z "$found" ]; then
				echo "no license file found for $path" >&2
				exit 1
			fi
		done
} >"$out"
