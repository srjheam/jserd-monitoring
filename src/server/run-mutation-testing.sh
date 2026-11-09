#!/usr/bin/env bash
set -euo pipefail

# Mutation testing multiplies suite time by the mutant count, so this scopes
# a normal run to the packages a branch actually changed. Pass --full to run
# every package by hand instead (never on a schedule; there is no CI to host
# it).
#
# --min-msi=70 gates the mutation score (not the covered-code score) and is
# only measured against internal/diff, at 76.70%. It is a provisional floor
# below that one measurement, not a per-package figure. Recalibrate per
# package as more of the server gets measured.

cd "$(dirname "$0")"

if [[ "${1:-}" == "--full" ]]; then
  targets=("./...")
else
  base="${MUTATION_DIFF_BASE:-master}"
  mapfile -t changed_files < <(git diff --name-only --relative "$base" -- '*.go' ':!*_test.go')
  if [[ ${#changed_files[@]} -eq 0 ]]; then
    echo "No changed server packages against $base. Nothing to mutate."
    exit 0
  fi
  mapfile -t targets < <(printf '%s\n' "${changed_files[@]}" | xargs -n1 dirname | sort -u | sed 's#^#./#')
fi

go tool go-mutesting --coverage --git-diff-lines --min-msi=70 --ignore-msi-with-no-mutations "${targets[@]}"
