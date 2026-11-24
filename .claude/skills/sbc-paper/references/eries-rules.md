# ERI-ES and SBC template rules

Sources:

- ERI-ES call for papers: https://eries.sbc.org.br/2026/submissao.php
- SBC template: https://www.sbc.org.br/wp-content/uploads/2024/07/modelosparapublicaodeartigos.zip (`Template_SBC/template-latex/sbc-template.tex`)

`sbc-jheam-ross.sty` implements the template. Keep the template layout: do not change margins, fonts, spacing, or page numbering in the `.tex` file.

## Paper

- **E1 Length.** A full paper has 6 to 10 pages in SBC format, including references, acknowledgements, and appendixes.
- **E2 Language.** English or Portuguese. This paper is in English.
- **E3 Template.** "devem seguir o modelo de formatação estabelecido pela SBC."
- **E4 Authors.** "no máximo, 9 (nove) autores."
- **E5 Abstract.** "For papers in English, you should add just an abstract [...] abstracts should not have more than 10 lines and must be in the first page of the paper."
- **E6 First page.** "The first page must display the paper title, the name and address of the authors, the abstract." The e-mail addresses go below the address.
- **E7 Final version.** After review, "os(as) autores(as) devem levar em consideração os comentários e sugestões fornecidos pelas revisões para aprimorar e enviar a versão final." Every reviewer comment gets a change, or a reason for no change.

## Layout

- **E8 Sections.** "Section titles must be in boldface, 13pt, flush left. [...] Section numbering is optional. The first paragraph of each section should not be indented."
- **E9 Captions.** "centered if less than one line [...] otherwise justified and indented by 0.8cm on both margins." The template sets the font. "Table caption must be placed before the table." A figure caption goes below the figure.
- **E10 Tables.** "avoid the use of colored or shaded backgrounds, and avoid thick, doubled, or unnecessary framing lines. When reporting empirical data, do not use more decimal digits than warranted by their precision and reproducibility."
- **E11 Images.** "Do not include images with excessive resolution." Use about 600 dpi for black-and-white images and 150 to 300 dpi for grayscale images. Color is allowed because the proceedings are electronic.

## References

- **E12 References.** "Bibliographic references must be unambiguous and uniform. We recommend giving the author names references in brackets." Use `\bibliographystyle{sbc}` with `sbc.bst`.
