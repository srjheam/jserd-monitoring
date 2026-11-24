# LLM jargon

Source: https://en.wikipedia.org/wiki/Wikipedia:Signs_of_AI_writing

The target is plain technical prose: each sentence states a fact about the system, the data, or the work, with ordinary verbs ("is", "has", "uses", "reads", "sends"). Each pattern below is a violation in prose. Code, TikZ paths, URLs, and quoted names are not prose.

## Sentence patterns

- **J1 Negative parallelism.** "It is not X, it is Y", "not X, but Y", "not only X but also Y", "not just X", "X rather than Y" when X is a contrast that no one claimed. State Y alone.
- **J2 Rule of three.** A list of three adjectives, nouns, or clauses for rhythm, not because the content has exactly three items. Keep only the items that carry a fact.
- **J3 Trailing analysis.** A clause that ends in an "-ing" comment on the sentence before it: ", highlighting...", ", ensuring...", ", reflecting...", ", contributing to...", ", underscoring...". End the sentence at the fact, and put a real consequence in its own sentence.
- **J4 Copula avoidance.** "serves as", "stands as", "functions as", "acts as", "represents", "boasts", "features" in place of "is" or "has". Use "is" or "has".
- **J5 Corrective framing.** "does not establish", "should be treated as X rather than Y", defensive denials of a claim that no one made. State what the work does.
- **J6 Summary openers.** "In summary", "In conclusion", "Overall", "In short", "To sum up". The Conclusion section heading already does this job.
- **J7 Didactic asides.** "It is important to note", "It is worth noting", "Note that", "Notably", "Importantly". State the fact.
- **J8 Outline-like challenges.** "Despite its X, it faces challenges", "Despite these challenges", "Challenges and future directions" as filler. Name each limitation and its cause.

## Vocabulary

- **J9 Significance inflation.** "pivotal", "crucial", "vital", "key role", "plays a significant role", "is a testament to", "underscores the importance", "marks a turning point", "sets the stage", "evolving landscape", "paves the way", "indelible", "deeply rooted". Say what the thing does, with a number or a source if it matters.
- **J10 AI vocabulary.** "delve", "intricate", "intricacies", "interplay", "tapestry", "landscape" (abstract), "meticulous", "meticulously", "garner", "bolster", "enduring", "vibrant", "showcase", "foster", "cultivate", "enhance", "leverage", "robust", "seamless", "seamlessly", "multifaceted", "holistic", "comprehensive", "streamline", "empower", "unlock", "harness", "elevate", "navigate" (abstract), "realm", "dynamic", "valuable insights", "align with", "resonate with", "deep dive", "quietly", "steady", "dependable", "matters". Use the plain word: "use", "improve", "complete", "detailed", "many".
- **J11 Promotional words.** "groundbreaking", "cutting-edge", "state-of-the-art" (without a comparison), "innovative", "renowned", "rich", "profound", "powerful", "exemplifies", "commitment to". Describe the property and its evidence.
- **J12 Vague attribution.** "Experts argue", "Studies show", "It is widely known", "several sources", "such as" before a list that is complete. Cite the source, or state the claim as the authors' own.
- **J13 Vague connection.** "in connection with", "in association with", "associated with", "related to" where the exact relation is known. Name the relation: "reads", "calls", "stores", "depends on".
- **J14 Filler transitions.** "Moreover", "Furthermore", "Additionally", "In addition", "Consequently" at the start of consecutive sentences. Most sentences need no transition word.

## Style

- **J15 Dashes.** No em dash (`---`, `—`) and no en dash (`--`, `–`) in prose. Use a comma, a colon, parentheses, a new sentence, or "to" for ranges ("RN1 to RN5").
- **J16 Synonym rotation.** One thing has one name in the whole paper. Do not rotate synonyms for the same thing ("tool", "system", "solution", "platform").
- **J17 Bold labels.** No `\textbf{Label:}` at the start of list items or paragraphs in prose. Figure annotations are not prose.
- **J18 Quotes.** Use LaTeX quotes (` ``text'' `), not curly quotes (“ ” ’) in the source.
- **J19 Hedges.** "may potentially", "could possibly", "arguably", "to some extent", "in many ways". State the claim at the strength that its source supports.
