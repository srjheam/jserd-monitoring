# AGENTS.md

## 1. Think Before Coding

**Don't assume. Don't hide confusion. Surface tradeoffs.**

Before implementing:
- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them - don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

## 2. Simplicity First

**Minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.

Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

## 3. Surgical Changes

**Touch only what you must. Clean up only your own mess.**

When editing existing code:
- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it - don't delete it.

When your changes create orphans:
- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.

The test: Every changed line should trace directly to the user's request.

## 4. Goal-Driven Execution

**Define success criteria. Loop until verified.**

Transform tasks into verifiable goals:
- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan:
```
1. [Step] → verify: [check]
2. [Step] → verify: [check]
3. [Step] → verify: [check]
```

Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.

## 5. Code Quality Gates

A pre-commit hook (`lefthook.yml`) runs on every commit that touches server or client files:

- Server: cyclomatic complexity must not exceed 10 per function, and no file may exceed 50 mutation sites. `golangci-lint` and `check-mutation-sites.sh` enforce these; see `src/server/AGENTS.md`.
- Client: cognitive complexity must not exceed 15 per function. `biome lint` enforces this; config in `src/client/biome.json`.
- Both: the test suite must pass. Coverage is printed on every commit but does not block it.

If a function you write or edit trips a complexity check, split it into smaller functions. Do not raise the cap to make the check pass.

## 6. Structure

Source code located under `src/`.

### PocketBase Server

Located under `src/server/`.

`cd src/server/`

Start with `go run . serve`.

Build with `CGO_ENABLED=0 go build`; then, serve with `./server serve`

### Next.js Client

Located under `src/client/`.

- Use pnpm.
- Use SPA Next.js.

## 7. Commit Is the Verification Step

**This refines §4. Do not run the quality gates before you commit.**

The pre-commit hook already runs the full fast tier: lint, tests, and
coverage. Running that tier by hand first duplicates the work.

- Commit as soon as you believe the change is correct.
- Treat the hook's result as your verification. A passing hook confirms the
  change. A failing hook reports what to fix.
- If the hook fails, fix the cause and commit again. Report the failure if
  you cannot make the hook pass.

**Never bypass the hook.** Do not use `git commit --no-verify`. Do not set
`LEFTHOOK=0` or any other variable that skips a hook. This repository has no
CI, so the pre-commit hook is the only gate. A bypass is the only way that
gate fails open.

## 8. Quality-Gate Vocabulary

**Finding** names one domain thing: a computed statement that a submission
needs a Chief editor (see `CONTEXT.md`). A linter, a formatter, a test
runner, or a coverage tool never produces a finding.

Call the output of a quality-gate tool a **violation**, always. This keeps
"finding" free of a second meaning.

## 9. Pulverisation

**Pulverisation** is the condition of code split into functions to satisfy a
per-function complexity cap, rather than to name a step of reasoning. Its
signature: a private function with exactly one caller, that carries no idea
of its own.

A per-function cap cannot see this condition forming, because extracting a
block out of a function always lowers that function's own score. Watch for
the signature above while you write or split a function.

## 10. Rules No Tool Checks

Linters and formatters catch about half of this repository's craft
heuristics. Apply the rest yourself:

- **Comments stay accurate and non-redundant.** Delete a comment that only
  restates the code below it. Update or delete a comment when the code it
  describes changes.
- **One function, one level of abstraction.** Read a function top to bottom.
  Every line must sit at the same level of detail as its neighbors. If one
  line calls a named domain operation and the next line manipulates a loop
  index, extract the loop into its own function, and only keep the
  extraction if the new function names an idea a caller can reason about
  without reading its body. If it does not, inline it back instead of
  leaving it split. This is the step-down rule, and it is the check that
  would have caught pulverisation before a cap did.
- **High cohesion, low coupling.** A function or a file must gather the
  data and logic that change together, and must not reach into another
  module's internals to do its job.
- **Single Responsibility Principle.** A function or a type has one reason
  to change. If you can describe what it does only with "and," split it by
  responsibility, not by line count.
