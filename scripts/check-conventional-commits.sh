#!/usr/bin/env bash
set -euo pipefail

if [[ "$#" -ne 1 ]]; then
  echo "usage: $0 <git-revision-range>" >&2
  exit 2
fi

range="$1"
allowed_types="build|chore|ci|docs|feat|fix|perf|refactor|revert|style|test"
header_pattern="^(${allowed_types})(\\([[:alnum:]./_-]+\\))?!?: [^[:space:]].*$"
invalid=0

git rev-list "$range" >/dev/null

while IFS= read -r record; do
  commit="${record%% *}"
  subject="${record#* }"
  if [[ ! "$subject" =~ $header_pattern ]]; then
    printf 'non-conventional commit %s: %s\n' "$commit" "$subject" >&2
    invalid=1
  fi
done < <(git log --no-merges --format='%H %s' "$range")

if [[ "$invalid" -ne 0 ]]; then
  exit 1
fi
