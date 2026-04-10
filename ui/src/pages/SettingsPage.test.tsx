import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import SettingsPage from './SettingsPage';
import { SettingsProvider } from '../hooks/useSettings';

describe('SettingsPage', () => {
	it('does not render observatory-specific preferences', () => {
		render(
			<SettingsProvider>
				<SettingsPage />
			</SettingsProvider>
		);

		expect(screen.queryByText('Observatory')).not.toBeInTheDocument();
		expect(screen.queryByText('Default Node Types')).not.toBeInTheDocument();
	});
});
