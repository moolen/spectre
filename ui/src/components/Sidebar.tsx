import React from 'react';
import { NavLink } from 'react-router-dom';
import { Icon, GhostMark, IconName } from './Common/Icon';

// Slim icon rail. Labels are shown as tooltips so the rail never changes width
// (the old hover-expand pushed and re-laid out the whole page).

export const SIDEBAR_WIDTH = 56;

interface NavItem {
  path: string;
  label: string;
  icon: IconName;
}

const navItems: NavItem[] = [
  { path: '/', label: 'Timeline', icon: 'timeline' },
  { path: '/graph', label: 'Graph', icon: 'graph' },
];

const settingsItem: NavItem = { path: '/settings', label: 'Settings', icon: 'settings' };

const sidebarCSS = `
  .sidebar-container {
    width: ${SIDEBAR_WIDTH}px;
    min-width: ${SIDEBAR_WIDTH}px;
    height: 100vh;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
    padding: 12px 0;
    background: var(--color-app-bg);
    border-right: 1px solid var(--color-border-soft);
    position: relative;
    z-index: 50;
  }

  .sidebar-logo {
    width: 32px;
    height: 32px;
    border-radius: 9px;
    display: grid;
    place-items: center;
    margin-bottom: 16px;
    color: #fff;
    background: linear-gradient(155deg, #f4a77b 0%, #e07a47 50%, #b9532a 100%);
    box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.25), 0 0 0 1px rgba(224, 122, 71, 0.35);
  }

  .sidebar-nav {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
  }

  .sidebar-spacer { flex: 1; }

  .sidebar-link {
    width: 36px;
    height: 36px;
    border-radius: 8px;
    display: grid;
    place-items: center;
    position: relative;
    color: var(--color-text-faint);
    transition: color 0.15s ease, background-color 0.15s ease;
  }

  .sidebar-link:hover {
    color: var(--color-text-primary);
    background: var(--color-surface-muted);
  }

  .sidebar-link.active {
    color: var(--color-text-primary);
    background: var(--color-surface-active);
  }

  .sidebar-link.active::before {
    content: '';
    position: absolute;
    left: -10px;
    top: 9px;
    bottom: 9px;
    width: 2px;
    border-radius: 2px;
    background: var(--color-accent);
  }

  .sidebar-link-label {
    position: absolute;
    left: calc(100% + 10px);
    top: 50%;
    transform: translate(-4px, -50%);
    padding: 4px 8px;
    border-radius: 6px;
    background: var(--color-surface-active);
    border: 1px solid var(--color-border-strong);
    color: var(--color-text-primary);
    font-size: 12px;
    font-weight: 500;
    white-space: nowrap;
    pointer-events: none;
    opacity: 0;
    transition: opacity 0.12s ease, transform 0.12s ease;
    box-shadow: var(--shadow-popover);
  }

  .sidebar-link:hover .sidebar-link-label,
  .sidebar-link:focus-visible .sidebar-link-label {
    opacity: 1;
    transform: translate(0, -50%);
  }
`;

const RailLink: React.FC<{ item: NavItem }> = ({ item }) => (
  <NavLink
    to={item.path}
    end={item.path === '/'}
    aria-label={item.label}
    className={({ isActive }) => `sidebar-link ${isActive ? 'active' : ''}`}
  >
    <Icon name={item.icon} size={18} />
    <span className="sidebar-link-label">{item.label}</span>
  </NavLink>
);

export function Sidebar() {
  return (
    <aside className="sidebar-container">
      <style>{sidebarCSS}</style>

      <div className="sidebar-logo" title="Spectre">
        <GhostMark size={18} />
      </div>

      <nav className="sidebar-nav">
        {navItems.map((item) => (
          <RailLink key={item.path} item={item} />
        ))}
      </nav>

      <div className="sidebar-spacer" />

      <nav className="sidebar-nav sidebar-nav-settings">
        <RailLink item={settingsItem} />
      </nav>
    </aside>
  );
}

export default Sidebar;
