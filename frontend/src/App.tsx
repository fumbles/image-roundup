import { useEffect, useState } from 'react'
import { BrowserRouter, Routes, Route, NavLink, Navigate, useLocation } from 'react-router-dom'
import {
  Header,
  HeaderGlobalAction,
  HeaderGlobalBar,
  HeaderName,
  HeaderNavigation,
  HeaderMenuItem,
  Content,
  Theme,
} from '@carbon/react'
import { LogoGithub, Moon, Sun } from '@carbon/icons-react'
import { getSettings, putSettings } from './api'
import OverviewPage from './pages/OverviewPage'
import ImagesPage from './pages/ImagesPage'
import RegistriesPage from './pages/RegistriesPage'
import SettingsPage from './pages/SettingsPage'
import { SETTINGS_SAVED_EVENT } from './theme'
import type { Settings } from './types'

type CarbonTheme = 'white' | 'g10' | 'g90' | 'g100'

const GITHUB_REPO_URL = 'https://github.com/fumbles/image-roundup'

function prefersDarkScheme() {
  return window.matchMedia?.('(prefers-color-scheme: dark)').matches ?? false
}

function resolveCarbonTheme(theme: Settings['theme']): CarbonTheme {
  if (theme === 'dark') return 'g100'
  if (theme === 'light') return 'white'
  return prefersDarkScheme() ? 'g100' : 'white'
}

// Carbon's HeaderMenuItem uses `as` (not `element`) to swap the root element.
// We wrap NavLink so the `isActive` class is applied correctly.
function NavItem({ to, children }: { to: string; children: React.ReactNode }) {
  const location = useLocation()
  const active = location.pathname.startsWith(to)
  return (
    <HeaderMenuItem
      as={NavLink}
      to={to}
      isCurrentPage={active}
    >
      {children}
    </HeaderMenuItem>
  )
}

export default function App() {
  return (
    <BrowserRouter>
      <AppShell />
    </BrowserRouter>
  )
}

function AppShell() {
  const [theme, setTheme] = useState<CarbonTheme>(() => resolveCarbonTheme('system'))
  const [settings, setSettings] = useState<Settings | null>(null)

  useEffect(() => {
    let mounted = true

    const applyTheme = (nextTheme: Settings['theme']) => {
      const resolved = resolveCarbonTheme(nextTheme)
      document.documentElement.dataset.theme = resolved
      setTheme(resolved)
    }

    getSettings()
      .then((settings) => {
        if (mounted) {
          setSettings(settings)
          applyTheme(settings.theme)
        }
      })
      .catch(() => {
        if (mounted) applyTheme('system')
      })

    const handleSettingsSaved = (event: Event) => {
      const detail = (event as CustomEvent<Settings>).detail
      if (detail?.theme) {
        setSettings(detail)
        applyTheme(detail.theme)
      }
    }

    const media = window.matchMedia?.('(prefers-color-scheme: dark)')
    const handleSystemThemeChange = () => {
      getSettings()
        .then((settings) => {
          setSettings(settings)
          if (settings.theme === 'system') applyTheme('system')
        })
        .catch(() => applyTheme('system'))
    }

    window.addEventListener(SETTINGS_SAVED_EVENT, handleSettingsSaved)
    media?.addEventListener('change', handleSystemThemeChange)

    return () => {
      mounted = false
      window.removeEventListener(SETTINGS_SAVED_EVENT, handleSettingsSaved)
      media?.removeEventListener('change', handleSystemThemeChange)
    }
  }, [])

  const toggleTheme = async () => {
    if (!settings) return
    const nextTheme: Settings['theme'] = theme === 'g100' ? 'light' : 'dark'
    const updated = await putSettings({ ...settings, theme: nextTheme })
    setSettings(updated)
    window.dispatchEvent(new CustomEvent(SETTINGS_SAVED_EVENT, { detail: updated }))
  }
  const openGithub = () => window.open(GITHUB_REPO_URL, '_blank', 'noopener,noreferrer')

  const dark = theme === 'g100'
  const themeActionLabel = !settings
    ? 'Loading theme settings'
    : dark ? 'Use light theme' : 'Use dark theme'

  return (
    <Theme as="div" theme={theme} className="ir-app-shell">
      <Header aria-label="Image Roundup" className="ir-header">
        <HeaderName href="/" prefix="" className="ir-header-name">
          <span className="ir-brand-mark" aria-hidden="true" />
          <span>Image Roundup</span>
        </HeaderName>
        <HeaderNavigation aria-label="Main navigation">
          <NavItem to="/overview">Overview</NavItem>
          <NavItem to="/images">Images</NavItem>
          <NavItem to="/registries">Registries</NavItem>
          <NavItem to="/settings">Settings</NavItem>
        </HeaderNavigation>
        <HeaderGlobalBar>
          <HeaderGlobalAction
            aria-label={themeActionLabel}
            tooltipAlignment="end"
            onClick={toggleTheme}
          >
            {dark ? <Sun size={20} /> : <Moon size={20} />}
          </HeaderGlobalAction>
          <HeaderGlobalAction
            aria-label="Open GitHub repository"
            tooltipAlignment="end"
            onClick={openGithub}
          >
            <LogoGithub size={20} />
          </HeaderGlobalAction>
        </HeaderGlobalBar>
      </Header>

      <Content className="ir-content">
        <Routes>
          <Route path="/" element={<Navigate to="/overview" replace />} />
          <Route path="/overview" element={<OverviewPage />} />
          <Route path="/images" element={<ImagesPage />} />
          <Route path="/registries" element={<RegistriesPage />} />
          <Route path="/settings" element={<SettingsPage />} />
        </Routes>
      </Content>
    </Theme>
  )
}
