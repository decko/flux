import { useState } from 'react';
import { createRoute } from '@tanstack/react-router';
import { Route as rootRoute } from './__root';
import { PipelineRunList } from '../components/PipelineRunList';

export const Route = createRoute({
  getParentRoute: () => rootRoute,
  path: '/pipeline-runs',
  component: PipelineRunsPage,
});

function PipelineRunsPage() {
  const [inputValue, setInputValue] = useState('');
  const [ticketId, setTicketId] = useState<string | undefined>(undefined);

  function applyFilter() {
    setTicketId(inputValue.trim() || undefined);
  }

  function handleKeyDown(e: React.KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'Enter') {
      applyFilter();
    }
  }

  return (
    <div>
      <h1 className="text-2xl font-bold text-gray-900">Pipeline Runs</h1>
      <p className="mt-2 text-gray-600">Monitor pipeline execution status.</p>

      <div className="mt-4 flex items-center gap-2">
        <input
          type="text"
          placeholder="Filter by ticket ID..."
          value={inputValue}
          onChange={(e) => setInputValue(e.target.value)}
          onKeyDown={handleKeyDown}
          className="block rounded-lg border border-gray-300 px-3 py-2 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
        />
        <button
          type="button"
          onClick={applyFilter}
          className="rounded-lg bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
        >
          Apply
        </button>
      </div>

      <div className="mt-6">
        <PipelineRunList ticketId={ticketId} />
      </div>
    </div>
  );
}

/** Exported for testing. */
export { PipelineRunsPage };
