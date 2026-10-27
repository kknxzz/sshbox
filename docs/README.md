# Documentation

The Markdown files are the source for the website. The matching reStructuredText files are generated from them.

After editing the Markdown, run `node website/scripts/build-docs.mjs` from the repository root and commit the generated files. The site also checks GitHub for updated Markdown and the document index when a visitor opens it. If GitHub is unavailable, it shows the last published copy.
