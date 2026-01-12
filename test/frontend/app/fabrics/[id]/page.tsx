'use client';

import { useCallback, useEffect, useState } from 'react';
import Link from 'next/link';
import { useParams } from 'next/navigation';
import { switchesAPI, portsAPI, Switch, SwitchPort, PortMapping } from '@/lib/api';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';

export default function FabricDetailPage() {
  const params = useParams();
  const fabricId = decodeURIComponent(params.id as string);
  
  const [switches, setSwitches] = useState<Switch[]>([]);
  const [selectedSwitch, setSelectedSwitch] = useState<Switch | null>(null);
  const [ports, setPorts] = useState<SwitchPort[]>([]);
  const [portMappings, setPortMappings] = useState<Map<string, PortMapping>>(new Map());
  const [loading, setLoading] = useState(true);
  const [portsLoading, setPortsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [syncing, setSyncing] = useState(false);
  const [selectedNodeInfo, setSelectedNodeInfo] = useState<PortMapping | null>(null);

  const loadSwitches = useCallback(async () => {
    try {
      setLoading(true);
      const data = await switchesAPI.list(fabricId);
      setSwitches(data);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load switches');
    } finally {
      setLoading(false);
    }
  }, [fabricId]);

  const loadPorts = async (sw: Switch) => {
    try {
      setPortsLoading(true);
      setSelectedSwitch(sw);
      const [portsData, mappingsData] = await Promise.all([
        portsAPI.list(fabricId, sw.name),
        portsAPI.getMappingsBySwitch(sw.name).catch(() => [])
      ]);
      setPorts(portsData);
      
      // Create a map of port_id -> mapping for quick lookup
      const mappingsMap = new Map<string, PortMapping>();
      mappingsData.forEach((m: PortMapping) => {
        mappingsMap.set(m.switch_port_id, m);
      });
      setPortMappings(mappingsMap);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load ports');
    } finally {
      setPortsLoading(false);
    }
  };

  const handleSyncSwitches = async () => {
    try {
      setSyncing(true);
      await switchesAPI.sync(fabricId);
      await loadSwitches();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to sync switches');
    } finally {
      setSyncing(false);
    }
  };

  const handleSyncAllPorts = async () => {
    try {
      setSyncing(true);
      await portsAPI.syncAll(fabricId);
      if (selectedSwitch) {
        await loadPorts(selectedSwitch);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to sync ports');
    } finally {
      setSyncing(false);
    }
  };

  useEffect(() => {
    loadSwitches();
  }, [loadSwitches]);

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <Link href="/fabrics" className="hover:text-foreground">Fabrics</Link>
        <span>/</span>
        <span className="text-foreground">{fabricId}</span>
      </div>

      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-foreground">{fabricId}</h1>
        <div className="flex gap-2">
          <Button variant="outline" asChild>
            <Link href={`/fabrics/${encodeURIComponent(fabricId)}/switches`}>
              View Switch Inventory
            </Link>
          </Button>
          <Button onClick={handleSyncSwitches} disabled={syncing}>
            {syncing ? 'Syncing...' : 'Sync Switches'}
          </Button>
          <Button variant="secondary" onClick={handleSyncAllPorts} disabled={syncing}>
            Sync All Ports
          </Button>
        </div>
      </div>

      {error && (
        <div className="p-4 bg-destructive/20 text-destructive rounded-lg">
          {error}
        </div>
      )}

      {/* Node Info Modal */}
      {selectedNodeInfo && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <Card className="w-full max-w-md">
            <CardHeader>
              <CardTitle>Compute Node Details</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              <div>
                <label className="block text-sm font-medium text-muted-foreground">Name</label>
                <div className="text-foreground">{selectedNodeInfo.compute_node?.name || 'Unknown'}</div>
              </div>
              <div>
                <label className="block text-sm font-medium text-muted-foreground">Hostname</label>
                <div className="text-foreground">{selectedNodeInfo.compute_node?.hostname || 'N/A'}</div>
              </div>
              <div>
                <label className="block text-sm font-medium text-muted-foreground">IP Address</label>
                <div className="text-foreground">{selectedNodeInfo.compute_node?.ip_address || 'N/A'}</div>
              </div>
              <div>
                <label className="block text-sm font-medium text-muted-foreground">NIC Name</label>
                <div className="text-foreground">{selectedNodeInfo.nic_name}</div>
              </div>
              <div>
                <label className="block text-sm font-medium text-muted-foreground">Port</label>
                <div className="text-foreground">{selectedNodeInfo.switch_port?.name || 'Unknown'}</div>
              </div>
              <div className="flex gap-2 justify-end pt-4">
                <Button variant="ghost" asChild>
                  <Link href="/compute-nodes">Go to Compute Nodes</Link>
                </Button>
                <Button variant="outline" onClick={() => setSelectedNodeInfo(null)}>
                  Close
                </Button>
              </div>
            </CardContent>
          </Card>
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Switches List */}
        <Card>
          <CardHeader>
            <CardTitle>Switches</CardTitle>
          </CardHeader>
          <CardContent>
            {loading ? (
              <div className="text-muted-foreground">Loading...</div>
            ) : switches.length === 0 ? (
              <div className="text-muted-foreground">No switches found</div>
            ) : (
              <div className="space-y-2">
                {switches.map((sw) => (
                  <button
                    key={sw.id}
                    onClick={() => loadPorts(sw)}
                    className={`w-full text-left p-4 rounded-lg border transition-colors ${
                      selectedSwitch?.id === sw.id
                        ? 'bg-accent border-primary'
                        : 'bg-card border-border hover:border-primary'
                    }`}
                  >
                    <div className="flex items-center justify-between gap-3">
                      <div className="font-medium text-foreground">{sw.name}</div>
                      <Button variant="ghost" size="sm" asChild onClick={(e) => e.stopPropagation()}>
                        <Link href={`/fabrics/${encodeURIComponent(fabricId)}/switches/${encodeURIComponent(sw.id)}`}>
                          Details
                        </Link>
                      </Button>
                    </div>
                    <div className="text-sm text-muted-foreground">
                      {sw.model} • {sw.ip_address}
                    </div>
                    <div className="text-xs text-muted-foreground mt-1">
                      Serial: {sw.serial_number}
                    </div>
                  </button>
                ))}
              </div>
            )}
          </CardContent>
        </Card>

        {/* Ports List */}
        <Card>
          <CardHeader>
            <CardTitle>Ports {selectedSwitch && `(${selectedSwitch.name})`}</CardTitle>
          </CardHeader>
          <CardContent>
            {!selectedSwitch ? (
              <div className="text-muted-foreground">Select a switch to view ports</div>
            ) : portsLoading ? (
              <div className="text-muted-foreground">Loading ports...</div>
            ) : ports.length === 0 ? (
              <div className="text-muted-foreground">No ports found</div>
            ) : (
              <div className="max-h-[600px] overflow-y-auto space-y-1">
                {[...ports].sort((a, b) => {
                  const extractNums = (name: string) => {
                    const matches = name.match(/(\d+)/g);
                    return matches ? matches.map(Number) : [0];
                  };
                  const numsA = extractNums(a.name);
                  const numsB = extractNums(b.name);
                  for (let i = 0; i < Math.max(numsA.length, numsB.length); i++) {
                    const diff = (numsA[i] || 0) - (numsB[i] || 0);
                    if (diff !== 0) return diff;
                  }
                  return a.name.localeCompare(b.name);
                }).map((port) => {
                  const mapping = portMappings.get(port.id);
                  return (
                    <div
                      key={port.id}
                      className={`p-3 rounded border ${
                        mapping
                          ? 'bg-accent border-primary'
                          : 'bg-card border-border'
                      }`}
                    >
                      <div className="flex items-center justify-between">
                        <span className="font-mono text-sm text-foreground">{port.name}</span>
                        <Badge variant={port.admin_state === 'true' ? 'default' : 'secondary'}>
                          {port.admin_state === 'true' ? 'Up' : 'Down'}
                        </Badge>
                      </div>
                      <div className="text-xs text-muted-foreground mt-1">Speed: {port.speed}</div>
                      {mapping && (
                        <button
                          onClick={() => setSelectedNodeInfo(mapping)}
                          className="mt-2 text-xs text-primary hover:underline flex items-center gap-1"
                        >
                          <span className="inline-block w-2 h-2 bg-primary rounded-full"></span>
                          Mapped: {mapping.compute_node?.name || 'Unknown'} ({mapping.nic_name})
                        </button>
                      )}
                    </div>
                  );
                })}
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
