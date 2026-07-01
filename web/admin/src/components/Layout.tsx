import { NavLink, Outlet } from 'react-router-dom'
import { ScrollArea } from './ScrollArea'

const links = [
  { to: '/', label: 'Overview', end: true },
  { to: '/jobs', label: 'Settlement jobs' },
  { to: '/ledger', label: 'Ledger' },
  { to: '/reconciliation', label: 'Reconciliation' },
  { to: '/audit', label: 'Audit log' },
]

export function Layout() {
  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="sidebar-brand">
          <h1>Arrakin</h1>
          <p>Settlement operations</p>
        </div>
        <nav>
          {links.map((link) => (
            <NavLink
              key={link.to}
              to={link.to}
              end={link.end}
              className={({ isActive }) => (isActive ? 'active' : undefined)}
            >
              {link.label}
            </NavLink>
          ))}
        </nav>
      </aside>
      <main className="main">
        <Outlet />
      </main>
    </div>
  )
}
