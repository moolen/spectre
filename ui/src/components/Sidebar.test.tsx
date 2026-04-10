import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { Sidebar } from './Sidebar';

describe('Sidebar', () => {
  it('does not show observatory or integrations navigation items', () => {
    window.history.pushState({}, '', '/?beta=true');

    render(
      <MemoryRouter>
        <Sidebar />
      </MemoryRouter>
    );

    expect(screen.queryByText('Observatory')).not.toBeInTheDocument();
    expect(screen.queryByText('Integrations')).not.toBeInTheDocument();
  });
});
