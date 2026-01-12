'use client';

import Link from 'next/link';
import { useParams } from 'next/navigation';
import { useEffect, useMemo, useState } from 'react';
import { switchesAPI, portsAPI, Switch } from '@/lib/api';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';

export default function FabricSwitchesPage() {
  const params = useParams();
  const fabricIdOrName = decodeURIComponent(params.id as string);

  const [switches, setSwitches] = useState<Switch[]>([]);
  const [filter, setFilter] = useState('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [syncing, setSyncing] = useState(false);

  const loadSwitches = async () => {
    try {
      setLoading(true);
      const data = await switchesAPI.list(fabricIdOrName);
      setSwitches(data);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load switches');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadSwitches();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [fabricIdOrName]);

  const filtered = useMemo(() => {
    const q = filter.trim().toLowerCase();
    if (!q) return switches;
    return switches.filter((s) =>
      [s.name, s.serial_number, s.model, s.ip_address].some((v) => (v || '').toLowerCase().includes(q))
    );
  }, [filter, switches]);

  const handleSyncSwitches = async () => {
    try {
      setSyncing(true);
      await switchesAPI.sync(fabricIdOrName);
      await loadSwitches();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to sync switches');
    } finally {
      setSyncing(false);
    }
  };

  const handleSyncPorts = async () => {
    try {
      setSyncing(true);
      await portsAPI.syncAll(fabricIdOrName);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to sync ports');
    } finally {
      setSyncing(false);
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <div className="flex items-center gap-2 text-sm text-muted-foreground mb-2">
            <Link href="/fabrics" className="hover:text-foreground">Fabrics</Link>
            <span>/</span>
            <Link href={`/fabrics/${encodeURIComponent(fabricIdOrName)}`} className="hover:text-foreground">{fabricIdOrName}</Link>
            <span>/</span>
            <span className="text-foreground">Switches</span>
          </div>
          <h1 className="text-2xl font-bold text-foreground">Switches</h1>
          <p className="text-muted-foreground">{fabricIdOrName}</p>
        </div>
        <div className="flex gap-2">
          <Button onClick={handleSyncSwitches} disabled={syncing}>
            {syncing ? 'Syncing...' : 'Sync Switches'}
          </Button>
          <Button variant="secondary" onClick={handleSyncPorts} disabled={syncing}>
            Sync Ports
          </Button>
        </div>
      </div>

      {error && (
        <div className="bg-destructive/20 border border-destructive text-destructive px-4 py-3 rounded-md">
          {error}
          <button onClick={() => setError(null)} className="ml-3 underline">
            Dismiss
          </button>
        </div>
      )}

      <Card>
        <CardHeader>
          <CardTitle>Inventory</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex items-center justify-between gap-4 mb-4">
            <Input
              placeholder="Filter by name / serial / model / ip..."
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              className="max-w-sm"
            />
            <span className="text-sm text-muted-foreground">
              {filtered.length} switch{filtered.length === 1 ? '' : 'es'}
            </span>
          </div>

          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>IP</TableHead>
                <TableHead>Model</TableHead>
                <TableHead>Serial</TableHead>
                <TableHead className="text-right">View</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading ? (
                <TableRow>
                  <TableCell colSpan={5} className="text-muted-foreground">
                    Loading...
                  </TableCell>
                </TableRow>
              ) : filtered.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={5} className="text-muted-foreground">
                    No switches
                  </TableCell>
                </TableRow>
              ) : (
                filtered.map((sw) => (
                  <TableRow key={sw.id}>
                    <TableCell>
                      <Link
                        href={`/fabrics/${encodeURIComponent(fabricIdOrName)}/switches/${encodeURIComponent(sw.id)}`}
                        className="text-primary hover:underline"
                      >
                        {sw.name}
                      </Link>
                    </TableCell>
                    <TableCell className="font-mono">{sw.ip_address || '-'}</TableCell>
                    <TableCell>{sw.model || '-'}</TableCell>
                    <TableCell className="font-mono">{sw.serial_number || '-'}</TableCell>
                    <TableCell className="text-right">
                      <Button variant="ghost" size="sm" asChild>
                        <Link href={`/fabrics/${encodeURIComponent(fabricIdOrName)}/switches/${encodeURIComponent(sw.id)}`}>
                          Details
                        </Link>
                      </Button>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  );
}
