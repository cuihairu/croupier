---
title: Editor reference analysis
---

# Editor reference analysis

This analysis compares the composite editor with amis-editor and Appsmith by interaction dimension. It records observed behavior and implementation status; it is not a product argument.

## Overall assessment

The earlier implementation had individual features but lacked a coherent operation flow. The largest gaps were:

1. A forced empty-canvas wizard instead of an immediately usable canvas.
2. No way to enter a modal and edit its children in context.
3. Weak separation between reusable data sources and visual components.

The August 2026 comparison round closed those workflow gaps. The current editor keeps Croupier's proposal-and-publication governance instead of adopting immediate deployment.

## Dimension comparison

| Dimension      | Appsmith                                             | amis-editor                             | Croupier implementation                                                                 |
| -------------- | ---------------------------------------------------- | --------------------------------------- | --------------------------------------------------------------------------------------- |
| New page       | Template or blank application choice.                | Directly blank canvas.                  | Blank draggable canvas; no forced wizard.                                               |
| Add component  | Drag to a drop target or click to place.             | Drag with insertion indicator or click. | Drag with insertion indicator or click.                                                 |
| Data component | Widget and query are loosely coupled and rebindable. | CRUD component selects an API.          | Function becomes the initial component; rebinding and multiple instances are supported. |
| Selection      | Resize handles, floating toolbar, and context menu.  | Selection and context actions.          | Selection frame and context menu.                                                       |
| Multi-select   | Marquee or modifier selection.                       | Shift accumulation.                     | Shift-click accumulation and bulk delete.                                               |
| Properties     | Property, style, and event tabs.                     | Config panels by component.             | Configuration and action tabs.                                                          |
| Events         | Dropdown or JavaScript.                              | Configurable actions.                   | Declarative ordered action chains.                                                      |
| Modal editing  | Enter modal content in place.                        | Modal canvas editing.                   | Placeholder card with inner canvas and breadcrumb return.                               |
| Preview        | Immediate top-bar mode switch.                       | Preview mode.                           | Uses the published renderer.                                                            |
| Data debugging | Persistent query panel.                              | Request debugging.                      | Bottom panel with one-click function execution.                                         |
| Outline        | Component tree.                                      | Tree or layer navigation.               | Outline tab.                                                                            |
| Save           | Immediate save or deploy.                            | Save configuration.                     | Proposal, validation, acceptance, publication, then menu mount.                         |

## Implemented improvement list

- In-modal nested editing.
- Drag insertion indicators.
- Property/action panel separation.
- Bottom data trial panel.
- Parent selection and context menu.
- Undo and redo.
- Backend action-chain model and visual editing.
- Multi-component modal grouping.
- Complete container-child interaction.
- Top-button round-trip restoration.
- Proposal and draft multi-source restoration.
- Shift multi-select and deletion.
- Same-function multiple instances and rebinding.

## Remaining boundaries

- Very deep container descendants use simplified rendering.
- Text nodes remain editor-only and do not publish.

These are deliberate model and maintenance trade-offs rather than undocumented defects.
