#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/.." && pwd)"

VERSION="${1:-}"
NOTES="${2:-}"

if [ -z "${VERSION}" ]; then
  echo "Usage: ./scripts/release.sh <version> [notes]"
  echo "Example: ./scripts/release.sh v0.5.0 \"Release notes\""
  exit 1
fi

if [[ "${VERSION}" != v* ]]; then
  echo "Error: version must include a leading v, for example v0.5.0" >&2
  exit 1
fi

: "${NOTES:=Release ${VERSION}}"

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Error: required command not found: $1" >&2
    exit 1
  fi
}

need_cmd curl
need_cmd git
need_cmd go
need_cmd mktemp
need_cmd perl

GITCODE_OWNER="${GITCODE_OWNER:-mindspore}"
GITCODE_REPO="${GITCODE_REPO:-mscli}"
GITCODE_API_BASE="${GITCODE_API_BASE:-https://api.gitcode.com/api/v5}"
DIST_DIR="${MSCLI_DIST_DIR:-${REPO_ROOT}/dist}"
CONNECT_TIMEOUT="${MSCLI_RELEASE_CONNECT_TIMEOUT:-10}"
GITCODE_REMOTE="${GITCODE_REMOTE:-}"
WORK_DIR="$(mktemp -d)"

cleanup() {
  rm -rf "${WORK_DIR}"
}
trap cleanup EXIT

if [ -z "${GITCODE_TOKEN:-}" ]; then
  echo "Error: GITCODE_TOKEN is required to create and upload GitCode releases" >&2
  exit 1
fi

api="${GITCODE_API_BASE%/}/repos/${GITCODE_OWNER}/${GITCODE_REPO}"
auth_q="access_token=${GITCODE_TOKEN}"

pick_gitcode_remote() {
  local origin_url=""

  if [ -n "${GITCODE_REMOTE}" ]; then
    printf '%s\n' "${GITCODE_REMOTE}"
    return 0
  fi

  origin_url="$(git remote get-url origin 2>/dev/null || true)"
  if printf '%s' "${origin_url}" | grep -q "gitcode.com[:/].*${GITCODE_OWNER}/${GITCODE_REPO}"; then
    printf '%s\n' "origin"
    return 0
  fi

  if git remote get-url mscli >/dev/null 2>&1; then
    printf '%s\n' "mscli"
    return 0
  fi

  printf '%s\n' "origin"
}

ensure_release_tag() {
  local remote="$1"

  if ! git rev-parse --verify --quiet "refs/tags/${VERSION}" >/dev/null; then
    echo "Creating local tag ${VERSION}..."
    git tag -a "${VERSION}" -m "Release ${VERSION}"
  fi

  if git ls-remote --exit-code --tags "${remote}" "refs/tags/${VERSION}" >/dev/null 2>&1; then
    echo "Tag ${VERSION} already exists on ${remote}"
    return 0
  fi

  echo "Pushing tag ${VERSION} to ${remote}..."
  git push "${remote}" "refs/tags/${VERSION}"
}

json_payload() {
  perl -MJSON::PP -e '
    use strict;
    use warnings;
    my ($tag, $name, $body) = @ARGV;
    print encode_json({
      tag_name => $tag,
      name => $name,
      body => $body,
      prerelease => JSON::PP::false,
    });
  ' "${VERSION}" "${VERSION}" "${NOTES}"
}

