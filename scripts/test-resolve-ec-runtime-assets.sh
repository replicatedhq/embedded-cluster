#!/bin/bash
set -euo pipefail

repo_root=$(cd "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/ec-runtime-cache-test.XXXXXX")
trap 'rm -rf "$tmp"' EXIT
fakebin="$tmp/bin"
store="$tmp/store"
mkdir -p "$fakebin" "$store"

cat > "$fakebin/aws" <<'EOF'
#!/bin/bash
set -euo pipefail
root=${FAKE_S3_ROOT:?}
if [ "$1" = s3api ] && [ "$2" = head-object ]; then
  while [ $# -gt 0 ]; do case "$1" in --bucket) bucket=$2; shift 2;; --key) key=$2; shift 2;; *) shift;; esac; done
  test -f "$root/$bucket/$key"
elif [ "$1" = s3api ] && [ "$2" = put-object ]; then
  while [ $# -gt 0 ]; do case "$1" in --bucket) bucket=$2; shift 2;; --key) key=$2; shift 2;; --body) body=$2; shift 2;; *) shift;; esac; done
  target="$root/$bucket/$key"; test ! -e "$target"; mkdir -p "$(dirname "$target")"; cp "$body" "$target"
elif [ "$1" = s3 ] && [ "$2" = cp ]; then
  shift 2; [ "${1:-}" = --no-progress ] && shift; source=$1; dest=$2
  case "$source" in s3://*) source="$root/${source#s3://}";; esac
  case "$dest" in s3://*) dest="$root/${dest#s3://}";; esac
  mkdir -p "$(dirname "$dest")"; cp "$source" "$dest"
else exit 2; fi
EOF
chmod +x "$fakebin/aws"

mkdir -p "$tmp/source"
printf images > "$tmp/source/images-amd64.tar"
printf charts > "$tmp/source/charts.tar.gz"
cat > "$tmp/plan.json" <<'EOF'
{"schema":"v1","networkMode":"airgap","platform":"linux/amd64","planDigest":"abc","requestedImages":[],"charts":[]}
EOF

PATH="$fakebin:$PATH" FAKE_S3_ROOT="$store" EC_RUNTIME_CACHE_BUCKET=test RUNTIME_PLAN="$tmp/plan.json" OUTPUT_DIR="$tmp/first" \
  BUILD_COMMAND="cp '$tmp/source/images-amd64.tar' \"\$OUTPUT_DIR/\"; cp '$tmp/source/charts.tar.gz' \"\$OUTPUT_DIR/\"" \
  "$repo_root/scripts/resolve-ec-runtime-assets.sh"

PATH="$fakebin:$PATH" FAKE_S3_ROOT="$store" EC_RUNTIME_CACHE_BUCKET=test RUNTIME_PLAN="$tmp/plan.json" OUTPUT_DIR="$tmp/second" \
  "$repo_root/scripts/resolve-ec-runtime-assets.sh"
cmp "$tmp/first/manifest.json" "$tmp/second/manifest.json"
