<!-- AGENT INSTRUCTIONS — these apply before writing any section.
- MUST keep the sections below in this order: Context / Motivation, Diagrams, Interface Changes, Description, Testing.
- Context / Motivation, Description, and Testing are always included. Diagrams and Interface Changes are optional: include each only when its comment says it applies.
- MUST read the existing description before rewriting. When updating an existing PR, MUST fetch its current body from GitHub (e.g. `gh pr view <number> --json body -q .body`) and edit that text, preserving anything the author wrote by hand.
- MUST describe only the end state of the branch. NEVER narrate earlier drafts, commits, or design iterations.
- NEVER reference plans, notes, or other working documents from the agent session (e.g. "per the plan", "Phase 2", "step 3 of the doc"). The reviewer has never seen them. State the relevant context directly, or link a ticket or doc the reviewer can open.
- IF a section does not apply, MUST omit it entirely. NEVER write N/A. -->

# Context / Motivation
<!-- Ideally written by the author. Keep it short: 1-3 sentences on why this change exists, framed in the wider context (the project, issue, or bug it belongs to). Link the issue and any related PRs.
- IF the author explained the motivation in the conversation, MUST use their words.
- MUST keep the "Agent draft" note below unless this section is the author's own text, verbatim. Summarizing or rewording what the author said still counts as an agent draft. NEVER pad it out; a reviewer should finish this section in under a minute.
- NEVER describe implementation here. That belongs in Description. -->

> [!NOTE]
> Agent draft. Author: rewrite this section in your own words, then delete this note.

# Diagrams
<!-- Up front so a reviewer can understand the shape of the change before reading the diff.
- MUST use a Mermaid diagram (GitHub renders ```mermaid blocks).
- Choose the type that fits: sequence diagram for request/workflow flows, flowchart for component boundaries or data flow, before/after pair when the change rewires an existing flow.
- MUST keep it small enough to read without zooming. Highlight what this PR adds or changes (e.g. `classDef changed fill:#f9d77e`).
- IF the change is trivial (a one-line fix, a config bump, a rename), MUST omit this section. -->

# Interface Changes
<!-- Describe each change from the consumer's perspective, grouped as New / Modified / Removed. MUST mark breaking changes with **Breaking**.
- Covers the public Go API (including generated provider code), CLI commands and flags, and the rendered `.tf.json` output.
- IF there are no public interface or API changes, MUST omit this section. -->

# Description
<!-- Okay to be fully agent-generated. Capture all the changes in detail, grouped by component, with code names, flags, and paths in backticks.
- MUST start with a one-line summary outside the toggle, then put the detail inside the <details> block below so the page stays scannable.
- MUST call out anything a reviewer should look at closely (judgement calls, behavior changes, rollout effects). -->

<details>
<summary>Full description</summary>

</details>

# Testing
<!-- End-to-end evidence only: `terraform`/`tofu` plan or validate runs against real providers, snapshot or migration test runs, manual verification, with links to CI jobs where possible.
- NEVER list unit tests. CI already shows them; at most, one line saying which behavior they cover.
- NEVER claim a test that was not run. IF end-to-end validation is still pending, MUST say so and say what will be run (e.g. "Will run `make snapshot-tests` against the updated provider"). -->
