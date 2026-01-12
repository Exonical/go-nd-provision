'use client';

import { useEffect, useState, useCallback } from 'react';
import Link from 'next/link';
import { computeNodesAPI, ComputeNode, PortMapping, ComputeNodeInterface, fabricsAPI, switchesAPI, portsAPI, Fabric, Switch, SwitchPort } from '@/lib/api';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';

export default function ComputeNodesPage() {
  const [nodes, setNodes] = useState<ComputeNode[]>([]);
  const [selectedNode, setSelectedNode] = useState<ComputeNode | null>(null);
  const [mappings, setMappings] = useState<PortMapping[]>([]);
  const [loading, setLoading] = useState(true);
  const [mappingsLoading, setMappingsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [nodeSearch, setNodeSearch] = useState('');
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [showMapForm, setShowMapForm] = useState(false);
  const [editingMapping, setEditingMapping] = useState<PortMapping | null>(null);
  const [editingNode, setEditingNode] = useState<ComputeNode | null>(null);

  // Create node form state
  const [newNode, setNewNode] = useState({ name: '', hostname: '', ip_address: '' });

  // Edit node form state
  const [editNodeName, setEditNodeName] = useState('');
  const [editNodeHostname, setEditNodeHostname] = useState('');
  const [editNodeIP, setEditNodeIP] = useState('');
  const [editNodeMAC, setEditNodeMAC] = useState('');
  const [editNodeDescription, setEditNodeDescription] = useState('');

  // Map port form state
  const [newMapping, setNewMapping] = useState({ switch: '', port_name: '', nic_name: '' });

  // Edit mapping form state
  const [editNicName, setEditNicName] = useState('');
  const [editSwitch, setEditSwitch] = useState('');
  const [editPortName, setEditPortName] = useState('');

  // Dropdown data for switch/port selection
  const [fabrics, setFabrics] = useState<Fabric[]>([]);
  const [switches, setSwitches] = useState<Switch[]>([]);
  const [ports, setPorts] = useState<SwitchPort[]>([]);
  const [selectedFabric, setSelectedFabric] = useState('');
  const [portFilter, setPortFilter] = useState('');
  const [portsLoading, setPortsLoading] = useState(false);

  // Interface management state
  const [interfaces, setInterfaces] = useState<ComputeNodeInterface[]>([]);
  const [interfacesLoading, setInterfacesLoading] = useState(false);
  const [showInterfaceForm, setShowInterfaceForm] = useState(false);
  const [newInterfaceRole, setNewInterfaceRole] = useState<'compute' | 'storage'>('compute');

  const loadNodes = useCallback(async () => {
    try {
      setLoading(true);
      const data = await computeNodesAPI.list();
      setNodes(data);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load nodes');
    } finally {
      setLoading(false);
    }
  }, []);

  const loadMappings = async (node: ComputeNode) => {
    try {
      setMappingsLoading(true);
      setSelectedNode(node);
      const [mappingsData, interfacesData] = await Promise.all([
        computeNodesAPI.getPortMappings(node.name),
        computeNodesAPI.getInterfaces(node.name),
      ]);
      setMappings(mappingsData);
      setInterfaces(interfacesData);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load mappings');
    } finally {
      setMappingsLoading(false);
    }
  };

  const loadInterfaces = async (node: ComputeNode) => {
    try {
      setInterfacesLoading(true);
      const data = await computeNodesAPI.getInterfaces(node.name);
      setInterfaces(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load interfaces');
    } finally {
      setInterfacesLoading(false);
    }
  };

  const handleCreateInterface = async (role: 'compute' | 'storage') => {
    if (!selectedNode) return;
    try {
      await computeNodesAPI.createInterface(selectedNode.name, { role });
      await loadInterfaces(selectedNode);
      setShowInterfaceForm(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create interface');
    }
  };

  const handleDeleteInterface = async (ifaceId: string) => {
    if (!selectedNode) return;
    if (!confirm('Delete this interface? Port mappings will be unassigned.')) return;
    try {
      await computeNodesAPI.deleteInterface(selectedNode.name, ifaceId);
      await loadInterfaces(selectedNode);
      await loadMappings(selectedNode);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete interface');
    }
  };

  const handleAssignMapping = async (mappingId: string, interfaceId: string | null) => {
    if (!selectedNode) return;
    try {
      await computeNodesAPI.assignMappingToInterface(selectedNode.name, mappingId, interfaceId);
      await loadMappings(selectedNode);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to assign mapping');
    }
  };

  const handleCreateNode = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await computeNodesAPI.create(newNode);
      setNewNode({ name: '', hostname: '', ip_address: '' });
      setShowCreateForm(false);
      await loadNodes();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create node');
    }
  };

  const handleDeleteNode = async (node: ComputeNode) => {
    if (!confirm(`Delete node "${node.name}"?`)) return;
    try {
      await computeNodesAPI.delete(node.name);
      if (selectedNode?.id === node.id) {
        setSelectedNode(null);
        setMappings([]);
      }
      await loadNodes();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete node');
    }
  };

  const handleEditNode = (node: ComputeNode) => {
    setEditingNode(node);
    setEditNodeName(node.name);
    setEditNodeHostname(node.hostname || '');
    setEditNodeIP(node.ip_address || '');
    setEditNodeMAC(node.mac_address || '');
    setEditNodeDescription(node.description || '');
  };

  const handleUpdateNode = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!editingNode) return;
    try {
      const updated = await computeNodesAPI.update(editingNode.name, {
        name: editNodeName,
        hostname: editNodeHostname,
        ip_address: editNodeIP,
        mac_address: editNodeMAC,
        description: editNodeDescription
      });
      setEditingNode(null);
      // Update selected node if it was the one edited
      if (selectedNode?.id === editingNode.id) {
        setSelectedNode(updated);
      }
      await loadNodes();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update node');
    }
  };

  const handleAddMapping = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedNode) return;
    try {
      await computeNodesAPI.addPortMapping(selectedNode.name, newMapping);
      setNewMapping({ switch: '', port_name: '', nic_name: '' });
      setShowMapForm(false);
      setSelectedFabric('');
      setSwitches([]);
      setPorts([]);
      setPortFilter('');
      await loadMappings(selectedNode);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to add mapping');
    }
  };

  const handleFabricChangeForAdd = async (fabricName: string) => {
    setSelectedFabric(fabricName);
    setNewMapping({ ...newMapping, switch: '', port_name: '' });
    setPorts([]);
    setPortFilter('');
    if (fabricName) {
      try {
        const switchesData = await switchesAPI.list(fabricName);
        setSwitches(switchesData);
      } catch (err) {
        console.error('Failed to load switches', err);
      }
    } else {
      setSwitches([]);
    }
  };

  const handleSwitchChangeForAdd = async (switchName: string) => {
    setNewMapping({ ...newMapping, switch: switchName, port_name: '' });
    setPortFilter('');
    if (switchName && selectedFabric) {
      try {
        setPortsLoading(true);
        const portsData = await portsAPI.list(selectedFabric, switchName);
        setPorts(portsData);
      } catch (err) {
        console.error('Failed to load ports', err);
      } finally {
        setPortsLoading(false);
      }
    } else {
      setPorts([]);
    }
  };

  const handleOpenAddMapping = async () => {
    setShowMapForm(true);
    setNewMapping({ switch: '', port_name: '', nic_name: '' });
    setSelectedFabric('');
    setSwitches([]);
    setPorts([]);
    setPortFilter('');
    try {
      const fabricsData = await fabricsAPI.list();
      setFabrics(fabricsData);
    } catch (err) {
      console.error('Failed to load fabrics', err);
    }
  };

  const handleDeleteMapping = async (mapping: PortMapping) => {
    if (!selectedNode) return;
    try {
      await computeNodesAPI.deletePortMapping(selectedNode.name, mapping.id);
      await loadMappings(selectedNode);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete mapping');
    }
  };

  const handleEditMapping = async (mapping: PortMapping) => {
    setEditingMapping(mapping);
    setEditNicName(mapping.nic_name);
    setEditSwitch(mapping.switch_port?.switch?.name || '');
    setEditPortName(mapping.switch_port?.name || '');
    setPortFilter('');
    
    // Load fabrics for dropdown
    try {
      const fabricsData = await fabricsAPI.list();
      setFabrics(fabricsData);
      
      // If we have a switch, load its fabric's switches and ports
      if (mapping.switch_port?.switch?.fabric_id) {
        const fabricName = fabricsData.find(f => f.id === mapping.switch_port?.switch?.fabric_id)?.name || '';
        setSelectedFabric(fabricName);
        if (fabricName) {
          const switchesData = await switchesAPI.list(fabricName);
          setSwitches(switchesData);
          if (mapping.switch_port?.switch?.name) {
            const portsData = await portsAPI.list(fabricName, mapping.switch_port.switch.name);
            setPorts(portsData);
          }
        }
      }
    } catch (err) {
      console.error('Failed to load dropdown data', err);
    }
  };

  const handleFabricChange = async (fabricName: string) => {
    setSelectedFabric(fabricName);
    setEditSwitch('');
    setEditPortName('');
    setPorts([]);
    if (fabricName) {
      try {
        const switchesData = await switchesAPI.list(fabricName);
        setSwitches(switchesData);
      } catch (err) {
        console.error('Failed to load switches', err);
      }
    } else {
      setSwitches([]);
    }
  };

  const handleSwitchChange = async (switchName: string) => {
    setEditSwitch(switchName);
    setEditPortName('');
    setPortFilter('');
    if (switchName && selectedFabric) {
      try {
        setPortsLoading(true);
        const portsData = await portsAPI.list(selectedFabric, switchName);
        setPorts(portsData);
      } catch (err) {
        console.error('Failed to load ports', err);
      } finally {
        setPortsLoading(false);
      }
    } else {
      setPorts([]);
    }
  };

  const filteredPorts = ports
    .filter(p => p.name.toLowerCase().includes(portFilter.toLowerCase()))
    .sort((a, b) => {
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
    });

  const handleUpdateMapping = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedNode || !editingMapping) return;
    try {
      await computeNodesAPI.updatePortMapping(selectedNode.name, editingMapping.id, {
        nic_name: editNicName,
        switch: editSwitch,
        port_name: editPortName
      });
      setEditingMapping(null);
      setEditNicName('');
      setEditSwitch('');
      setEditPortName('');
      await loadMappings(selectedNode);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update mapping');
    }
  };

  useEffect(() => {
    loadNodes();
  }, [loadNodes]);

  const visibleNodes = nodes.filter((n) => {
    const q = nodeSearch.trim().toLowerCase();
    if (!q) return true;
    return [n.name, n.hostname ?? '', n.ip_address ?? ''].some((v) => v.toLowerCase().includes(q));
  });

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-foreground">Compute Nodes</h1>
          <p className="text-muted-foreground">Manage compute nodes and port mappings</p>
        </div>
        <Button onClick={() => setShowCreateForm(true)}>+ Create Node</Button>
      </div>

      {error && (
        <div className="bg-destructive/20 border border-destructive text-destructive px-4 py-3 rounded-md">
          {error}
          <button onClick={() => setError(null)} className="ml-4 underline">Dismiss</button>
        </div>
      )}

      {/* Edit Node Modal */}
      {editingNode && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <Card className="w-full max-w-md">
            <CardHeader>
              <CardTitle>Edit Compute Node</CardTitle>
            </CardHeader>
            <CardContent>
              <form onSubmit={handleUpdateNode} className="space-y-4">
                <div>
                  <label className="block text-sm font-medium text-muted-foreground mb-1">Name *</label>
                  <Input
                    type="text"
                    value={editNodeName}
                    onChange={(e) => setEditNodeName(e.target.value)}
                    required
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-muted-foreground mb-1">Hostname</label>
                  <Input
                    type="text"
                    value={editNodeHostname}
                    onChange={(e) => setEditNodeHostname(e.target.value)}
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-muted-foreground mb-1">IP Address</label>
                  <Input
                    type="text"
                    value={editNodeIP}
                    onChange={(e) => setEditNodeIP(e.target.value)}
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-muted-foreground mb-1">MAC Address</label>
                  <Input
                    type="text"
                    value={editNodeMAC}
                    onChange={(e) => setEditNodeMAC(e.target.value)}
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-muted-foreground mb-1">Description</label>
                  <textarea
                    value={editNodeDescription}
                    onChange={(e) => setEditNodeDescription(e.target.value)}
                    rows={3}
                    className="w-full px-3 py-2 border border-input rounded-md bg-background text-foreground"
                  />
                </div>
                <div className="flex gap-2 justify-end pt-4">
                  <Button type="button" variant="outline" onClick={() => setEditingNode(null)}>
                    Cancel
                  </Button>
                  <Button type="submit">Save</Button>
                </div>
              </form>
            </CardContent>
          </Card>
        </div>
      )}

      {/* Create Node Modal */}
      {showCreateForm && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <Card className="w-full max-w-md">
            <CardHeader>
              <CardTitle>Create Compute Node</CardTitle>
            </CardHeader>
            <CardContent>
              <form onSubmit={handleCreateNode} className="space-y-4">
                <div>
                  <label className="block text-sm font-medium text-muted-foreground mb-1">Name *</label>
                  <Input
                    type="text"
                    value={newNode.name}
                    onChange={(e) => setNewNode({ ...newNode, name: e.target.value })}
                    required
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-muted-foreground mb-1">Hostname</label>
                  <Input
                    type="text"
                    value={newNode.hostname}
                    onChange={(e) => setNewNode({ ...newNode, hostname: e.target.value })}
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-muted-foreground mb-1">IP Address</label>
                  <Input
                    type="text"
                    value={newNode.ip_address}
                    onChange={(e) => setNewNode({ ...newNode, ip_address: e.target.value })}
                  />
                </div>
                <div className="flex gap-2 justify-end pt-4">
                  <Button type="button" variant="outline" onClick={() => setShowCreateForm(false)}>
                    Cancel
                  </Button>
                  <Button type="submit">Create</Button>
                </div>
              </form>
            </CardContent>
          </Card>
        </div>
      )}

      {/* Edit Mapping Modal */}
      {editingMapping && selectedNode && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <Card className="w-full max-w-md">
            <CardHeader>
              <CardTitle>Edit Port Mapping</CardTitle>
            </CardHeader>
            <CardContent>
              <form onSubmit={handleUpdateMapping} className="space-y-4">
                <div>
                  <label className="block text-sm font-medium text-muted-foreground mb-1">Fabric *</label>
                  <select
                    value={selectedFabric}
                    onChange={(e) => handleFabricChange(e.target.value)}
                    required
                    className="w-full px-3 py-2 border border-input rounded-md bg-background text-foreground"
                  >
                    <option value="">Select a fabric...</option>
                    {fabrics.map((f) => (
                      <option key={f.id} value={f.name}>{f.name}</option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="block text-sm font-medium text-muted-foreground mb-1">Switch *</label>
                  <select
                    value={editSwitch}
                    onChange={(e) => handleSwitchChange(e.target.value)}
                    required
                    disabled={!selectedFabric}
                    className="w-full px-3 py-2 border border-input rounded-md bg-background text-foreground disabled:opacity-50"
                  >
                    <option value="">Select a switch...</option>
                    {switches.map((sw) => (
                      <option key={sw.id} value={sw.name}>{sw.name} ({sw.ip_address})</option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="block text-sm font-medium text-muted-foreground mb-1">Port *</label>
                  <Input
                    type="text"
                    value={portFilter}
                    onChange={(e) => setPortFilter(e.target.value)}
                    placeholder="Filter ports..."
                    className="mb-2"
                  />
                  <select
                    value={editPortName}
                    onChange={(e) => setEditPortName(e.target.value)}
                    required
                    disabled={!editSwitch || portsLoading}
                    className="w-full px-3 py-2 border border-input rounded-md bg-background text-foreground disabled:opacity-50"
                    size={6}
                  >
                    {portsLoading ? (
                      <option value="">Loading ports...</option>
                    ) : filteredPorts.length === 0 ? (
                      <option value="">No ports found</option>
                    ) : (
                      filteredPorts.map((p) => (
                        <option key={p.id} value={p.name}>{p.name}</option>
                      ))
                    )}
                  </select>
                </div>
                <div>
                  <label className="block text-sm font-medium text-muted-foreground mb-1">NIC Name *</label>
                  <Input
                    type="text"
                    value={editNicName}
                    onChange={(e) => setEditNicName(e.target.value)}
                    placeholder="e.g., eth0"
                    required
                  />
                </div>
                <div className="flex gap-2 justify-end pt-4">
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() => { setEditingMapping(null); setEditNicName(''); setEditSwitch(''); setEditPortName(''); setSelectedFabric(''); setSwitches([]); setPorts([]); setPortFilter(''); }}
                  >
                    Cancel
                  </Button>
                  <Button type="submit">Save</Button>
                </div>
              </form>
            </CardContent>
          </Card>
        </div>
      )}

      {/* Add Mapping Modal */}
      {showMapForm && selectedNode && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <Card className="w-full max-w-md">
            <CardHeader>
              <CardTitle>Add Port Mapping to {selectedNode.name}</CardTitle>
            </CardHeader>
            <CardContent>
              <form onSubmit={handleAddMapping} className="space-y-4">
                <div>
                  <label className="block text-sm font-medium text-muted-foreground mb-1">Fabric *</label>
                  <select
                    value={selectedFabric}
                    onChange={(e) => handleFabricChangeForAdd(e.target.value)}
                    required
                    className="w-full px-3 py-2 border border-input rounded-md bg-background text-foreground"
                  >
                    <option value="">Select a fabric...</option>
                    {fabrics.map((f) => (
                      <option key={f.id} value={f.name}>{f.name}</option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="block text-sm font-medium text-muted-foreground mb-1">Switch *</label>
                  <select
                    value={newMapping.switch}
                    onChange={(e) => handleSwitchChangeForAdd(e.target.value)}
                    required
                    disabled={!selectedFabric}
                    className="w-full px-3 py-2 border border-input rounded-md bg-background text-foreground disabled:opacity-50"
                  >
                    <option value="">Select a switch...</option>
                    {switches.map((sw) => (
                      <option key={sw.id} value={sw.name}>{sw.name} ({sw.ip_address})</option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="block text-sm font-medium text-muted-foreground mb-1">Port *</label>
                  <Input
                    type="text"
                    value={portFilter}
                    onChange={(e) => setPortFilter(e.target.value)}
                    placeholder="Filter ports..."
                    className="mb-2"
                  />
                  <select
                    value={newMapping.port_name}
                    onChange={(e) => setNewMapping({ ...newMapping, port_name: e.target.value })}
                    required
                    disabled={!newMapping.switch || portsLoading}
                    className="w-full px-3 py-2 border border-input rounded-md bg-background text-foreground disabled:opacity-50"
                    size={6}
                  >
                    {portsLoading ? (
                      <option value="">Loading ports...</option>
                    ) : filteredPorts.length === 0 ? (
                      <option value="">No ports found</option>
                    ) : (
                      filteredPorts.map((p) => (
                        <option key={p.id} value={p.name}>{p.name}</option>
                      ))
                    )}
                  </select>
                </div>
                <div>
                  <label className="block text-sm font-medium text-muted-foreground mb-1">NIC Name *</label>
                  <Input
                    type="text"
                    value={newMapping.nic_name}
                    onChange={(e) => setNewMapping({ ...newMapping, nic_name: e.target.value })}
                    placeholder="e.g., eth0"
                    required
                  />
                </div>
                <div className="flex gap-2 justify-end pt-4">
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() => { setShowMapForm(false); setSelectedFabric(''); setSwitches([]); setPorts([]); setPortFilter(''); }}
                  >
                    Cancel
                  </Button>
                  <Button type="submit">Add Mapping</Button>
                </div>
              </form>
            </CardContent>
          </Card>
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Nodes List */}
        <Card>
          <CardHeader>
            <CardTitle>Nodes</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-3">
              <Input
                value={nodeSearch}
                onChange={(e) => setNodeSearch(e.target.value)}
                placeholder="Search nodes by name/hostname/IP..."
              />
              <div className="text-xs text-muted-foreground">
                {visibleNodes.length} node{visibleNodes.length !== 1 ? 's' : ''}
              </div>
            </div>
            <div className="h-4" />
            {loading ? (
              <div className="text-muted-foreground">Loading...</div>
            ) : visibleNodes.length === 0 ? (
              <div className="text-muted-foreground">No compute nodes found</div>
            ) : (
              <div className="space-y-2">
                {visibleNodes.map((node) => (
                  <div
                    key={node.id}
                    className={`p-4 rounded-lg border transition-colors ${
                      selectedNode?.id === node.id
                        ? 'bg-primary/10 border-primary'
                        : 'bg-card border-border hover:border-primary'
                    }`}
                  >
                    <div className="flex items-center justify-between">
                      <button
                        onClick={() => loadMappings(node)}
                        className="flex-1 text-left"
                      >
                        <div className="font-medium text-foreground">{node.name}</div>
                        <div className="text-sm text-muted-foreground">
                          {node.hostname || node.ip_address || 'No hostname/IP'}
                        </div>
                      </button>
                      <div className="flex items-center gap-2 ml-4">
                        <Button variant="ghost" size="sm" asChild>
                          <Link href={`/compute-nodes/${encodeURIComponent(node.name)}`}>Details</Link>
                        </Button>
                        <Button variant="ghost" size="sm" onClick={() => handleEditNode(node)}>Edit</Button>
                        <Button variant="ghost" size="sm" onClick={() => handleDeleteNode(node)} className="text-destructive hover:text-destructive">Delete</Button>
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>

        {/* Interfaces & Port Mappings */}
        <div className="space-y-6">
          <Card>
            <CardHeader className="flex-row items-center justify-between">
              <CardTitle>
                {selectedNode ? (
                  <span>
                    Selected: <span className="text-foreground">{selectedNode.name}</span>
                  </span>
                ) : (
                  'Select a node'
                )}
              </CardTitle>
              {selectedNode && (
                <div className="flex items-center gap-2">
                  <Button size="sm" variant="outline" asChild>
                    <Link href={`/compute-nodes/${encodeURIComponent(selectedNode.name)}`}>Open details</Link>
                  </Button>
                  <Button size="sm" variant="secondary" onClick={() => setShowInterfaceForm(true)}>
                    + Interface
                  </Button>
                  <Button size="sm" onClick={handleOpenAddMapping}>
                    + Mapping
                  </Button>
                </div>
              )}
            </CardHeader>
            <CardContent>
              {!selectedNode ? (
                <div className="text-muted-foreground">
                  Pick a node on the left to manage its interfaces and port mappings.
                </div>
              ) : (
                <Tabs defaultValue="mappings" className="w-full">
                  <TabsList>
                    <TabsTrigger value="mappings">Port Mappings ({mappings.length})</TabsTrigger>
                    <TabsTrigger value="interfaces">Interfaces ({interfaces.length})</TabsTrigger>
                  </TabsList>

                  <TabsContent value="mappings" className="mt-4">
                    {mappingsLoading ? (
                      <div className="text-muted-foreground">Loading mappings...</div>
                    ) : mappings.length === 0 ? (
                      <div className="text-muted-foreground">No port mappings</div>
                    ) : (
                      <Table>
                        <TableHeader>
                          <TableRow>
                            <TableHead>NIC</TableHead>
                            <TableHead>Switch</TableHead>
                            <TableHead>Port</TableHead>
                            <TableHead>Interface</TableHead>
                            <TableHead className="text-right">Actions</TableHead>
                          </TableRow>
                        </TableHeader>
                        <TableBody>
                          {mappings.map((mapping) => {
                            const assignedInterface = interfaces.find((i) => i.id === mapping.interface_id);
                            return (
                              <TableRow key={mapping.id}>
                                <TableCell className="font-mono text-sm">{mapping.nic_name}</TableCell>
                                <TableCell>{mapping.switch_port?.switch?.name || 'Unknown'}</TableCell>
                                <TableCell>{mapping.switch_port?.name || 'Unknown'}</TableCell>
                                <TableCell>
                                  <div className="flex items-center gap-2">
                                    <select
                                      value={mapping.interface_id || ''}
                                      onChange={(e) => handleAssignMapping(mapping.id, e.target.value || null)}
                                      className="text-xs px-2 py-1 border border-input rounded bg-background text-foreground"
                                    >
                                      <option value="">Unassigned</option>
                                      {interfaces.map((iface) => (
                                        <option key={iface.id} value={iface.id}>
                                          {iface.role.toUpperCase()}
                                        </option>
                                      ))}
                                    </select>
                                    {assignedInterface?.role ? (
                                      <Badge variant={assignedInterface.role === 'compute' ? 'compute' : 'storage'}>
                                        {assignedInterface.role.toUpperCase()}
                                      </Badge>
                                    ) : null}
                                  </div>
                                </TableCell>
                                <TableCell className="text-right">
                                  <div className="inline-flex items-center gap-1">
                                    <Button variant="ghost" size="sm" onClick={() => handleEditMapping(mapping)}>
                                      Edit
                                    </Button>
                                    <Button
                                      variant="ghost"
                                      size="sm"
                                      onClick={() => handleDeleteMapping(mapping)}
                                      className="text-destructive hover:text-destructive"
                                    >
                                      Remove
                                    </Button>
                                  </div>
                                </TableCell>
                              </TableRow>
                            );
                          })}
                        </TableBody>
                      </Table>
                    )}
                  </TabsContent>

                  <TabsContent value="interfaces" className="mt-4">
                    {interfacesLoading ? (
                      <div className="text-muted-foreground">Loading...</div>
                    ) : interfaces.length === 0 ? (
                      <div className="text-muted-foreground">
                        No interfaces defined. Add compute/storage interfaces to organize port mappings.
                      </div>
                    ) : (
                      <Table>
                        <TableHeader>
                          <TableRow>
                            <TableHead>Role</TableHead>
                            <TableHead>Interface ID</TableHead>
                            <TableHead className="text-right">Actions</TableHead>
                          </TableRow>
                        </TableHeader>
                        <TableBody>
                          {interfaces.map((iface) => (
                            <TableRow key={iface.id}>
                              <TableCell>
                                <Badge variant={iface.role === 'compute' ? 'compute' : 'storage'}>
                                  {iface.role.toUpperCase()}
                                </Badge>
                              </TableCell>
                              <TableCell className="font-mono text-xs">{iface.id}</TableCell>
                              <TableCell className="text-right">
                                <Button
                                  variant="ghost"
                                  size="sm"
                                  onClick={() => handleDeleteInterface(iface.id)}
                                  className="text-destructive hover:text-destructive"
                                >
                                  Delete
                                </Button>
                              </TableCell>
                            </TableRow>
                          ))}
                        </TableBody>
                      </Table>
                    )}
                  </TabsContent>
                </Tabs>
              )}
            </CardContent>
          </Card>

          {/* Interface Creation Modal */}
          {showInterfaceForm && selectedNode && (
            <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
              <Card className="w-full max-w-sm">
                <CardHeader>
                  <CardTitle>Add Interface</CardTitle>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div>
                    <label className="block text-sm font-medium text-muted-foreground mb-2">Role</label>
                    <select
                      value={newInterfaceRole}
                      onChange={(e) => setNewInterfaceRole(e.target.value as 'compute' | 'storage')}
                      className="w-full px-3 py-2 border border-input rounded-md bg-background text-foreground"
                    >
                      <option value="compute">Compute</option>
                      <option value="storage">Storage</option>
                    </select>
                  </div>
                  <div className="flex gap-2 justify-end pt-4">
                    <Button variant="outline" onClick={() => setShowInterfaceForm(false)}>Cancel</Button>
                    <Button onClick={() => handleCreateInterface(newInterfaceRole)}>Create</Button>
                  </div>
                </CardContent>
              </Card>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
