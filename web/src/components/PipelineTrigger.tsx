import { useState } from 'react';

export interface PipelineTriggerProps {
  runId: string;
}

/** Read JWT token from localStorage (set by login flow). */
function getToken(): string | null {
  try {
    return localStorage.getItem('flux_token');
  } catch {
    return null;
  }
}

/**
 * PipelineTrigger renders a button that POSTs to
 * `/api/v1/pipeline-runs/{runId}/trigger` to trigger a pipeline run.
 *
 * States: idle → loading (spinner + disabled) → success (confirmation) or error (alert).
 * The button re-enables after the request completes regardless of outcome.
 */
export function PipelineTrigger({ runId }: PipelineTriggerProps) {
  const [status, setStatus] = useState<'idle' | 'loading' | 'success' | 'error'>('idle');
  const [errorMessage, setErrorMessage] = useState<string>('');

  async function handleTrigger() {
    setStatus('loading');
    setErrorMessage('');

    try {
      const token = getToken();
      const headers: Record<string, string> = {
        'Content-Type': 'application/json',
      };
      if (token) {
        headers['Authorization'] = `Bearer ${token}`;
      }

      const res = await fetch(`/api/v1/pipeline-runs/${runId}/trigger`, {
        method: 'POST',
        headers,
      });

      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        throw new Error((body as Record<string, unknown>).error as string || res.statusText);
      }

      setStatus('success');
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      setErrorMessage(message);
      setStatus('error');
    }
  }

  return (
    <div className="space-y-2">
      <button
        type="button"
        disabled={status === 'loading'}
        onClick={handleTrigger}
        className="inline-flex items-center gap-1.5 rounded-md bg-blue-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-50"
      >
        {status === 'loading' && (
          <span
            className="inline-block h-3.5 w-3.5 animate-spin rounded-full border-2 border-white border-t-transparent"
            aria-label="triggering"
          />
        )}
        Trigger Pipeline
      </button>

      {status === 'error' && (
        <div
          role="alert"
          className="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-800"
        >
          {errorMessage}
        </div>
      )}

      {status === 'success' && (
        <p className="text-sm font-medium text-green-700" role="status">
          Pipeline triggered
        </p>
      )}
    </div>
  );
}
