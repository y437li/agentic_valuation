# Display Mode Styles

This document defines the two display modes for the TIED platform UI.

## Mode Toggle
Users can switch between modes via a toggle in the Global Header.
Mode preference is persisted to `localStorage`.

---

## Standard Mode (Default)

**Target Users**: General users, students, newcomers to financial modeling

### Design Philosophy
Consumer-friendly, approachable, modern SaaS aesthetic (Notion/Airbnb style)

### Specifications

| Property | Value |
|----------|-------|
| **Row Height** | 48px |
| **Font Family** | `Inter`, sans-serif |
| **Font Size** | 14px |
| **Border Radius** | 8px |
| **Background** | `#ffffff` (Light) |
| **Text Color** | `#1f2937` |
| **Card Style** | Soft shadows, rounded corners |
| **Negative Numbers** | `-1,234` with minus sign |
| **Grid Lines** | Horizontal only |
| **Icons** | Lucide icons with color accents |

### Color Palette
```
Primary:     #8b5cf6 (Purple)
Success:     #10b981 (Green)
Warning:     #f59e0b (Amber)
Error:       #ef4444 (Red)
Background:  #ffffff
Surface:     #f9fafb
Border:      #e5e7eb
Text:        #1f2937
Muted:       #6b7280
```

---

## Professional Mode

**Target Users**: Financial analysts, portfolio managers, investment professionals

### Design Philosophy
High-density, no-nonsense, Bloomberg Terminal / Excel inspired

### Specifications

| Property | Value |
|----------|-------|
| **Row Height** | 28px (40% reduction) |
| **Font Family** | `JetBrains Mono`, monospace |
| **Font Size** | 12px |
| **Border Radius** | 2px (sharp) |
| **Background** | `#0d1117` (Dark) |
| **Text Color** | `#e6edf3` |
| **Card Style** | Flat, minimal borders |
| **Negative Numbers** | `(1,234)` with parentheses |
| **Grid Lines** | Horizontal + Vertical |
| **Icons** | Minimal, functional |

### Color Palette (Dark Theme)
```
Background:  #0d1117 (GitHub Dark)
Surface:     #161b22
Border:      #30363d
Text:        #e6edf3
Muted:       #8b949e
```

### Financial Color Coding (Blue/Black Rule)
```
Input (Hard-coded):     #58a6ff (Blue)
Calculated (Formula):   #e6edf3 (White/Default)
Link (Source):          #3fb950 (Green)
Negative:               #f85149 (Red, optional)
```

### Number Formatting Rules
1. **Negative numbers**: Use accounting brackets `(1,234)` not minus `-1,234`
2. **Decimal alignment**: All numbers right-aligned, decimals vertically aligned
3. **Thousands separator**: Always use commas `1,234,567`
4. **Percentage**: Show as `12.5%` not `0.125`

---

## CSS Variable Reference

```css
:root[data-theme="standard"] {
  --display-row-height: 48px;
  --display-font-family: 'Inter', sans-serif;
  --display-font-size: 14px;
  --display-border-radius: 8px;
  --display-bg-primary: #ffffff;
  --display-bg-surface: #f9fafb;
  --display-text-primary: #1f2937;
  --display-text-muted: #6b7280;
  --display-border-color: #e5e7eb;
}

:root[data-theme="professional"] {
  --display-row-height: 28px;
  --display-font-family: 'JetBrains Mono', monospace;
  --display-font-size: 12px;
  --display-border-radius: 2px;
  --display-bg-primary: #0d1117;
  --display-bg-surface: #161b22;
  --display-text-primary: #e6edf3;
  --display-text-muted: #8b949e;
  --display-border-color: #30363d;
  --display-input-color: #58a6ff;
  --display-calc-color: #e6edf3;
  --display-link-color: #3fb950;
}
```

---

## Implementation Notes

### ThemeContext
- Store `displayMode: 'standard' | 'professional'` in React Context
- Persist to `localStorage` key: `tied-display-mode`
- Apply `data-theme` attribute to `<html>` element

### Component Adaptation
Components should use CSS variables for theme-aware styling:
```tsx
// Example: Row styling
<tr style={{ height: 'var(--display-row-height)' }}>
```

### Font Loading
JetBrains Mono is loaded from Google Fonts only when Professional mode is active.
