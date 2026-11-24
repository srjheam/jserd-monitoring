---
name: sbc-paper
description: Write, revise, review, or build the SBC paper (the TCC) in PGs/2026_2_SBC_Jheam_Ross/. Use for any change to sbc-jheam-ross.tex or sbc-jheam-ross.bib, a review of the paper against its rules, or a PDF build.
---

# SBC paper

The paper is `PGs/2026_2_SBC_Jheam_Ross/sbc-jheam-ross.tex`, with references in `sbc-jheam-ross.bib`. It is an ERI-ES full paper in English, and it is also the author's TCC. Run every command from `PGs/2026_2_SBC_Jheam_Ross/`.

## Rules

Three rule files apply to every task. Read all three before you start:

- [`references/eries-rules.md`](references/eries-rules.md): ERI-ES and SBC template rules, IDs `E1`...
- [`references/advisor-rules.md`](references/advisor-rules.md): the advisor's and the course's rules, IDs `A1`...
- [`references/llm-jargon.md`](references/llm-jargon.md): banned LLM phrasing, IDs `J1`...

When two rules conflict, `E` wins over `A`. `J` applies everywhere in prose. The paper has its own style: these rule files are the whole style guide, so a style from the session (for example, an output style) does not apply to the paper text.

## Sources of truth

Every claim in the paper must trace to a source. Use them in this order:

1. What the system does: the code in `src/` and its tests.
2. Motivation, context, and requirements: the first submission in `PGs/2026_1_PG1_Jheam_Ross/` (in Portuguese).

A claim is **unverified** when no source covers it, or when two sources disagree:

- In a grilling or planning session, ask the user before you write the claim.
- During implementation, write the claim and put `% CHECK: <question for the user>` on the line above it. List every `% CHECK:` you added in your final message.

## Tasks

### Write

Add a new section, paragraph, figure, or table.

1. Find the section of the outline (`A5`) where the text belongs.
2. Collect the facts from the sources of truth. Handle each unverified claim as above.
3. Write in English, at the level of detail of the text around it, with the terms that the paper already uses for the same things.
4. Apply every rule in the three rule files to the new text.
5. Run **Build**.

Done when the new text traces to sources, obeys every rule, and the build passes.

### Revise

Change text that already exists, for a rule violation, a review comment, or a user request.

1. Change the smallest span that fixes the problem. Keep the meaning, unless the meaning is the problem.
2. If the fix changes a claim, trace the new claim to a source.
3. Apply every rule to the changed text.
4. Run **Build**.

Done when the violation is gone, no new violation appears in the changed text, and the build passes.

### Review

Check the whole paper against every rule, and write a report. Do not edit the paper in this task.

1. Read the `.tex` file and the `.bib` file in full, and the PDF from a fresh **Build**.
2. Apply every rule in the three files, one by one, to the whole paper. Check each `.bib` entry for `A24`. Check each claim about the system against the code for `A23`.
3. Write the report to `.reviews/<YYYY-MM-DD-HHMM>.md` in the paper folder. Git ignores this folder.

Done when every rule ID appears in the report, with its violations or with "no violation".

Each violation in the report has:

- the location, as `sbc-jheam-ross.tex:<line>` (or `.bib`, or the PDF page)
- the rule ID
- the quoted text
- why it violates the rule
- a proposed fix

Sort the violations most severe first: `E` violations, then wrong or unverified claims, then the rest.

### Build

```bash
latexmk -pdf sbc-jheam-ross.tex
```

Then check:

- The log has no errors, no undefined references, and no undefined citations: `grep -nE 'Error|undefined' sbc-jheam-ross.log`.
- The page count obeys `E1`: `pdfinfo sbc-jheam-ross.pdf | grep Pages`.
- The abstract obeys `E5`: `pdftotext -l 1 -layout sbc-jheam-ross.pdf -` and count the abstract lines.
- Report each `Overfull \hbox` in the log with its line.

Done when every check passes, or when you reported each failure to the user.
