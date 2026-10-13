import type { GlobalThemeOverrides } from 'naive-ui'

/**
 * Bridges the CSS design tokens (src/styles/tokens.css) into naive-ui so the
 * admin panel inherits the same palette instead of naive-ui's defaults.
 * Hex values are duplicated here only because naive-ui takes JS objects.
 */

const pink = {
  300: '#f9a8d4',
  400: '#f472b6',
  500: '#ec4899',
  600: '#db2777',
  700: '#be185d',
}

const shared: GlobalThemeOverrides['common'] = {
  primaryColor: pink[500],
  primaryColorHover: pink[400],
  primaryColorPressed: pink[600],
  primaryColorSuppl: pink[400],
  infoColor: '#60a5fa',
  successColor: '#22c55e',
  warningColor: '#f59e0b',
  errorColor: '#ef4444',
  borderRadius: '10px',
  borderRadiusSmall: '6px',
  fontFamily:
    "'Public Sans', 'Noto Sans SC', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif",
  fontFamilyMono: "'JetBrains Mono', ui-monospace, SFMono-Regular, Consolas, monospace",
  fontSize: '14px',
  lineHeight: '1.6',
}

export const darkOverrides: GlobalThemeOverrides = {
  common: {
    ...shared,
    bodyColor: '#09090b',
    cardColor: '#18181b',
    modalColor: '#18181b',
    popoverColor: '#18181b',
    tableColor: '#18181b',
    inputColor: '#111113',
    codeColor: '#111113',
    tagColor: '#27272a',
    tableHeaderColor: '#111113',
    hoverColor: '#1f1f23',
    textColorBase: '#fafafa',
    textColor1: '#fafafa',
    textColor2: '#a1a1aa',
    textColor3: '#71717a',
    borderColor: '#27272a',
    dividerColor: '#27272a',
    closeIconColor: '#71717a',
    scrollbarColor: '#3f3f46',
    scrollbarColorHover: '#52525b',
    boxShadow1: '0 1px 2px rgb(0 0 0 / 40%)',
    boxShadow2: '0 4px 16px -4px rgb(0 0 0 / 55%)',
    boxShadow3: '0 16px 48px -12px rgb(0 0 0 / 65%)',
  },
  Card: { borderColor: '#27272a' },
  Input: { borderHover: `1px solid ${pink[400]}`, borderFocus: `1px solid ${pink[500]}` },
  Menu: { itemTextColor: '#a1a1aa', itemIconColor: '#71717a' },
}

export const lightOverrides: GlobalThemeOverrides = {
  common: {
    ...shared,
    primaryColor: pink[600],
    primaryColorHover: pink[700],
    primaryColorPressed: pink[700],
    primaryColorSuppl: pink[500],
    bodyColor: '#fafafa',
    cardColor: '#ffffff',
    modalColor: '#ffffff',
    popoverColor: '#ffffff',
    tableColor: '#ffffff',
    inputColor: '#ffffff',
    codeColor: '#f4f4f5',
    tagColor: '#f4f4f5',
    tableHeaderColor: '#fafafa',
    hoverColor: '#f4f4f5',
    textColorBase: '#09090b',
    textColor1: '#09090b',
    textColor2: '#52525b',
    textColor3: '#71717a',
    borderColor: '#e4e4e7',
    dividerColor: '#e4e4e7',
    scrollbarColor: '#d4d4d8',
    scrollbarColorHover: '#a1a1aa',
  },
  Card: { borderColor: '#e4e4e7' },
  Input: { borderHover: `1px solid ${pink[600]}`, borderFocus: `1px solid ${pink[600]}` },
  Menu: { itemTextColor: '#52525b', itemIconColor: '#71717a' },
}
