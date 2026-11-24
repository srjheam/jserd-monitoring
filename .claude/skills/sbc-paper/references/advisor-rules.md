# Advisor and course rules

Sources:

- Advisor (Prof. Monalessa Perini Barcellos): https://inf.ufes.br/~monalessa/orientacao/informacoes-gerais-para-orientandos/
- Course monograph template, "Dicas para escrita em LaTeX": http://www.informatica.ufes.br/sites/informatica.ufes.br/files/field/anexo/modelo-monografia-ccomp-2019.pdf

## Writing

- **A1 Write in English.** Compose the text in English. Do not write a Portuguese draft to translate.
- **A2 No sloppy text.** The text has no typos, no grammar errors, no `??` in the PDF, and no unfinished sentences.
- **A3 Good models.** When you need a model for a section, use published papers of good quality. A model is a model for structure and narrative only. A phrase in a model that breaks a `J` rule is a violation in this paper too.
- **A4 Impersonal voice.** The subject of a sentence is the system, a component, a role, or the work. The paper does not use "we", "our", or "us".

## Structure and narrative

- **A5 Outline.** A tool paper has this order of sections: Introduction; context and the illustrative scenario; requirements and architecture; features; evaluation, if the work has one; related work; discussion and limitations; conclusion. Each section has one job. Text that does the job of another section moves to that section.
- **A6 Introduction.** The Introduction goes from the context to the problem, then to the gap in existing tools, then to the tool. It lists the contributions as (i), (ii), (iii). Its last paragraph gives the organization of the paper, one sentence for each section.
- **A7 Section goals.** Each section and subsection opens with a statement of its goal and of what it presents. The reader knows why the section exists before the details start.
- **A8 Running example.** One illustrative scenario from the real context appears early. The features section uses the same scenario as its running example.
- **A9 Requirements.** The requirements defined at the start of the work (the first submission) are in a table. Each row has an ID and a sentence of the form "The system must...". A functional requirement has its dependencies. A non-functional requirement has its category. A paragraph groups the requirements into categories by their IDs. A set of rules (for example, the notification rules) comes after a statement of why the rules were defined and what they are for.
- **A10 Architecture.** The architecture opens with the main design decisions and their reasons, with links to the requirement IDs. The text then explains the architecture figure, one component and one data flow at a time. Each element in the figure has a name in the text, and each component in the text is in the figure. Technical detail about the code belongs here.
- **A11 Features.** The features section tells what the user does in the system, step by step, from the user's point of view. Each feature has a screenshot, and the text refers to each screenshot. How the code does it belongs in the architecture (`A10`).
- **A12 Figures and tables.** The text introduces each figure and table before it appears, and tells what the reader sees in it.
- **A13 Flow.** Each paragraph has one topic and links to the paragraph before it through content: the next step, the next component, or a consequence. Fragmented text is a violation: short paragraphs with no link between them, or facts listed without a thread that guides the reader.
- **A14 Related work.** Related work compares the tool with each existing tool or approach, feature by feature. When three or more tools compare on the same features, a table summarizes the comparison.
- **A15 Evaluation.** An evaluation states its goal, its research questions, its procedure, its participants, its results, the interpretation of the results, and the threats to validity. Each claim about a result has the strength that its evidence supports.
- **A16 Artifact availability.** A section before the references lists each public artifact (source code, documentation, online instance) with its URL or DOI.

## LaTeX

- **A17 Label prefixes.** Labels use the prefixes `sec-`, `fig-`, `tbl-`, and `lst-`, with English names. Example: `\label{sec-introduction}`.
- **A18 Tie before references.** Put `~` before `\ref`: `Figure~\ref{fig-architecture}`.
- **A19 Capitals.** Capitalize a reference to one item ("Section 3", "Figure 2", "Table 1"). Use lowercase for a plural or a generic mention ("the next sections").
- **A20 Shallow nesting.** Avoid deep section nesting. Use `\section` and `\subsection` only.

## Conclusion

- **A21 Results and objectives.** The Conclusion shows the relation between the results and the objectives stated in the Introduction. It covers the contributions and the limitations.
- **A22 Future work.** Explain each future work item, how and why.

## Correctness

- **A23 Responsibility.** The authors are responsible for every claim: "Se o conteúdo estiver errado ou for fake, a responsabilidade é SUA." Every claim traces to a source of truth (see `SKILL.md`).
- **A24 Real references.** Every reference exists and its metadata is correct: "garantir que o conteúdo e todas as referências citadas em seu trabalho sejam corretos e verdadeiros." For each `.bib` entry, open its DOI or URL, and compare the authors, title, year, and venue. Every `.bib` entry is cited, and every citation has a `.bib` entry.
