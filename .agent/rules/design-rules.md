# Interface Design and Accessibility

> Scope: Changes to user interfaces in applications that expose an interactive UI. Follow the project’s component system and accessibility targets where established. This guidance supports accessibility; it does not certify WCAG conformance.

## Reuse and Accessibility

- Reuse existing project components and patterns where they meet the requirement. Choose native or custom controls based on usability, accessibility, and platform behavior rather than a universal preference.
- Provide adequate contrast, visible keyboard focus, keyboard operation, and semantic labels for interactive controls. Do not remove a native focus indicator without a visible replacement.
- Associate form controls with accessible names. Use semantic elements and ARIA only where appropriate; custom widgets must provide equivalent keyboard and assistive-technology behavior.
- Do not communicate state using color alone. Ensure errors and status changes are available through text or appropriate semantics.
- Dialogs and other modal interactions must manage focus and provide an understandable way to dismiss them, consistent with platform conventions and user needs.

## Layout and States

- Support the project’s target viewport sizes, input modalities, and user preferences. Avoid motion that creates usability or accessibility problems; respect reduced-motion preferences where relevant.
- Represent loading, empty, error, and success states clearly when applicable. Choose visual treatments that fit product needs and avoid unnecessary layout shifts.
