import React from 'react';
import { NavLink } from 'react-router-dom';

// Sidebar navigation component with auto-collapse behavior

interface SidebarProps {
  onHoverChange?: (isHovered: boolean) => void;
}

interface NavItem {
  path: string;
  label: string;
  icon: React.ReactNode;
}

const navItems: NavItem[] = [
  {
    path: '/',
    label: 'Timeline',
    icon: (
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <path d="M2 12h3l2-4 3 8 4-12 5 6 3-2h3" />
      </svg>
    ),
  },
  {
    path: '/graph',
    label: 'Graph',
    icon: (
      <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><circle cx="5" cy="6" r="3"></circle><path d="M5 9v6"></path><circle cx="5" cy="18" r="3"></circle><path d="M12 3v18"></path><circle cx="19" cy="6" r="3"></circle><path d="M16 15.7A9 9 0 0 0 19 9"></path></svg>
    ),
  },
  {
    path: '/agents',
    label: 'Agents',
    icon: (
      <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><path d="M16.247 7.761a6 6 0 0 1 0 8.478"></path><path d="M19.075 4.933a10 10 0 0 1 0 14.134"></path><path d="M4.925 19.067a10 10 0 0 1 0-14.134"></path><path d="M7.753 16.239a6 6 0 0 1 0-8.478"></path><circle cx="12" cy="12" r="2"></circle></svg>
    ),
  },
  {
    path: '/dashboards',
    label: 'Dashboards',
    icon: (
      <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><rect width="7" height="9" x="3" y="3" rx="1"></rect><rect width="7" height="5" x="14" y="3" rx="1"></rect><rect width="7" height="9" x="14" y="12" rx="1"></rect><rect width="7" height="5" x="3" y="16" rx="1"></rect></svg>
    ),
  },
];

const settingsItem: NavItem = {
  path: '/settings',
  label: 'Settings',
  icon: (
    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z" />
      <circle cx="12" cy="12" r="3" />
    </svg>
  ),
};

// CSS styles as a string for the component
const sidebarCSS = `
  @keyframes gradient-shift {
    0%, 100% { background-position: 0% 0%; }
    50% { background-position: 100% 100%; }
  }
  
  @keyframes levitate {
    0% { transform: translate(0px, 0px); }
    15% { transform: translate(0.5px, -1.5px); }
    30% { transform: translate(-0.3px, -2.2px); }
    45% { transform: translate(0.8px, -1px); }
    60% { transform: translate(-0.5px, -2.5px); }
    75% { transform: translate(0.2px, -0.8px); }
    90% { transform: translate(-0.4px, -1.8px); }
    100% { transform: translate(0px, 0px); }
  }

  .sidebar-container {
    width: 64px;
    min-width: 64px;
    height: 100vh;
    background-color: #0d0d0d;
    border-right: 1px solid #2a2a2a;
    display: flex;
    flex-direction: column;
    padding: 0 0 16px 0;
    transition: width 0.25s cubic-bezier(0.4, 0, 0.2, 1), min-width 0.25s cubic-bezier(0.4, 0, 0.2, 1);
    overflow: hidden;
    position: fixed;
    top: 0;
    left: 0;
    z-index: 50;
  }

  .sidebar-container:hover {
    width: 220px;
    min-width: 220px;
  }

  .sidebar-logo-container {
    padding: 16px 12px 14px;
    border-bottom: 1px solid #2a2a2a;
    margin-bottom: 16px;
    display: flex;
    align-items: center;
    gap: 12px;
    min-height: 73px;
  }

  .sidebar-logo-icon {
    width: 40px;
    height: 40px;
    min-width: 40px;
    flex-shrink: 0;
    border-radius: 12px;
    display: flex;
    align-items: center;
    justify-content: center;
    color: white;
    box-shadow: 0 10px 15px -3px rgba(99, 102, 241, 0.2);
    background: linear-gradient(135deg, #6366f1 0%, #8b5cf6 25%, #a855f7 50%, #7c3aed 75%, #6366f1 100%);
    background-size: 200% 200%;
    animation: gradient-shift 15s ease-in-out infinite;
    transition: transform 0.25s cubic-bezier(0.4, 0, 0.2, 1);
  }

  .sidebar-logo-text {
    color: #ffffff;
    font-size: 20px;
    font-weight: 700;
    letter-spacing: 0.5px;
    margin: 0;
    white-space: nowrap;
    opacity: 0;
    transform: translateX(-10px);
    transition: opacity 0.2s ease 0.05s, transform 0.2s ease 0.05s;
  }

  .sidebar-container:hover .sidebar-logo-text {
    opacity: 1;
    transform: translateX(0);
  }

  .sidebar-nav {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 0;
    flex: 1;
  }

  .sidebar-nav-settings {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 0;
    margin-top: auto;
  }

  .sidebar-link {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 12px 22px;
    border-radius: 0;
    text-decoration: none;
    color: #a0a0a0;
    font-size: 14px;
    font-weight: 500;
    transition: background-color 0.15s ease, color 0.15s ease;
    position: relative;
    width: 100%;
    margin: 0;
    white-space: nowrap;
    overflow: hidden;
  }

  .sidebar-link:hover {
    background-color: #161616;
    color: #ffffff;
  }

  .sidebar-link.active {
    background-color: #1a1a1a;
    color: #a855f7;
    border-right: 3px solid #a855f7;
  }

  .sidebar-link.active:hover {
    color: #a855f7;
  }

  .sidebar-link-icon {
    display: flex;
    align-items: center;
    min-width: 20px;
    transition: color 0.15s ease;
  }

  .sidebar-link.active .sidebar-link-icon {
    color: #a855f7;
  }

  .sidebar-link-label {
    opacity: 0;
    transform: translateX(-10px);
    transition: opacity 0.2s ease 0.05s, transform 0.2s ease 0.05s;
  }

  .sidebar-container:hover .sidebar-link-label {
    opacity: 1;
    transform: translateX(0);
  }
`;

