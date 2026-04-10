import { describe, expect, it, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import App from './App';
import { SettingsProvider } from './hooks/useSettings';
import { BetaFeaturesProvider } from './contexts/BetaFeaturesContext';

vi.mock('./pages/TimelinePage', () => ({ default: () => <div>Timeline Page</div> }));
vi.mock('./pages/NamespaceGraphPage', () => ({ default: () => <div>Graph Page</div> }));
vi.mock('./pages/AgentsPage', () => ({ default: () => <div>Agents Page</div> }));
vi.mock('./pages/SettingsPage', () => ({ default: () => <div>Settings Page</div> }));

describe('App routes', () => {
  beforeEach(() => {
    window.history.pushState({}, '', '/?beta=true');
  });

  it('does not register the observatory route', () => {
    render(
      <MemoryRouter initialEntries={['/observatory']}>
        <BetaFeaturesProvider>
          <SettingsProvider>
            <App />
          </SettingsProvider>
        </BetaFeaturesProvider>
      </MemoryRouter>
    );

    expect(screen.queryByText('Observatory Page')).not.toBeInTheDocument();
  });

  it('does not register the integrations route', () => {
    render(
      <MemoryRouter initialEntries={['/integrations']}>
        <BetaFeaturesProvider>
          <SettingsProvider>
            <App />
          </SettingsProvider>
        </BetaFeaturesProvider>
      </MemoryRouter>
    );

    expect(screen.queryByText('Integrations Page')).not.toBeInTheDocument();
  });
});
