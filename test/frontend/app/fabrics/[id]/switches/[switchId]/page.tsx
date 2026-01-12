'use client';

import Link from 'next/link';
import { useParams } from 'next/navigation';
import { useEffect, useMemo, useState } from 'react';
import { switchesAPI, portsAPI, Switch, SwitchPort, PortMapping } from '@/lib/api';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';

export default function SwitchDetailPage() {
  const params = useParams();
  const fabricIdOrName = decodeURIComponent(params.id as string);
  const switchIdOrName = decodeURIComponent(params.switchId as string);

  const [sw, setSw] = useState<Switch | null>(null);
  const [ports, setPorts] = useState<SwitchPort[]>([]);
  const [mappings, setMappings] = useState<PortMapping[]>([]);

  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [portFilter, setPortFilter] = useState('');

  const loadAll = async () => {
    try {
      setLoading(true);
      const [swData, portsData, mappingData] = await Promise.all([
        switchesAPI.get(fabricIdOrName, switchIdOrName),
        portsAPI.list(fabricIdOrName, switchIdOrName),
        portsAPI.getMappingsBySwitch(switchIdOrName).catch(() => []),
      ]);
      setSw(swData);
      setPorts(portsData);
      setMappings(mappingData);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load switch');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadAll();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [fabricIdOrName, switchIdOrName]);

  const mappingByPortId = useMemo(() => {
    const map = new Map<string, PortMapping>();
    for (const m of mappings) {
      map.set(m.switch_port_id, m);
    }
    return map;
  }, [mappings]);

  const filteredPorts = useMemo(() => {
    const q = portFilter.trim().toLowerCase();
    const list = [...ports].sort((a, b) => a.name.localeCompare(b.name));
    if (!q) return list;
    return list.filter((p) => [p.name, p.description, p.port_number].some((v) => (v || '').toLowerCase().includes(q)));
  }, [ports, portFilter]);

  if (loading) {
    return <div className="text-muted-foreground">Loading...</div>;
  }

  if (!sw) {
    return (
      <div className="space-y-4">
        <div className="text-foreground">Switch not found.</div>
        <Link href={`/fabrics/${encodeURIComponent(fabricIdOrName)}/switches`} className="text-primary hover:underline">
          ← Back to Switches
        </Link>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <div className="flex items-center gap-2 text-sm text-muted-foreground mb-2">
            <Link href="/fabrics" className="hover:text-foreground">Fabrics</Link>
            <span>/</span>
            <Link href={`/fabrics/${encodeURIComponent(fabricIdOrName)}`} className="hover:text-foreground">{fabricIdOrName}</Link>
            <span>/</span>
            <Link href={`/fabrics/${encodeURIComponent(fabricIdOrName)}/switches`} className="hover:text-foreground">Switches</Link>
            <span>/</span>
            <span className="text-foreground">{sw.name}</span>
          </div>
          <h1 className="text-2xl font-bold text-foreground">{sw.name}</h1>
          <p className="text-muted-foreground">{fabricIdOrName} • {sw.model || 'Unknown model'}</p>
        </div>
        <Button variant="outline" onClick={loadAll}>Refresh</Button>
      </div>

      {error && (
        <div className="bg-destructive/20 border border-destructive text-destructive px-4 py-3 rounded-md">
          {error}
          <button onClick={() => setError(null)} className="ml-3 underline">
            Dismiss
          </button>
        </div>
      )}

      <Tabs defaultValue="overview">
        <TabsList>
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="ports">Ports</TabsTrigger>
          <TabsTrigger value="mappings">Mappings</TabsTrigger>
        </TabsList>

        <TabsContent value="overview">
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-6 mt-4">
            <Card>
              <CardHeader>
                <CardTitle>General</CardTitle>
              </CardHeader>
              <CardContent className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div>
                  <div className="text-sm text-muted-foreground">Name</div>
                  <div className="text-foreground">{sw.name}</div>
                </div>
                <div>
                  <div className="text-sm text-muted-foreground">IP</div>
                  <div className="text-foreground font-mono">{sw.ip_address || '-'}</div>
                </div>
                <div>
                  <div className="text-sm text-muted-foreground">Model</div>
                  <div className="text-foreground">{sw.model || '-'}</div>
                </div>
                <div>
                  <div className="text-sm text-muted-foreground">Serial</div>
                  <div className="text-foreground font-mono">{sw.serial_number || '-'}</div>
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>Inventory</CardTitle>
              </CardHeader>
              <CardContent className="grid grid-cols-2 gap-4">
                <div>
                  <div className="text-sm text-muted-foreground">Ports</div>
                  <div className="text-2xl font-bold text-foreground">{ports.length}</div>
                </div>
                <div>
                  <div className="text-sm text-muted-foreground">Mappings</div>
                  <div className="text-2xl font-bold text-foreground">{mappings.length}</div>
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>Port status</CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                <div className="flex items-center justify-between">
                  <span className="text-sm text-muted-foreground">Admin up</span>
                  <Badge variant="success">
                    {ports.filter((p) => p.admin_state === 'true').length}
                  </Badge>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-sm text-muted-foreground">Admin down</span>
                  <Badge variant="secondary">
                    {ports.filter((p) => p.admin_state !== 'true').length}
                  </Badge>
                </div>
              </CardContent>
            </Card>
          </div>
        </TabsContent>

        <TabsContent value="ports">
          <Card className="mt-4">
            <CardHeader>
              <CardTitle>Ports</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="flex items-center justify-between gap-4 mb-4">
                <Input
                  placeholder="Filter by name / description..."
                  value={portFilter}
                  onChange={(e) => setPortFilter(e.target.value)}
                  className="max-w-sm"
                />
                <span className="text-sm text-muted-foreground">
                  {filteredPorts.length} port{filteredPorts.length === 1 ? '' : 's'}
                </span>
              </div>

              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Port</TableHead>
                    <TableHead>Description</TableHead>
                    <TableHead>Speed</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Mapping</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {filteredPorts.map((p) => {
                    const mapping = mappingByPortId.get(p.id);
                    return (
                      <TableRow key={p.id}>
                        <TableCell className="font-mono">{p.name}</TableCell>
                        <TableCell>{p.description || '-'}</TableCell>
                        <TableCell>{p.speed || '-'}</TableCell>
                        <TableCell>
                          <Badge variant={p.admin_state === 'true' ? 'success' : 'secondary'}>
                            {p.admin_state === 'true' ? 'Up' : 'Down'}
                          </Badge>
                        </TableCell>
                        <TableCell>
                          {mapping?.compute_node ? (
                            <Link
                              href={`/compute-nodes/${encodeURIComponent(mapping.compute_node.name)}`}
                              className="text-primary hover:underline text-sm"
                            >
                              {mapping.compute_node.name} ({mapping.nic_name})
                            </Link>
                          ) : (
                            <span className="text-sm text-muted-foreground">Unassigned</span>
                          )}
                        </TableCell>
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="mappings">
          <Card className="mt-4">
            <CardHeader>
              <CardTitle>Mappings</CardTitle>
            </CardHeader>
            <CardContent>
              {mappings.length === 0 ? (
                <div className="text-muted-foreground">No mappings for this switch.</div>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Port</TableHead>
                      <TableHead>Compute node</TableHead>
                      <TableHead>NIC</TableHead>
                      <TableHead>Interface</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {mappings.map((m) => (
                      <TableRow key={m.id}>
                        <TableCell className="font-mono">{m.switch_port?.name || '-'}</TableCell>
                        <TableCell>
                          {m.compute_node ? (
                            <Link
                              href={`/compute-nodes/${encodeURIComponent(m.compute_node.name)}`}
                              className="text-primary hover:underline"
                            >
                              {m.compute_node.name}
                            </Link>
                          ) : (
                            <span className="text-muted-foreground">-</span>
                          )}
                        </TableCell>
                        <TableCell className="font-mono">{m.nic_name || '-'}</TableCell>
                        <TableCell>{m.interface_id || '-'}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}
