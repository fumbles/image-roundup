import { BrowserRouter, Routes, Route, NavLink, Navigate, useLocation } from 'react-router-dom'
import {
  Header,
  HeaderName,
  HeaderNavigation,
  HeaderMenuItem,
  Content,
} from '@carbon/react'
import OverviewPage from './pages/OverviewPage'
import ImagesPage from './pages/ImagesPage'
import RegistriesPage from './pages/RegistriesPage'
import SettingsPage from './pages/SettingsPage'

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
  return (
    <>
      <Header aria-label="Image Roundup">
        <HeaderName href="/" prefix="">
          Image Roundup
        </HeaderName>
        <HeaderNavigation aria-label="Main navigation">
          <NavItem to="/overview">Overview</NavItem>
          <NavItem to="/images">Images</NavItem>
          <NavItem to="/registries">Registries</NavItem>
          <NavItem to="/settings">Settings</NavItem>
        </HeaderNavigation>
      </Header>

      <Content style={{ paddingTop: 48 }}>
        <Routes>
          <Route path="/" element={<Navigate to="/overview" replace />} />
          <Route path="/overview" element={<OverviewPage />} />
          <Route path="/images" element={<ImagesPage />} />
          <Route path="/registries" element={<RegistriesPage />} />
          <Route path="/settings" element={<SettingsPage />} />
        </Routes>
      </Content>
    </>
  )
}
