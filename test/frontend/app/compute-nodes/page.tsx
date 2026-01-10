'use client';

import { useEffect, useState, useCallback } from 'react';
import { computeNodesAPI, ComputeNode, PortMapping, ComputeNodeInterface, fabricsAPI, switchesAPI, portsAPI, Fabric, Switch, SwitchPort } from '@/lib/api';

export default function ComputeNodesPage() {
  const [nodes, setNodes] = useState<ComputeNode[]>([]);
  const [selectedNode, setSelectedNode] = useState<ComputeNode | null>(null);
  const [mappings, setMappings] = useState<PortMapping[]>([]);
  const [loading, setLoading] = useState(true);
  const [mappingsLoading, setMappingsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
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

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-zinc-900 dark:text-white">Compute Nodes</h1>
          <p className="text-zinc-600 dark:text-zinc-400">Manage compute nodes and port mappings</p>
        </div>
        <button
          onClick={() => setShowCreateForm(true)}
          className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
        >
          + Create Node
        </button>
      </div>

      {error && (
        <div className="p-4 bg-red-100 dark:bg-red-900/20 text-red-700 dark:text-red-400 rounded-lg">
          {error}
          <button onClick={() => setError(null)} className="ml-4 underline">Dismiss</button>
        </div>
      )}

      {/* Edit Node Modal */}
      {editingNode && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white dark:bg-zinc-900 p-6 rounded-lg w-full max-w-md">
            <h2 className="text-xl font-semibold mb-4 text-zinc-900 dark:text-white">Edit Compute Node</h2>
            <form onSubmit={handleUpdateNode} className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-zinc-700 dark:text-zinc-300 mb-1">Name *</label>
                <input
                  type="text"
                  value={editNodeName}
                  onChange={(e) => setEditNodeName(e.target.value)}
                  required
                  className="w-full px-3 py-2 border border-zinc-300 dark:border-zinc-700 rounded-lg bg-white dark:bg-zinc-800 text-zinc-900 dark:text-white"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-zinc-700 dark:text-zinc-300 mb-1">Hostname</label>
                <input
                  type="text"
                  value={editNodeHostname}
                  onChange={(e) => setEditNodeHostname(e.target.value)}
                  className="w-full px-3 py-2 border border-zinc-300 dark:border-zinc-700 rounded-lg bg-white dark:bg-zinc-800 text-zinc-900 dark:text-white"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-zinc-700 dark:text-zinc-300 mb-1">IP Address</label>
                <input
                  type="text"
                  value={editNodeIP}
                  onChange={(e) => setEditNodeIP(e.target.value)}
                  className="w-full px-3 py-2 border border-zinc-300 dark:border-zinc-700 rounded-lg bg-white dark:bg-zinc-800 text-zinc-900 dark:text-white"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-zinc-700 dark:text-zinc-300 mb-1">MAC Address</label>
                <input
                  type="text"
                  value={editNodeMAC}
                  onChange={(e) => setEditNodeMAC(e.target.value)}
                  className="w-full px-3 py-2 border border-zinc-300 dark:border-zinc-700 rounded-lg bg-white dark:bg-zinc-800 text-zinc-900 dark:text-white"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-zinc-700 dark:text-zinc-300 mb-1">Description</label>
                <textarea
                  value={editNodeDescription}
                  onChange={(e) => setEditNodeDescription(e.target.value)}
                  rows={3}
                  className="w-full px-3 py-2 border border-zinc-300 dark:border-zinc-700 rounded-lg bg-white dark:bg-zinc-800 text-zinc-900 dark:text-white"
                />
              </div>
              <div className="flex gap-2 justify-end">
                <button
                  type="button"
                  onClick={() => setEditingNode(null)}
                  className="px-4 py-2 text-zinc-600 dark:text-zinc-400 hover:text-zinc-900 dark:hover:text-white"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
                >
                  Save
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Create Node Modal */}
      {showCreateForm && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white dark:bg-zinc-900 p-6 rounded-lg w-full max-w-md">
            <h2 className="text-xl font-semibold mb-4 text-zinc-900 dark:text-white">Create Compute Node</h2>
            <form onSubmit={handleCreateNode} className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-zinc-700 dark:text-zinc-300 mb-1">Name *</label>
                <input
                  type="text"
                  value={newNode.name}
                  onChange={(e) => setNewNode({ ...newNode, name: e.target.value })}
                  required
                  className="w-full px-3 py-2 border border-zinc-300 dark:border-zinc-700 rounded-lg bg-white dark:bg-zinc-800 text-zinc-900 dark:text-white"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-zinc-700 dark:text-zinc-300 mb-1">Hostname</label>
                <input
                  type="text"
                  value={newNode.hostname}
                  onChange={(e) => setNewNode({ ...newNode, hostname: e.target.value })}
                  className="w-full px-3 py-2 border border-zinc-300 dark:border-zinc-700 rounded-lg bg-white dark:bg-zinc-800 text-zinc-900 dark:text-white"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-zinc-700 dark:text-zinc-300 mb-1">IP Address</label>
                <input
                  type="text"
                  value={newNode.ip_address}
                  onChange={(e) => setNewNode({ ...newNode, ip_address: e.target.value })}
                  className="w-full px-3 py-2 border border-zinc-300 dark:border-zinc-700 rounded-lg bg-white dark:bg-zinc-800 text-zinc-900 dark:text-white"
                />
              </div>
              <div className="flex gap-2 justify-end">
                <button
                  type="button"
                  onClick={() => setShowCreateForm(false)}
                  className="px-4 py-2 text-zinc-600 dark:text-zinc-400 hover:text-zinc-900 dark:hover:text-white"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
                >
                  Create
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Edit Mapping Modal */}
      {editingMapping && selectedNode && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white dark:bg-zinc-900 p-6 rounded-lg w-full max-w-md">
            <h2 className="text-xl font-semibold mb-4 text-zinc-900 dark:text-white">
              Edit Port Mapping
            </h2>
            <form onSubmit={handleUpdateMapping} className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-zinc-700 dark:text-zinc-300 mb-1">Fabric *</label>
                <select
                  value={selectedFabric}
                  onChange={(e) => handleFabricChange(e.target.value)}
                  required
                  className="w-full px-3 py-2 border border-zinc-300 dark:border-zinc-700 rounded-lg bg-white dark:bg-zinc-800 text-zinc-900 dark:text-white"
                >
                  <option value="">Select a fabric...</option>
                  {fabrics.map((f) => (
                    <option key={f.id} value={f.name}>{f.name}</option>
                  ))}
                </select>
              </div>
              <div>
                <label className="block text-sm font-medium text-zinc-700 dark:text-zinc-300 mb-1">Switch *</label>
                <select
                  value={editSwitch}
                  onChange={(e) => handleSwitchChange(e.target.value)}
                  required
                  disabled={!selectedFabric}
                  className="w-full px-3 py-2 border border-zinc-300 dark:border-zinc-700 rounded-lg bg-white dark:bg-zinc-800 text-zinc-900 dark:text-white disabled:opacity-50"
                >
                  <option value="">Select a switch...</option>
                  {switches.map((sw) => (
                    <option key={sw.id} value={sw.name}>{sw.name} ({sw.ip_address})</option>
                  ))}
                </select>
              </div>
              <div>
                <label className="block text-sm font-medium text-zinc-700 dark:text-zinc-300 mb-1">Port *</label>
                <input
                  type="text"
                  value={portFilter}
                  onChange={(e) => setPortFilter(e.target.value)}
                  placeholder="Filter ports..."
                  className="w-full px-3 py-2 mb-2 border border-zinc-300 dark:border-zinc-700 rounded-lg bg-white dark:bg-zinc-800 text-zinc-900 dark:text-white"
                />
                <select
                  value={editPortName}
                  onChange={(e) => setEditPortName(e.target.value)}
                  required
                  disabled={!editSwitch || portsLoading}
                  className="w-full px-3 py-2 border border-zinc-300 dark:border-zinc-700 rounded-lg bg-white dark:bg-zinc-800 text-zinc-900 dark:text-white disabled:opacity-50"
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
                <label className="block text-sm font-medium text-zinc-700 dark:text-zinc-300 mb-1">NIC Name *</label>
                <input
                  type="text"
                  value={editNicName}
                  onChange={(e) => setEditNicName(e.target.value)}
                  placeholder="e.g., eth0"
                  required
                  className="w-full px-3 py-2 border border-zinc-300 dark:border-zinc-700 rounded-lg bg-white dark:bg-zinc-800 text-zinc-900 dark:text-white"
                />
              </div>
              <div className="flex gap-2 justify-end">
                <button
                  type="button"
                  onClick={() => { setEditingMapping(null); setEditNicName(''); setEditSwitch(''); setEditPortName(''); setSelectedFabric(''); setSwitches([]); setPorts([]); setPortFilter(''); }}
                  className="px-4 py-2 text-zinc-600 dark:text-zinc-400 hover:text-zinc-900 dark:hover:text-white"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
                >
                  Save
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Add Mapping Modal */}
      {showMapForm && selectedNode && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white dark:bg-zinc-900 p-6 rounded-lg w-full max-w-md">
            <h2 className="text-xl font-semibold mb-4 text-zinc-900 dark:text-white">
              Add Port Mapping to {selectedNode.name}
            </h2>
            <form onSubmit={handleAddMapping} className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-zinc-700 dark:text-zinc-300 mb-1">Fabric *</label>
                <select
                  value={selectedFabric}
                  onChange={(e) => handleFabricChangeForAdd(e.target.value)}
                  required
                  className="w-full px-3 py-2 border border-zinc-300 dark:border-zinc-700 rounded-lg bg-white dark:bg-zinc-800 text-zinc-900 dark:text-white"
                >
                  <option value="">Select a fabric...</option>
                  {fabrics.map((f) => (
                    <option key={f.id} value={f.name}>{f.name}</option>
                  ))}
                </select>
              </div>
              <div>
                <label className="block text-sm font-medium text-zinc-700 dark:text-zinc-300 mb-1">Switch *</label>
                <select
                  value={newMapping.switch}
                  onChange={(e) => handleSwitchChangeForAdd(e.target.value)}
                  required
                  disabled={!selectedFabric}
                  className="w-full px-3 py-2 border border-zinc-300 dark:border-zinc-700 rounded-lg bg-white dark:bg-zinc-800 text-zinc-900 dark:text-white disabled:opacity-50"
                >
                  <option value="">Select a switch...</option>
                  {switches.map((sw) => (
                    <option key={sw.id} value={sw.name}>{sw.name} ({sw.ip_address})</option>
                  ))}
                </select>
              </div>
              <div>
                <label className="block text-sm font-medium text-zinc-700 dark:text-zinc-300 mb-1">Port *</label>
                <input
                  type="text"
                  value={portFilter}
                  onChange={(e) => setPortFilter(e.target.value)}
                  placeholder="Filter ports..."
                  className="w-full px-3 py-2 mb-2 border border-zinc-300 dark:border-zinc-700 rounded-lg bg-white dark:bg-zinc-800 text-zinc-900 dark:text-white"
                />
                <select
                  value={newMapping.port_name}
                  onChange={(e) => setNewMapping({ ...newMapping, port_name: e.target.value })}
                  required
                  disabled={!newMapping.switch || portsLoading}
                  className="w-full px-3 py-2 border border-zinc-300 dark:border-zinc-700 rounded-lg bg-white dark:bg-zinc-800 text-zinc-900 dark:text-white disabled:opacity-50"
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
                <label className="block text-sm font-medium text-zinc-700 dark:text-zinc-300 mb-1">NIC Name *</label>
                <input
                  type="text"
                  value={newMapping.nic_name}
                  onChange={(e) => setNewMapping({ ...newMapping, nic_name: e.target.value })}
                  placeholder="e.g., eth0"
                  required
                  className="w-full px-3 py-2 border border-zinc-300 dark:border-zinc-700 rounded-lg bg-white dark:bg-zinc-800 text-zinc-900 dark:text-white"
                />
              </div>
              <div className="flex gap-2 justify-end">
                <button
                  type="button"
                  onClick={() => { setShowMapForm(false); setSelectedFabric(''); setSwitches([]); setPorts([]); setPortFilter(''); }}
                  className="px-4 py-2 text-zinc-600 dark:text-zinc-400 hover:text-zinc-900 dark:hover:text-white"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
                >
                  Add Mapping
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Nodes List */}
        <div className="space-y-4">
          <h2 className="text-lg font-semibold text-zinc-900 dark:text-white">Nodes</h2>
          {loading ? (
            <div className="text-zinc-600 dark:text-zinc-400">Loading...</div>
          ) : nodes.length === 0 ? (
            <div className="text-zinc-600 dark:text-zinc-400">No compute nodes found</div>
          ) : (
            <div className="space-y-2">
              {nodes.map((node) => (
                <div
                  key={node.id}
                  className={`p-4 rounded-lg border transition-colors ${
                    selectedNode?.id === node.id
                      ? 'bg-blue-50 dark:bg-blue-900/20 border-blue-500'
                      : 'bg-white dark:bg-zinc-900 border-zinc-200 dark:border-zinc-800'
                  }`}
                >
                  <div className="flex items-center justify-between">
                    <button
                      onClick={() => loadMappings(node)}
                      className="flex-1 text-left"
                    >
                      <div className="font-medium text-zinc-900 dark:text-white">{node.name}</div>
                      <div className="text-sm text-zinc-600 dark:text-zinc-400">
                        {node.hostname || 'No hostname'} • {node.ip_address || 'No IP'}
                      </div>
                    </button>
                    <div className="flex gap-2">
                      <button
                        onClick={() => handleEditNode(node)}
                        className="text-blue-600 hover:text-blue-700 text-sm px-2"
                      >
                        Edit
                      </button>
                      <button
                        onClick={() => handleDeleteNode(node)}
                        className="text-red-600 hover:text-red-700 text-sm px-2"
                      >
                        Delete
                      </button>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Interfaces & Port Mappings */}
        <div className="space-y-4">
          {/* Interfaces Section */}
          <div>
            <div className="flex items-center justify-between mb-2">
              <h2 className="text-lg font-semibold text-zinc-900 dark:text-white">
                Interfaces {selectedNode && `(${selectedNode.name})`}
              </h2>
              {selectedNode && (
                <button
                  onClick={() => setShowInterfaceForm(true)}
                  className="text-sm px-3 py-1 bg-purple-600 text-white rounded hover:bg-purple-700"
                >
                  + Add Interface
                </button>
              )}
            </div>
            {!selectedNode ? (
              <div className="text-zinc-600 dark:text-zinc-400 text-sm">Select a node to manage interfaces</div>
            ) : interfacesLoading ? (
              <div className="text-zinc-600 dark:text-zinc-400 text-sm">Loading...</div>
            ) : interfaces.length === 0 ? (
              <div className="text-zinc-600 dark:text-zinc-400 text-sm">No interfaces defined. Add compute/storage interfaces to organize port mappings.</div>
            ) : (
              <div className="flex gap-2 flex-wrap">
                {interfaces.map((iface) => (
                  <div
                    key={iface.id}
                    className={`px-3 py-2 rounded border text-sm ${
                      iface.role === 'compute'
                        ? 'bg-blue-50 dark:bg-blue-900/20 border-blue-300 dark:border-blue-700'
                        : 'bg-orange-50 dark:bg-orange-900/20 border-orange-300 dark:border-orange-700'
                    }`}
                  >
                    <div className="flex items-center gap-2">
                      <span className={`font-medium ${iface.role === 'compute' ? 'text-blue-700 dark:text-blue-300' : 'text-orange-700 dark:text-orange-300'}`}>
                        {iface.role.toUpperCase()}
                      </span>
                      <button
                        onClick={() => handleDeleteInterface(iface.id)}
                        className="text-red-500 hover:text-red-700 text-xs"
                        title="Delete interface"
                      >
                        ×
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Interface Creation Modal */}
          {showInterfaceForm && selectedNode && (
            <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
              <div className="bg-white dark:bg-zinc-900 p-6 rounded-lg w-full max-w-sm">
                <h3 className="text-lg font-semibold mb-4 text-zinc-900 dark:text-white">Add Interface</h3>
                <div className="space-y-4">
                  <div>
                    <label className="block text-sm font-medium text-zinc-700 dark:text-zinc-300 mb-2">Role</label>
                    <select
                      value={newInterfaceRole}
                      onChange={(e) => setNewInterfaceRole(e.target.value as 'compute' | 'storage')}
                      className="w-full px-3 py-2 border border-zinc-300 dark:border-zinc-700 rounded-lg bg-white dark:bg-zinc-800 text-zinc-900 dark:text-white"
                    >
                      <option value="compute">Compute</option>
                      <option value="storage">Storage</option>
                    </select>
                  </div>
                  <div className="flex gap-2 justify-end">
                    <button
                      onClick={() => setShowInterfaceForm(false)}
                      className="px-4 py-2 text-zinc-600 dark:text-zinc-400"
                    >
                      Cancel
                    </button>
                    <button
                      onClick={() => handleCreateInterface(newInterfaceRole)}
                      className="px-4 py-2 bg-purple-600 text-white rounded hover:bg-purple-700"
                    >
                      Create
                    </button>
                  </div>
                </div>
              </div>
            </div>
          )}

          {/* Port Mappings Section */}
          <div className="flex items-center justify-between">
            <h2 className="text-lg font-semibold text-zinc-900 dark:text-white">
              Port Mappings
            </h2>
            {selectedNode && (
              <button
                onClick={handleOpenAddMapping}
                className="text-sm px-3 py-1 bg-green-600 text-white rounded hover:bg-green-700"
              >
                + Add Mapping
              </button>
            )}
          </div>
          {!selectedNode ? (
            <div className="text-zinc-600 dark:text-zinc-400">Select a node to view port mappings</div>
          ) : mappingsLoading ? (
            <div className="text-zinc-600 dark:text-zinc-400">Loading mappings...</div>
          ) : mappings.length === 0 ? (
            <div className="text-zinc-600 dark:text-zinc-400">No port mappings</div>
          ) : (
            <div className="space-y-2">
              {mappings.map((mapping) => {
                const assignedInterface = interfaces.find(i => i.id === mapping.interface_id);
                return (
                  <div
                    key={mapping.id}
                    className="p-3 bg-white dark:bg-zinc-900 rounded border border-zinc-200 dark:border-zinc-800"
                  >
                    <div className="flex items-center justify-between">
                      <div className="flex-1">
                        <div className="font-mono text-sm text-zinc-900 dark:text-white">
                          {mapping.nic_name} → {mapping.switch_port?.switch?.name || 'Unknown'}/{mapping.switch_port?.name || 'Unknown'}
                        </div>
                        <div className="text-xs text-zinc-500 mt-1">
                          Port ID: {mapping.switch_port_id}
                        </div>
                      </div>
                      <div className="flex items-center gap-3">
                        {/* Interface Assignment Dropdown */}
                        <select
                          value={mapping.interface_id || ''}
                          onChange={(e) => handleAssignMapping(mapping.id, e.target.value || null)}
                          className={`text-xs px-2 py-1 border rounded ${
                            assignedInterface?.role === 'compute'
                              ? 'border-blue-300 bg-blue-50 dark:bg-blue-900/20'
                              : assignedInterface?.role === 'storage'
                              ? 'border-orange-300 bg-orange-50 dark:bg-orange-900/20'
                              : 'border-zinc-300 dark:border-zinc-600 bg-white dark:bg-zinc-800'
                          }`}
                        >
                          <option value="">Unassigned</option>
                          {interfaces.map((iface) => (
                            <option key={iface.id} value={iface.id}>
                              {iface.role.toUpperCase()}
                            </option>
                          ))}
                        </select>
                        <button
                          onClick={() => handleEditMapping(mapping)}
                          className="text-blue-600 hover:text-blue-700 text-xs"
                        >
                          Edit
                        </button>
                        <button
                          onClick={() => handleDeleteMapping(mapping)}
                          className="text-red-600 hover:text-red-700 text-xs"
                        >
                          Remove
                        </button>
                      </div>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