json_get() {
  local file="$1"
  local field="$2"
  perl -MJSON::PP -e '
    use strict;
    use warnings;
    my ($file, $field) = @ARGV;
    local $/;
    open my $fh, "<", $file or exit 0;
    my $json = eval { decode_json(<$fh>) } or exit 0;
    exit 0 unless ref($json) eq "HASH";
    print($json->{$field} // q());
  ' "${file}" "${field}"
}

release_upload_field() {
  local file="$1"
  local field="$2"
  perl -MJSON::PP -e '
    use strict;
    use warnings;
    my ($file, $field) = @ARGV;
    local $/;
    open my $fh, "<", $file or exit 1;
    my $json = decode_json(<$fh>);
    my @path = split /\./, $field;
    my $value = $json;
    for my $part (@path) {
      exit 1 unless ref($value) eq "HASH";
      $value = $value->{$part};
    }
    exit 1 unless defined $value && !ref($value);
    print $value;
  ' "${file}" "${field}"
}

release_upload_header_args() {
  local file="$1"
  perl -MJSON::PP -e '
    use strict;
    use warnings;
    my ($file) = @ARGV;
    local $/;
    open my $fh, "<", $file or exit 0;
    my $json = eval { decode_json(<$fh>) } or exit 0;
    my $headers = $json->{headers};
    exit 0 unless ref($headers) eq "HASH";
    for my $key (sort keys %$headers) {
      my $value = $headers->{$key};
      next if ref($value);
      print "-H\0${key}: ${value}\0";
    }
  ' "${file}"
}

create_or_update_release() {
  local payload
  local http_code
  local error_code

  payload="$(json_payload)"

  echo "Checking GitCode release ${GITCODE_OWNER}/${GITCODE_REPO}@${VERSION}..." >&2
  http_code="$(curl -sS -o "${WORK_DIR}/release.json" -w "%{http_code}" \
    --connect-timeout "${CONNECT_TIMEOUT}" \
    "${api}/releases/tags/${VERSION}?${auth_q}")"
  error_code="$(json_get "${WORK_DIR}/release.json" error_code)"

  if [ "${http_code}" = "200" ]; then
    echo "GitCode release ${VERSION} already exists." >&2
    return 0
  fi

  if [ "${http_code}" = "400" ] && [ "${error_code}" = "404" ]; then
    http_code="404"
  fi

  if [ "${http_code}" != "404" ]; then
    echo "Error: unexpected GitCode release lookup response: ${http_code}" >&2
    cat "${WORK_DIR}/release.json" >&2 || true
    exit 1
  fi

  echo "Creating GitCode release ${VERSION}..." >&2
  curl -sS --fail \
    -X POST \
    --connect-timeout "${CONNECT_TIMEOUT}" \
    "${api}/releases?${auth_q}" \
    -H "Content-Type: application/json" \
    -d "${payload}" \
    > "${WORK_DIR}/result.json"
}


upload_assets() {
  local file
  local file_name
  local upload_meta
  local upload_url
  local upload_status
  local -a header_args

  if [ ! -d "${DIST_DIR}" ]; then
    echo "Error: asset directory not found: ${DIST_DIR}" >&2
    exit 1
  fi

  shopt -s nullglob
  for file in "${DIST_DIR}"/*; do
    if [ ! -f "${file}" ]; then
      continue
    fi
    file_name="$(basename "${file}")"
    echo "Uploading ${file_name}..."

    upload_meta="${WORK_DIR}/upload-${file_name}.json"
    curl -sS --fail \
      --connect-timeout "${CONNECT_TIMEOUT}" \
      "${api}/releases/${VERSION}/upload_url?${auth_q}&file_name=${file_name}" \
      > "${upload_meta}"
    upload_url="$(release_upload_field "${upload_meta}" upload_url || release_upload_field "${upload_meta}" url || release_upload_field "${upload_meta}" data.upload_url || release_upload_field "${upload_meta}" data.url)"
    if [ -z "${upload_url}" ]; then
      echo "Error: GitCode did not return an upload URL for ${file_name}" >&2
      cat "${upload_meta}" >&2 || true
      exit 1
    fi

    header_args=()
    while IFS= read -r -d '' arg; do
      header_args+=("${arg}")
    done < <(release_upload_header_args "${upload_meta}")

    upload_status="$(curl -sS -o "${WORK_DIR}/upload-${file_name}.out" -w "%{http_code}" \
      -X PUT \
      --connect-timeout "${CONNECT_TIMEOUT}" \
      "${header_args[@]}" \
      --data-binary "@${file}" \
      "${upload_url}")"
    if [ "${upload_status}" != "200" ] && [ "${upload_status}" != "201" ] && [ "${upload_status}" != "204" ]; then
      echo "Error: upload ${file_name} failed with HTTP ${upload_status}" >&2
      cat "${WORK_DIR}/upload-${file_name}.out" >&2 || true
      exit 1
    fi
  done
  shopt -u nullglob
}


cd "${REPO_ROOT}"

gitcode_remote="$(pick_gitcode_remote)"

echo "==> Preparing release tag"
ensure_release_tag "${gitcode_remote}"

echo ""
echo "==> Building release assets"
"${SCRIPT_DIR}/build-release-assets.sh" "${VERSION}"

echo ""
echo "==> Creating or updating GitCode release"
create_or_update_release

echo ""
echo "==> Uploading assets"
upload_assets

echo ""
echo "Done. GitCode release:"
echo "  https://gitcode.com/${GITCODE_OWNER}/${GITCODE_REPO}/releases/tag/${VERSION}"