export function Sidebar({ onHoverChange }: SidebarProps) {
  return (
    <aside
      className="sidebar-container"
      onMouseEnter={() => onHoverChange?.(true)}
      onMouseLeave={() => onHoverChange?.(false)}
    >
      <style>{sidebarCSS}</style>
      
      {/* Logo */}
      <div className="sidebar-logo-container">
        <div className="sidebar-logo-icon">
          <svg
            width="24"
            height="24"
            viewBox="0 0 24 24"
            fill="none"
            xmlns="http://www.w3.org/2000/svg"
            style={{ animation: 'levitate 10s ease-in-out infinite' }}
          >
            <path d="M12 2C7.58172 2 4 5.58172 4 10V19C4 20.6569 5.34315 22 7 22C7.63228 22 8.21952 21.7909 8.70773 21.4312L10.5858 19.5531C10.9609 19.1781 11.4696 18.9674 12 18.9674C12.5304 18.9674 13.0391 19.1781 13.4142 19.5531L15.2923 21.4312C15.7805 21.7909 16.3677 22 17 22C18.6569 22 20 20.6569 20 19V10C20 5.58172 16.4183 2 12 2Z" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
            <path d="M9 10H9.01" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
            <path d="M15 10H15.01" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
          </svg>
        </div>
        <h1 className="sidebar-logo-text">Spectre</h1>
      </div>

      {/* Main Navigation */}
      <nav className="sidebar-nav">
        {navItems.map((item) => (
          <NavLink
            key={item.path}
            to={item.path}
            className={({ isActive }) => `sidebar-link ${isActive ? 'active' : ''}`}
          >
            <div className="sidebar-link-icon">
              {item.icon}
            </div>
            <span className="sidebar-link-label">{item.label}</span>
          </NavLink>
        ))}
      </nav>

      {/* Settings Navigation */}
      <nav className="sidebar-nav-settings">
        <NavLink
          to={settingsItem.path}
          className={({ isActive }) => `sidebar-link ${isActive ? 'active' : ''}`}
        >
          <div className="sidebar-link-icon">
            {settingsItem.icon}
          </div>
          <span className="sidebar-link-label">{settingsItem.label}</span>
        </NavLink>
      </nav>
    </aside>
  );
}

export default Sidebar;
