<!-- BEGIN:pocketbase-agent-rules -->
 
# PocketBase: ALWAYS read docs before coding
 
Before any PocketBase work, find and read the relevant doc in `https://pocketbase.io/docs/go-overview/`. Your training data is outdated — the docs are the source of truth.
 
<!-- END:pocketbase-agent-rules -->

# Cyclomatic complexity: keep every function at 10 or under

`golangci-lint run` enforces this cap on every commit (`.golangci.yml`). If a
function you write or edit exceeds 10, split it into smaller functions. Do
not raise the cap in `.golangci.yml`.

The cap moved from 4 to 10 because a cap of 4 pushed splits that named no
idea: 65% of unexported server functions ended up with exactly one caller.

# Per-file cap: keep every file at 50 mutation sites or under

`lefthook run gauntlet` enforces this cap with `check-mutation-sites.sh`. A
function-per-file cap alone rewards splitting without limit, because
extracting a block always lowers the extracted function's own score. Mutation
sites per file is the one cheap signal that rises instead of falling when a
function splits without naming a new idea, which is why it is the second
jaw of this repository's complexity gate.
