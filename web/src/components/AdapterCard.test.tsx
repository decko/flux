import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { AdapterCard, type AdapterCardProps } from './AdapterCard';

const defaultProps: AdapterCardProps = {
  type: 'github',
  name: 'flux-org/flux-core',
  health: 'healthy',
};

function renderCard(props: Partial<AdapterCardProps> = {}) {
  return render(<AdapterCard {...defaultProps} {...props} />);
}

describe('AdapterCard', () => {
  // --- Rendering ---

  it('renders the adapter type', () => {
    renderCard({ type: 'jira' });
    expect(screen.getByText(/jira/i)).toBeInTheDocument();
  });

  it('renders the adapter name', () => {
    renderCard({ name: 'my-org/my-repo' });
    expect(screen.getByText(/my-org\/my-repo/)).toBeInTheDocument();
  });

  it('shows the type and name combination', () => {
    renderCard({ type: 'linear', name: 'team/backend' });
    expect(screen.getByText(/linear/i)).toBeInTheDocument();
    expect(screen.getByText(/team\/backend/)).toBeInTheDocument();
  });

  // --- Health indicator ---

  it('shows green health indicator when healthy', () => {
    renderCard({ health: 'healthy' });
    const indicator = screen.getByLabelText('Health: healthy');
    expect(indicator).toHaveClass(/bg-green/);
  });

  it('shows red health indicator when unhealthy', () => {
    renderCard({ health: 'unhealthy' });
    const indicator = screen.getByLabelText('Health: unhealthy');
    expect(indicator).toHaveClass(/bg-red/);
  });

  it('shows gray health indicator when unknown', () => {
    renderCard({ health: 'unknown' });
    const indicator = screen.getByLabelText('Health: unknown');
    expect(indicator).toHaveClass(/bg-gray/);
  });

  it('visually hides the health dot text so it is screen-reader accessible', () => {
    renderCard();
    const dot = screen.getByLabelText('Health: healthy');
    expect(dot).toBeInTheDocument();
    // The dot element itself should not contain visible text for the status.
    expect(dot.textContent?.trim()).toBeFalsy();
  });

  it.each(['healthy', 'unhealthy', 'unknown'] as const)(
    'renders correctly for health=%s',
    (health) => {
      const { container } = renderCard({ health });
      // Should always render the type and name.
      expect(screen.getByText(new RegExp(defaultProps.type, 'i'))).toBeInTheDocument();
      expect(
        screen.getByText(new RegExp(defaultProps.name, 'i')),
      ).toBeInTheDocument();
      // Should not throw regardless of health value.
      expect(container).toBeTruthy();
    },
  );
});
