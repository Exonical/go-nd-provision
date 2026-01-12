'use client';

import Link from 'next/link';
import { useParams } from 'next/navigation';
import { useEffect, useState } from 'react';
import {
  computeNodesAPI,
  ComputeNode,
  ComputeNodeInterface,
  PortMapping,
  fabricsAPI,
  switchesAPI,
  portsAPI,
  Fabric,
  Switch,
  SwitchPort,
} from '@/lib/api';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle, CardAction } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';

export default function ComputeNodeDetailPage() {
  const params = useParams();
  const nodeIdOrName = decodeURIComponent(params.id as string);

  const [node, setNode] = useState<ComputeNode | null>(null);
  const [interfaces, setInterfaces] = useState<ComputeNodeInterface[]>([]);
  const [mappings, setMappings] = useState<PortMapping[]>([]);

  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [creatingInterface, setCreatingInterface] = useState(false);
  const [creatingMapping, setCreatingMapping] = useState(false);

  // Dropdown data for Add Mapping form
  const [fabrics, setFabrics] = useState<Fabric[]>([]);
  const [switches, setSwitches] = useState<Switch[]>([]);
  const [ports, setPorts] = useState<SwitchPort[]>([]);
  const [portMappingsForSwitch, setPortMappingsForSwitch] = useState<PortMapping[]>([]);

  const [selectedFabricId, setSelectedFabricId] = useState('');
  const [selectedSwitchId, setSelectedSwitchId] = useState('');
  const [selectedPortId, setSelectedPortId] = useState('');
  const [newMappingNICName, setNewMappingNICName] = useState('');

  // Alert dialog state
  const [deleteInterfaceId, setDeleteInterfaceId] = useState<string | null>(null);
  const [deleteMappingId, setDeleteMappingId] = useState<string | null>(null);
  const [portOverwriteInfo, setPortOverwriteInfo] = useState<{ portId: string; nodeName: string } | null>(null);

  const loadAll = async () => {
    try {
      setLoading(true);
      const [nodeData, ifaceData, mappingData] = await Promise.all([
        computeNodesAPI.get(nodeIdOrName),
        computeNodesAPI.getInterfaces(nodeIdOrName),
        computeNodesAPI.getPortMappings(nodeIdOrName),
      ]);
      setNode(nodeData);
      setInterfaces(ifaceData);
      setMappings(mappingData);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load compute node');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadAll();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [nodeIdOrName]);

  const handleCreateInterface = async (role: 'compute' | 'storage') => {
    try {
      setCreatingInterface(true);
      await computeNodesAPI.createInterface(nodeIdOrName, { role });
      const ifaceData = await computeNodesAPI.getInterfaces(nodeIdOrName);
      setInterfaces(ifaceData);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create interface');
    } finally {
      setCreatingInterface(false);
    }
  };

  const handleDeleteInterface = async (ifaceId: string) => {
    try {
      await computeNodesAPI.deleteInterface(nodeIdOrName, ifaceId);
      await loadAll();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete interface');
    } finally {
      setDeleteInterfaceId(null);
    }
  };

  const handleAssignMapping = async (mappingId: string, interfaceId: string | null) => {
    try {
      await computeNodesAPI.assignMappingToInterface(nodeIdOrName, mappingId, interfaceId);
      const mappingData = await computeNodesAPI.getPortMappings(nodeIdOrName);
      setMappings(mappingData);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to assign mapping');
    }
  };

  // Load fabrics on mount
  useEffect(() => {
    fabricsAPI.list().then(setFabrics).catch(console.error);
  }, []);

  // Load switches when fabric changes
  const handleFabricChange = async (fabricId: string) => {
    setSelectedFabricId(fabricId);
    setSelectedSwitchId('');
    setSelectedPortId('');
    setSwitches([]);
    setPorts([]);
    setPortMappingsForSwitch([]);
    if (fabricId) {
      try {
        const data = await switchesAPI.list(fabricId);
        setSwitches(data);
      } catch (err) {
        console.error('Failed to load switches', err);
      }
    }
  };

  // Load ports and their mappings when switch changes
  const handleSwitchChange = async (switchId: string) => {
    setSelectedSwitchId(switchId);
    setSelectedPortId('');
    setPorts([]);
    setPortMappingsForSwitch([]);
    if (switchId && selectedFabricId) {
      try {
        const sw = switches.find((s) => s.id === switchId);
        if (sw) {
          const [portsData, mappingsData] = await Promise.all([
            portsAPI.list(selectedFabricId, sw.name),
            portsAPI.getMappingsBySwitch(sw.name).catch(() => []),
          ]);
          setPorts(portsData);
          setPortMappingsForSwitch(mappingsData);
        }
      } catch (err) {
        console.error('Failed to load ports', err);
      }
    }
  };

  // Check if a port is in use
  const getPortMapping = (portId: string) => {
    return portMappingsForSwitch.find((m) => m.switch_port_id === portId);
  };

  // Handle port selection with confirmation if in use
  const handlePortChange = (portId: string) => {
    const existingMapping = getPortMapping(portId);
    if (existingMapping && existingMapping.compute_node_id !== node?.id) {
      const nodeName = existingMapping.compute_node?.name || 'another node';
      setPortOverwriteInfo({ portId, nodeName });
      return;
    }
    setSelectedPortId(portId);
  };

  const confirmPortOverwrite = () => {
    if (portOverwriteInfo) {
      setSelectedPortId(portOverwriteInfo.portId);
      setPortOverwriteInfo(null);
    }
  };

  const handleCreateMapping = async () => {
    const selectedSwitch = switches.find((s) => s.id === selectedSwitchId);
    const selectedPort = ports.find((p) => p.id === selectedPortId);

    if (!selectedSwitch || !selectedPort || !newMappingNICName) {
      setError('Switch, Port, and NIC name are required');
      return;
    }

    try {
      setCreatingMapping(true);
      await computeNodesAPI.addPortMapping(nodeIdOrName, {
        switch: selectedSwitch.name,
        port_name: selectedPort.name,
        nic_name: newMappingNICName,
      });
      // Reset form
      setSelectedFabricId('');
      setSelectedSwitchId('');
      setSelectedPortId('');
      setNewMappingNICName('');
      setSwitches([]);
      setPorts([]);
      setPortMappingsForSwitch([]);
      // Reload mappings
      const mappingData = await computeNodesAPI.getPortMappings(nodeIdOrName);
      setMappings(mappingData);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create mapping');
    } finally {
      setCreatingMapping(false);
    }
  };

  const handleDeleteMapping = async (mappingId: string) => {
    try {
      await computeNodesAPI.deletePortMapping(nodeIdOrName, mappingId);
      const mappingData = await computeNodesAPI.getPortMappings(nodeIdOrName);
      setMappings(mappingData);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete mapping');
    } finally {
      setDeleteMappingId(null);
    }
  };

  if (loading) {
    return <div className="text-muted-foreground">Loading...</div>;
  }

  if (!node) {
    return (
      <div className="space-y-4">
        <div className="text-foreground">Compute node not found.</div>
        <Link href="/compute-nodes" className="text-primary hover:underline">
          ← Back to Compute Nodes
        </Link>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <div className="flex items-center gap-2 text-sm text-muted-foreground mb-2">
            <Link href="/compute-nodes" className="hover:text-foreground">Compute Nodes</Link>
            <span>/</span>
            <span className="text-foreground">{node.name}</span>
          </div>
          <h1 className="text-2xl font-bold text-foreground">{node.name}</h1>
          <p className="text-muted-foreground">{node.hostname || node.ip_address || 'Compute node'}</p>
        </div>
        <Button variant="outline" asChild>
          <Link href="/switch-ports">Bulk assign ports</Link>
        </Button>
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
          <TabsTrigger value="interfaces">Interfaces</TabsTrigger>
          <TabsTrigger value="mappings">Port mappings</TabsTrigger>
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
                  <div className="text-foreground">{node.name}</div>
                </div>
                <div>
                  <div className="text-sm text-muted-foreground">Hostname</div>
                  <div className="text-foreground">{node.hostname || '-'}</div>
                </div>
                <div>
                  <div className="text-sm text-muted-foreground">IP</div>
                  <div className="text-foreground font-mono">{node.ip_address || '-'}</div>
                </div>
                <div>
                  <div className="text-sm text-muted-foreground">MAC</div>
                  <div className="text-foreground font-mono">{node.mac_address || '-'}</div>
                </div>
                <div className="sm:col-span-2">
                  <div className="text-sm text-muted-foreground">Description</div>
                  <div className="text-foreground">{node.description || '-'}</div>
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>Inventory</CardTitle>
              </CardHeader>
              <CardContent className="grid grid-cols-2 gap-4">
                <div>
                  <div className="text-sm text-muted-foreground">Interfaces</div>
                  <div className="text-2xl font-bold text-foreground">{interfaces.length}</div>
                </div>
                <div>
                  <div className="text-sm text-muted-foreground">Port mappings</div>
                  <div className="text-2xl font-bold text-foreground">{mappings.length}</div>
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>Quick status</CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                <div className="flex items-center justify-between">
                  <span className="text-sm text-muted-foreground">Mapped</span>
                  <Badge variant={mappings.some((m) => m.switch_port_id) ? 'success' : 'secondary'}>
                    {mappings.some((m) => m.switch_port_id) ? 'Yes' : 'No'}
                  </Badge>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-sm text-muted-foreground">Has interfaces</span>
                  <Badge variant={interfaces.length > 0 ? 'success' : 'outline'}>
                    {interfaces.length > 0 ? 'Yes' : 'No'}
                  </Badge>
                </div>
              </CardContent>
            </Card>
          </div>
        </TabsContent>

        <TabsContent value="interfaces">
          <Card className="mt-4">
            <CardHeader className="flex-row items-center justify-between">
              <CardTitle>Interfaces</CardTitle>
              <CardAction>
                <div className="flex gap-2">
                  <Button size="sm" onClick={() => handleCreateInterface('compute')} disabled={creatingInterface}>
                    + Compute
                  </Button>
                  <Button size="sm" variant="secondary" onClick={() => handleCreateInterface('storage')} disabled={creatingInterface}>
                    + Storage
                  </Button>
                </div>
              </CardAction>
            </CardHeader>
            <CardContent>
              {interfaces.length === 0 ? (
                <div className="text-muted-foreground">No interfaces.</div>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Role</TableHead>
                      <TableHead>Hostname</TableHead>
                      <TableHead>IP</TableHead>
                      <TableHead>MAC</TableHead>
                      <TableHead className="text-right">Actions</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {interfaces.map((iface) => (
                      <TableRow key={iface.id}>
                        <TableCell>
                          <Badge variant={iface.role === 'compute' ? 'compute' : 'storage'}>
                            {iface.role}
                          </Badge>
                        </TableCell>
                        <TableCell>{iface.hostname || '-'}</TableCell>
                        <TableCell className="font-mono">{iface.ip_address || '-'}</TableCell>
                        <TableCell className="font-mono">{iface.mac_address || '-'}</TableCell>
                        <TableCell className="text-right">
                          <Button variant="ghost" size="sm" onClick={() => setDeleteInterfaceId(iface.id)} className="text-destructive hover:text-destructive">
                            Delete
                          </Button>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="mappings">
          <div className="space-y-6 mt-4">
            <Card>
              <CardHeader>
                <CardTitle>Add mapping</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
                  <div>
                    <label className="text-sm text-muted-foreground block mb-1">Fabric</label>
                    <select
                      value={selectedFabricId}
                      onChange={(e) => handleFabricChange(e.target.value)}
                      className="w-full border border-input bg-background text-foreground rounded-md px-3 py-2"
                    >
                      <option value="">Select fabric...</option>
                      {fabrics.map((f) => (
                        <option key={f.id} value={f.id}>{f.name}</option>
                      ))}
                    </select>
                  </div>
                  <div>
                    <label className="text-sm text-muted-foreground block mb-1">Switch</label>
                    <select
                      value={selectedSwitchId}
                      onChange={(e) => handleSwitchChange(e.target.value)}
                      disabled={!selectedFabricId}
                      className="w-full border border-input bg-background text-foreground rounded-md px-3 py-2 disabled:opacity-50"
                    >
                      <option value="">Select switch...</option>
                      {switches.map((sw) => (
                        <option key={sw.id} value={sw.id}>{sw.name} ({sw.ip_address})</option>
                      ))}
                    </select>
                  </div>
                  <div>
                    <label className="text-sm text-muted-foreground block mb-1">Port</label>
                    <select
                      value={selectedPortId}
                      onChange={(e) => handlePortChange(e.target.value)}
                      disabled={!selectedSwitchId}
                      className="w-full border border-input bg-background text-foreground rounded-md px-3 py-2 disabled:opacity-50"
                    >
                      <option value="">Select port...</option>
                      {ports.map((p) => {
                        const mapping = getPortMapping(p.id);
                        const inUse = mapping && mapping.compute_node_id !== node?.id;
                        return (
                          <option key={p.id} value={p.id}>
                            {p.name}{inUse ? ` (in use: ${mapping?.compute_node?.name || 'unknown'})` : ''}
                          </option>
                        );
                      })}
                    </select>
                  </div>
                  <div>
                    <label className="text-sm text-muted-foreground block mb-1">NIC name</label>
                    <Input value={newMappingNICName} onChange={(e) => setNewMappingNICName(e.target.value)} placeholder="eth0" />
                  </div>
                </div>
                <div className="mt-4 flex justify-end">
                  <Button onClick={handleCreateMapping} disabled={creatingMapping || !selectedPortId}>
                    {creatingMapping ? 'Adding...' : 'Add mapping'}
                  </Button>
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>Port mappings</CardTitle>
              </CardHeader>
              <CardContent>
                {mappings.length === 0 ? (
                  <div className="text-muted-foreground">No port mappings.</div>
                ) : (
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Switch</TableHead>
                        <TableHead>Port</TableHead>
                        <TableHead>NIC</TableHead>
                        <TableHead>Interface</TableHead>
                        <TableHead className="text-right">Actions</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {mappings.map((m) => (
                        <TableRow key={m.id}>
                          <TableCell>{m.switch_port?.switch?.name || '-'}</TableCell>
                          <TableCell className="font-mono">{m.switch_port?.name || '-'}</TableCell>
                          <TableCell className="font-mono">{m.nic_name || '-'}</TableCell>
                          <TableCell>
                            <select
                              value={m.interface_id || ''}
                              onChange={(e) => handleAssignMapping(m.id, e.target.value || null)}
                              className="border border-input bg-background text-foreground rounded px-2 py-1 text-sm"
                            >
                              <option value="">-- None --</option>
                              {interfaces.map((iface) => (
                                <option key={iface.id} value={iface.id}>
                                  {iface.role}
                                </option>
                              ))}
                            </select>
                          </TableCell>
                          <TableCell className="text-right">
                            <Button variant="ghost" size="sm" onClick={() => setDeleteMappingId(m.id)} className="text-destructive hover:text-destructive">
                              Delete
                            </Button>
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                )}
              </CardContent>
            </Card>
          </div>
        </TabsContent>
      </Tabs>

      {/* Delete Interface Alert Dialog */}
      <AlertDialog open={!!deleteInterfaceId} onOpenChange={(open) => !open && setDeleteInterfaceId(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete Interface</AlertDialogTitle>
            <AlertDialogDescription>
              Are you sure you want to delete this interface? Port mappings assigned to this interface will be unassigned.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={() => deleteInterfaceId && handleDeleteInterface(deleteInterfaceId)}>
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Delete Mapping Alert Dialog */}
      <AlertDialog open={!!deleteMappingId} onOpenChange={(open) => !open && setDeleteMappingId(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete Port Mapping</AlertDialogTitle>
            <AlertDialogDescription>
              Are you sure you want to delete this port mapping?
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={() => deleteMappingId && handleDeleteMapping(deleteMappingId)}>
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Port Overwrite Confirmation Alert Dialog */}
      <AlertDialog open={!!portOverwriteInfo} onOpenChange={(open) => !open && setPortOverwriteInfo(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Port Already Mapped</AlertDialogTitle>
            <AlertDialogDescription>
              This port is already mapped to <strong>{portOverwriteInfo?.nodeName}</strong>. Do you want to overwrite the existing mapping?
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={confirmPortOverwrite}>
              Overwrite
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
