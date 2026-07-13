# Design QA

## Source and implementation

- Direction: a restrained operations console with a fixed navigation rail, compact
  controls, clear data hierarchy, and consistent light and dark themes.
- Viewports: 1487 x 1058, 1024 x 768, and 390 x 844 CSS pixels at DPR 1
- State: authenticated administrator, light theme, dashboard, six pending review rows, command search empty

## Evidence

- Layout: fixed sidebar, four-metric row, review table, health panel, and quick-create surface preserve the selected direction's hierarchy and spacing.
- Typography: system UI stack with Chinese fallbacks is loaded; headings, metric values, labels, body copy, and table text retain a clear scale without clipping.
- Color and surfaces: indigo primary actions, low-contrast gray page background, white panels, semantic green/amber/red states, restrained borders, and soft shadows match the selected direction.
- Icons: self-hosted Phosphor regular and duotone fonts render consistently across navigation, metrics, buttons, states, forms, and menus.
- Behavior: dashboard navigation, modal open/cancel, dark/light theme switching, global search, quick-create menu, short-link filtering, and user management were exercised successfully.
- Responsiveness: verified at 1487 x 1058, 1024 x 768, and 390 x 844. The page has no document-level horizontal overflow; compact tables scroll inside their own surface.
- Accessibility: semantic labels and visible focus styles remain present, reduced-motion overrides are included, and icon-only mobile navigation exposes translated accessible names with practical tap targets.
- Runtime: browser console contained no errors and the Phosphor fonts reported `loaded`.

## QA history

### Pass 1

- P1 · Layout/behavior: at the default 1280 × 720 browser size, the two-column dashboard left only 631px for a 760px review table, clipping the approve/reject controls. Fixed by stacking the health panel below 1380px and retaining the two-column composition at the reference width.
- P2 · Spacing/content: the quick-create summary wrapped vertically in the crowded header. Fixed with a stable minimum width and no-wrap label, plus tighter content/sidebar tokens.
- P2 · Responsiveness/accessibility: tablet review content had slight avoidable overflow and mobile navigation controls were undersized. Fixed with a 740px tablet table floor and 44px mobile tap targets.

### Pass 2

- Re-captured at the exact 1487 x 1058 source dimensions.
- All review actions remained within their panel, the quick-create surface remained visible within the viewport, document width matched viewport width, dark mode preserved contrast, and key flows remained functional.
- No unresolved P0, P1, or P2 findings.

### Pass 3

- Removed the duplicate dashboard creation action and kept a single context-aware entry point.
- Preserved button icons across loading states and synchronized the localized profile role and document title without a reload.
- Kept the language and theme capsule fixed at the mobile top-right without obscuring the primary content.
- Raised mobile dialogs above the sticky navigation and verified short-link and live-QR creation flows at 390 x 844.
- Verified system-setting field alignment at 1024 x 768 and confirmed that the browser console remained error-free.

Final result: passed
