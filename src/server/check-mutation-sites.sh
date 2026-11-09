#!/usr/bin/env bash
set -euo pipefail

# Enforces the per-file cap from
# .scratch/quality-gates/issues/04-settle-the-deferred-decisions.md,
# Decision B: mutation sites per file, capped at 50, no cheaper proxy. A
# rising site count is the one per-file signal that cannot be gamed by
# splitting a function, which is why it is the gate instead of function
# count or line count.
#
# Scoping mirrors run-mutation-testing.sh: only the packages a branch
# actually changed, unless --full is passed. --no-exec skips running the
# test suite per mutant, so this only generates and type-checks candidate
# mutants, which is far cheaper than a scored run but still not free
# (dozens of seconds for the largest packages), so --full stays a by-hand
# check, never the default.

CAP=50

cd "$(dirname "$0")"

if [[ "${1:-}" == "--full" ]]; then
  mapfile -t targets < <(go list -f '{{.Dir}}' ./... | sed "s#^$PWD#.#")
else
  base="${MUTATION_DIFF_BASE:-master}"
  mapfile -t changed_files < <(git diff --name-only --relative "$base" -- '*.go' ':!*_test.go')
  if [[ ${#changed_files[@]} -eq 0 ]]; then
    echo "No changed server packages against $base. Nothing to check."
    exit 0
  fi
  mapfile -t targets < <(printf '%s\n' "${changed_files[@]}" | xargs -n1 dirname | sort -u | sed 's#^#./#')
fi

offenders=()

for target in "${targets[@]}"; do
  dir="${target#./}"

  scratch="$(mktemp -d)"
  trap 'rm -rf "$scratch"' EXIT

  TMPDIR="$scratch" go tool go-mutesting --no-exec --do-not-remove-tmp-folder --quiet "./$dir/..." >/dev/null 2>&1 || true

  mutant_dir="$(find "$scratch" -maxdepth 1 -type d -name 'go-mutesting-*')/$dir"
  if [[ ! -d "$mutant_dir" ]]; then
    rm -rf "$scratch"
    trap - EXIT
    continue
  fi

  while IFS= read -r -d '' source_file; do
    base_name="$(basename "$source_file")"
    count=$(find "$mutant_dir" -maxdepth 1 -name "${base_name}.*" ! -name '*.original' | wc -l)
    if (( count > CAP )); then
      offenders+=("$dir/$base_name: $count sites (cap $CAP)")
    fi
  done < <(find "$dir" -maxdepth 1 -name '*.go' ! -name '*_test.go' -print0)

  rm -rf "$scratch"
  trap - EXIT
done

if (( ${#offenders[@]} > 0 )); then
  echo "Files over the mutation-sites-per-file cap:"
  printf '  %s\n' "${offenders[@]}"
  exit 1
fi

echo "No file exceeds $CAP mutation sites."
