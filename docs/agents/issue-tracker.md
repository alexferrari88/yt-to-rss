# Issue tracker: Local Markdown

Issues and specs live as Markdown files in `.scratch/`.

## Conventions

- One feature per directory: `.scratch/<feature-slug>/`.
- Spec: `.scratch/<feature-slug>/spec.md`.
- Tickets: `.scratch/<feature-slug>/issues/<NN>-<slug>.md`,
  numbered from `01`, with one file per ticket.
- Triage state: a `Status:` line near the top of each ticket;
  use the role strings from `triage-labels.md`.
- Conversation history: append under `## Comments`.

## Publishing and fetching

To publish, create the appropriate file and its parent directories.
To fetch a ticket, read its referenced path.

## Wayfinding operations

- Map: `.scratch/<effort>/map.md`, with Notes,
  Decisions-so-far, and Fog sections.
- Child tickets: `.scratch/<effort>/issues/NN-<slug>.md`.
  Record `Type: research`, `prototype`, `grilling`, or `task`.
- Dependencies: `Blocked by: NN, NN`.
- Frontier: open, unblocked, unclaimed tickets; lowest number first.
  A dependency is satisfied when its ticket is resolved.
- Claim: save `Status: claimed` before working.
- Resolve: append the answer under `## Answer`, save
  `Status: resolved`, and add a summary and ticket link to
  the map's Decisions-so-far section.
