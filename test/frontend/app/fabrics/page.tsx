'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { fabricsAPI, Fabric } from '@/lib/api';
import { Button } from '@/components/ui/button';
import { Card, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { ChevronRight } from 'lucide-react';

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
          <h1 className="text-2xl font-bold text-foreground">Fabrics</h1>
          <p className="text-muted-foreground">Nexus Dashboard fabrics</p>
        </div>
        <Button onClick={handleSync} disabled={syncing}>
          {syncing ? 'Syncing...' : 'Sync from NDFC'}
        </Button>
      </div>

      {error && (
        <div className="p-4 bg-destructive/20 text-destructive rounded-lg">
          {error}
        </div>
      )}

      {loading ? (
        <div className="text-muted-foreground">Loading...</div>
      ) : fabrics.length === 0 ? (
        <div className="text-muted-foreground">No fabrics found. Click &quot;Sync from NDFC&quot; to fetch fabrics.</div>
      ) : (
        <div className="grid gap-4">
          {fabrics.map((fabric) => (
            <Link key={fabric.id} href={`/fabrics/${encodeURIComponent(fabric.name)}`}>
              <Card className="hover:border-primary transition-colors cursor-pointer">
                <CardHeader className="flex-row items-center justify-between py-4">
                  <div>
                    <CardTitle className="text-lg">{fabric.name}</CardTitle>
                    <CardDescription>Type: {fabric.type || 'N/A'}</CardDescription>
                  </div>
                  <ChevronRight className="h-5 w-5 text-muted-foreground" />
                </CardHeader>
              </Card>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
