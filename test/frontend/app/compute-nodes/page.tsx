'use client';

import { useEffect, useState, useCallback } from 'react';
import { computeNodesAPI, ComputeNode, PortMapping } from '@/lib/api';

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

  // Create node form state
  const [newNode, setNewNode] = useState({ name: '', hostname: '', ip_address: '' });

  // Map port form state
  const [newMapping, setNewMapping] = useState({ switch: '', port_name: '', nic_name: '' });

  // Edit mapping form state
  const [editNicName, setEditNicName] = useState('');
  const [editSwitch, setEditSwitch] = useState('');
  const [editPortName, setEditPortName] = useState('');

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
      const data = await computeNodesAPI.getPortMappings(node.name);
      setMappings(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load mappings');
    } finally {
      setMappingsLoading(false);
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

  const handleAddMapping = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedNode) return;
    try {
      await computeNodesAPI.addPortMapping(selectedNode.name, newMapping);
      setNewMapping({ switch: '', port_name: '', nic_name: '' });
      setShowMapForm(false);
      await loadMappings(selectedNode);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to add mapping');
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

  const handleEditMapping = (mapping: PortMapping) => {
    setEditingMapping(mapping);
    setEditNicName(mapping.nic_name);
    setEditSwitch(mapping.switch_port?.switch?.name || '');
    setEditPortName(mapping.switch_port?.name || '');
  };

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
                <label className="block text-sm font-medium text-zinc-700 dark:text-zinc-300 mb-1">Switch Name *</label>
                <input
                  type="text"
                  value={editSwitch}
                  onChange={(e) => setEditSwitch(e.target.value)}
                  placeholder="e.g., site1-leaf1"
                  required
                  className="w-full px-3 py-2 border border-zinc-300 dark:border-zinc-700 rounded-lg bg-white dark:bg-zinc-800 text-zinc-900 dark:text-white"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-zinc-700 dark:text-zinc-300 mb-1">Port Name *</label>
                <input
                  type="text"
                  value={editPortName}
                  onChange={(e) => setEditPortName(e.target.value)}
                  placeholder="e.g., Ethernet1/29"
                  required
                  className="w-full px-3 py-2 border border-zinc-300 dark:border-zinc-700 rounded-lg bg-white dark:bg-zinc-800 text-zinc-900 dark:text-white"
                />
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
                  onClick={() => { setEditingMapping(null); setEditNicName(''); setEditSwitch(''); setEditPortName(''); }}
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
                <label className="block text-sm font-medium text-zinc-700 dark:text-zinc-300 mb-1">Switch Name *</label>
                <input
                  type="text"
                  value={newMapping.switch}
                  onChange={(e) => setNewMapping({ ...newMapping, switch: e.target.value })}
                  placeholder="e.g., site1-leaf1"
                  required
                  className="w-full px-3 py-2 border border-zinc-300 dark:border-zinc-700 rounded-lg bg-white dark:bg-zinc-800 text-zinc-900 dark:text-white"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-zinc-700 dark:text-zinc-300 mb-1">Port Name *</label>
                <input
                  type="text"
                  value={newMapping.port_name}
                  onChange={(e) => setNewMapping({ ...newMapping, port_name: e.target.value })}
                  placeholder="e.g., Ethernet1/29"
                  required
                  className="w-full px-3 py-2 border border-zinc-300 dark:border-zinc-700 rounded-lg bg-white dark:bg-zinc-800 text-zinc-900 dark:text-white"
                />
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
                  onClick={() => setShowMapForm(false)}
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
                    <button
                      onClick={() => handleDeleteNode(node)}
                      className="text-red-600 hover:text-red-700 text-sm px-2"
                    >
                      Delete
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Port Mappings */}
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <h2 className="text-lg font-semibold text-zinc-900 dark:text-white">
              Port Mappings {selectedNode && `(${selectedNode.name})`}
            </h2>
            {selectedNode && (
              <button
                onClick={() => setShowMapForm(true)}
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
              {mappings.map((mapping) => (
                <div
                  key={mapping.id}
                  className="p-3 bg-white dark:bg-zinc-900 rounded border border-zinc-200 dark:border-zinc-800"
                >
                  <div className="flex items-center justify-between">
                    <div>
                      <div className="font-mono text-sm text-zinc-900 dark:text-white">
                        {mapping.nic_name} → {mapping.switch_port?.switch?.name || 'Unknown'}/{mapping.switch_port?.name || 'Unknown'}
                      </div>
                      <div className="text-xs text-zinc-500 mt-1">
                        Port ID: {mapping.switch_port_id}
                      </div>
                    </div>
                    <div className="flex gap-2">
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
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
