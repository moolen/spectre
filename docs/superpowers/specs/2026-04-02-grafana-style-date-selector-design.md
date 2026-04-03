# Grafana-Style Date Selector Design

## Goal

Simplify Spectre's date-selection UX in the timeline and graph-related UI so the calendar icon opens a day picker directly, selecting a day writes a full timestamp string with midnight seconds, and users can then edit the time manually in the text field.

## Scope

This design covers the shared date input behavior used by the timeline custom range controls and the graph timestamp editor. Because the same shared component is also used in Settings export fields, those fields will inherit the same improved picker behavior automatically.

In scope:
- remove the extra "calendar inside calendar" interaction step
- change calendar selection from date-time selection to day-only selection
- write selected days back into the text input as `YYYY-MM-DD 00:00:00`
- preserve manual typing of both absolute timestamps and relative expressions
- align timeline and graph-related surfaces on one shared interaction model

Out of scope:
- replacing native browser date input styling with a custom calendar grid
- changing quick preset behavior
- changing backend time parsing semantics
- broader filter-bar or observability page redesign

## Current Behavior

The current timeline custom range input uses a shared text field with a calendar icon. Clicking the icon opens a popover that contains a native `datetime-local` input. On many platforms that native control requires another calendar click before the user can actually choose a date, which adds an unnecessary intermediary step.

The graph timestamp flow currently goes through `TimestampPicker`, which itself uses the shared calendar-backed text input for custom timestamps. That means the core UX problem is concentrated in the shared input component, but the graph timestamp surface still needs verification so its labels, validation, and apply behavior remain correct after the picker change.

## User-Approved Interaction

The approved interaction is:

- only the calendar icon opens the picker
- clicking the text input itself does not open the picker
- the picker allows selecting only a day
- selecting a day writes `YYYY-MM-DD 00:00:00` into the text field
- the user can then type the time portion manually
- relative expressions such as `now`, `2h ago`, and `now-30m` remain valid when entered manually

This keeps day selection fast while preserving the flexibility of a plain text timestamp field.

## Recommended Approach

Use the existing shared date-input component and replace its inner `datetime-local` control with a native `date` input.

Why this approach:

- it removes the extra click with the smallest behavior change
- it preserves the current text-input workflow and validation paths
- it can be reused in both the timeline and graph timestamp surfaces
- it avoids the implementation and maintenance cost of a fully custom calendar widget

Rejected alternatives:

- swapping the main text field itself to `type="date"` when the icon is clicked would make manual editing and relative-expression preservation more brittle
- building a custom calendar would provide more styling control but is unnecessary for the requested behavior

## Component Design

### Shared input component

`ui/src/components/TimeInputWithCalendar.tsx` remains the shared entrypoint for date selection.

Changes:

- keep the main field as `type="text"`
- keep the existing calendar icon button
- keep icon-only opening behavior
- replace the popover content from `input[type="datetime-local"]` to `input[type="date"]`
- when a day is selected, format the resulting value as `YYYY-MM-DD 00:00:00`
- close the popover immediately after successful selection

The component continues to accept manual edits as plain text so relative expressions remain possible.

### Timeline integration

`ui/src/components/TimeRangeDropdown.tsx` continues using the shared component for the custom start and end fields.

No new UI structure is required there. The main behavioral change is that calendar selection writes a second-precision absolute timestamp string instead of a minute-precision value derived from `datetime-local`.

### Graph integration

`ui/src/components/TimestampPicker.tsx` is the graph-facing timestamp editor used from the namespace graph controls.

No structural rewrite is required there. Instead, the plan should verify that `TimestampPicker` continues to work correctly once the shared input switches from `datetime-local` to date-only selection.

That keeps the graph timestamp editor aligned with the timeline:

- same icon-only open affordance
- same date-only selection behavior
- same resulting timestamp string format
- same ability to type relative or absolute values by hand

## Parsing And Formatting

`ui/src/utils/timeParsing.ts` should become the single place that formats absolute input values with second precision.

Behavior requirements:

- absolute values selected from the date picker must render as `YYYY-MM-DD HH:mm:ss`
- existing absolute formats without seconds should continue parsing
- absolute formats with seconds should parse successfully
- relative expressions should continue parsing exactly as they do today

Formatting consistency matters because the user explicitly wants the field to show values such as `2026-04-04 00:00:00` after selecting a day.

## Validation And Refresh Behavior

Timeline behavior:

- existing custom-range validation remains in place
- invalid manual input continues surfacing a validation error
- valid relative or absolute expressions continue applying normally

Graph behavior:

- the text field should tolerate partially typed input locally
- refreshes should only trigger when the current text parses into a valid `Date`
- selecting a day from the picker should immediately produce a valid timestamp string and may trigger refresh using that parsed value

This avoids breaking the editing experience while still preserving the current "refresh on valid change" behavior.

## Testing Strategy

Update unit tests for the shared input component in `ui/src/components/TimeInputWithCalendar.test.tsx` to cover:

- icon click opens the picker
- the picker uses a date-only input rather than `datetime-local`
- selecting a day writes `YYYY-MM-DD 00:00:00` into the text field
- Escape closes the picker
- clicking outside closes the picker

Update parsing and formatting tests in `ui/src/utils/timeParsing.test.ts` to cover:

- formatting helper returns second-precision values
- parsing accepts `YYYY-MM-DD HH:mm:ss`
- existing supported absolute and relative formats still work

If graph timestamp tests already exist near the timestamp editor, add focused coverage for:

- shared component rendering in the editor
- no refresh on invalid partial input
- refresh on valid selected or typed timestamp

If such tests do not exist in the current branch, the minimum automated coverage should remain centered on the shared component and parsing helpers, followed by a manual smoke test of the namespace graph timestamp editor.

## Risks And Mitigations

Risk:
- native `input[type="date"]` varies slightly across browsers and operating systems

Mitigation:
- rely only on the common behavior of day selection, not on browser-specific presentation details

Risk:
- graph timestamp apply logic may currently assume every custom-input change yields a valid parsed value

Mitigation:
- isolate parsing before refresh and only call refresh when parsing succeeds

Risk:
- changing display formatting to include seconds could affect assertions in existing tests

Mitigation:
- update formatting tests deliberately and keep all absolute-value expectations consistent at second precision

## Validation

Implementation should be considered complete only after:

- shared input tests pass
- parsing utility tests pass
- timeline custom range can select a day in one icon click path and shows `YYYY-MM-DD 00:00:00`
- manual typing still accepts relative expressions
- namespace graph timestamp editing follows the same interaction model without validation regressions
