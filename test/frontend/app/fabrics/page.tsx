'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { fabricsAPI, Fabric } from '@/lib/api';

export default function FabricsPage() {
  const [fabrics, setFabrics] = useState<Fabric[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [syncing, setSyncing] = useState(false);

  const loadFabrics = async () => {
    try {
      setLoading(true);
      const data = await fabricsAPI.list();
      setFabrics(data);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load fabrics');
    } finally {
      setLoading(false);
    }
  };

  const handleSync = async () => {
    try {
      setSyncing(true);
      await fabricsAPI.sync();
      await loadFabrics();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to sync fabrics');
    } finally {
      setSyncing(false);
    }
  };

  useEffect(() => {
    loadFabrics();
  }, []);

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-zinc-900 dark:text-white">Fabrics</h1>
          <p className="text-zinc-600 dark:text-zinc-400">Nexus Dashboard fabrics</p>
        </div>
        <button
          onClick={handleSync}
          disabled={syncing}
          className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50"
        >
          {syncing ? 'Syncing...' : 'Sync from NDFC'}
        </button>
      </div>

      {error && (
        <div className="p-4 bg-red-100 dark:bg-red-900/20 text-red-700 dark:text-red-400 rounded-lg">
          {error}
        </div>
      )}

      {loading ? (
        <div className="text-zinc-600 dark:text-zinc-400">Loading...</div>
      ) : fabrics.length === 0 ? (
        <div className="text-zinc-600 dark:text-zinc-400">No fabrics found. Click &quot;Sync from NDFC&quot; to fetch fabrics.</div>
      ) : (
        <div className="grid gap-4">
          {fabrics.map((fabric) => (
            <Link
              key={fabric.id}
              href={`/fabrics/${encodeURIComponent(fabric.name)}`}
              className="block p-4 bg-white dark:bg-zinc-900 rounded-lg border border-zinc-200 dark:border-zinc-800 hover:border-blue-500 transition-colors"
            >
              <div className="flex items-center justify-between">
                <div>
                  <h2 className="text-lg font-semibold text-zinc-900 dark:text-white">{fabric.name}</h2>
                  <p className="text-sm text-zinc-600 dark:text-zinc-400">Type: {fabric.type || 'N/A'}</p>
                </div>
                <span className="text-zinc-400">→</span>
              </div>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
