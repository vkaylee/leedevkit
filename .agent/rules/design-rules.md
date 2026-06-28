# 🎨 UI/UX Design Rules & WCAG 2.1 AA Compliance

## 1. Design System
- **Components:** Use predefined components in `webdashboard/src/components/ui`. Don't reinvent the wheel.
- **Premium Patterns:** Use premium design patterns (e.g., Combobox/Radix instead of native selects).

## 2. Accessibility & WCAG 2.1 AA Standards (MANDATORY)
To guarantee the application is fully accessible, all code and UI changes MUST strictly follow these WCAG 2.1 AA guidelines:

### A. Color Contrast Ratios
- **Regular Text:** Text smaller than 18pt (24px) or 14pt bold (18.67px) MUST have a contrast ratio of at least **4.5:1** against the background.
- **Large Text:** Text 18pt (24px) or larger, or 14pt bold (18.67px) or larger, MUST have a contrast ratio of at least **3.0:1**.
- **Interactive Components:** Graphical objects and user interface components (borders, state icons, input borders) MUST have a contrast ratio of at least **3.0:1** against adjacent colors.

### B. Keyboard Accessibility & Focus
- **Full Keyboard Access:** Every interactive element (buttons, dropdowns, inputs, modal triggers) MUST be fully focusable and operable via keyboard alone (`Tab`, `Shift+Tab`, `Space`, `Enter`).
- **Visible Focus States:** Focus rings (`focus-visible:ring-2`) must ALWAYS be visible when navigating via keyboard. NEVER suppress browser-default outlines without providing a custom visible focus style.
- **Focus Trapping:** Dialogs, drawers, and modal windows MUST trap focus within their container when open, and restore focus to the trigger upon closing. Pressing `Escape` must close active overlays.

### C. Screen Reader Support (Semantic HTML & ARIA)
- **Descriptive Labels:** Icon-only buttons (e.g., trash bin, pen, chevron) MUST include a descriptive `aria-label` or a visually hidden `<span class="sr-only">Label</span>` element.
- **Form Controls:** Every input form element MUST have an explicitly associated `<label>` using `htmlFor` or be labeled via `aria-labelledby`/`aria-label`.
- **Semantic Tags:** Use HTML5 semantic tags (`<main>`, `<header>`, `<nav>`, `<aside>`, `<section>`) instead of nested generic `<div>` wrappers.

### D. Information & Sensory Characteristics
- **Multi-channel Cues:** Never convey states, errors, or alerts solely through color (e.g. turning a border red on validation error). Always supply text warnings, semantic aria-invalid attributes, or iconography alongside color changes.

## 3. Responsive & Motion
- **Layout:** Mobile-first approach. Ensure layouts scale cleanly from mobile screens up to desktop monitors.
- **Animations:** Use subtle transitions (`duration-200`); avoid jarring or over-the-top animations. Implement smooth micro-interactions on hover and focus states.

## 4. Edge Cases & States
- **Data Loading:** Always implement Skeleton loaders while fetching data to prevent layout shift.
- **No Data:** Always implement clear, beautiful Empty States when there is no data.

