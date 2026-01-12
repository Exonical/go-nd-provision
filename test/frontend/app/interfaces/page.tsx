'use client';

import Link from 'next/link';
import { useEffect, useMemo, useState } from 'react';
import { computeNodesAPI, ComputeNode, ComputeNodeInterface } from '@/lib/api';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';

type InterfaceRow = {
  node: ComputeNode;
  iface: ComputeNodeInterface;
};

export default function InterfacesPage() {
  const [rows, setRows] = useState<InterfaceRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [filter, setFilter] = useState('');

  const load = async () => {
    try {
      setLoading(true);
      const nodes = await computeNodesAPI.list();

      const perNode = await Promise.all(
        nodes.map(async (node) => {
          try {
            const ifaces = await computeNodesAPI.getInterfaces(node.id);
            return ifaces.map((iface) => ({ node, iface }));
          } catch {
            return [] as InterfaceRow[];
          }
        })
      );

      setRows(perNode.flat());
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load interfaces');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, []);

  const filtered = useMemo(() => {
    const q = filter.trim().toLowerCase();
    if (!q) return rows;

    return rows.filter(({ node, iface }) => {
      return [
        node.name,
        node.hostname,
        node.ip_address,
        iface.role,
        iface.hostname,
        iface.ip_address,
        iface.mac_address,
      ].some((v) => (v || '').toLowerCase().includes(q));
    });
  }, [filter, rows]);

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-foreground">Interfaces</h1>
          <p className="text-muted-foreground">Cross-node view of compute/storage interfaces</p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" asChild>
            <Link href="/switch-ports">Bulk assign ports</Link>
          </Button>
          <Button variant="outline" onClick={load}>Refresh</Button>
        </div>
      </div>

      {error && (
        <div className="bg-destructive/20 border border-destructive text-destructive-foreground px-4 py-3 rounded-md">
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
              placeholder="Filter by node / role / ip / mac..."
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              className="max-w-sm"
            />
            <span className="text-sm text-muted-foreground">
              {filtered.length} interface{filtered.length === 1 ? '' : 's'}
            </span>
          </div>

          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Node</TableHead>
                <TableHead>Role</TableHead>
                <TableHead>Hostname</TableHead>
                <TableHead>IP</TableHead>
                <TableHead>MAC</TableHead>
                <TableHead>Port mappings</TableHead>
                <TableHead className="text-right">View</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading ? (
                <TableRow>
                  <TableCell colSpan={7} className="text-muted-foreground">
                    Loading...
                  </TableCell>
                </TableRow>
              ) : filtered.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={7} className="text-muted-foreground">
                    No interfaces
                  </TableCell>
                </TableRow>
              ) : (
                filtered.map(({ node, iface }) => {
                  const mappingCount = iface.port_mappings?.length || 0;
                  return (
                    <TableRow key={`${node.id}:${iface.id}`}>
                      <TableCell>
                        <Link
                          href={`/compute-nodes/${encodeURIComponent(node.name)}`}
                          className="text-primary hover:underline"
                        >
                          {node.name}
                        </Link>
                      </TableCell>
                      <TableCell>
                        <Badge variant={iface.role === 'compute' ? 'compute' : 'storage'}>
                          {iface.role}
                        </Badge>
                      </TableCell>
                      <TableCell>{iface.hostname || '-'}</TableCell>
                      <TableCell className="font-mono">{iface.ip_address || '-'}</TableCell>
                      <TableCell className="font-mono">{iface.mac_address || '-'}</TableCell>
                      <TableCell>{mappingCount}</TableCell>
                      <TableCell className="text-right">
                        <Button variant="ghost" size="sm" asChild>
                          <Link href={`/compute-nodes/${encodeURIComponent(node.name)}?tab=interfaces`}>
                            Details
                          </Link>
                        </Button>
                      </TableCell>
                    </TableRow>
                  );
                })
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  );
}
