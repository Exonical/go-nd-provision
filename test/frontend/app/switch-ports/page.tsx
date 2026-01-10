'use client';

import { useState, useEffect, useCallback } from 'react';
import Link from 'next/link';
import {
  fabricsAPI,
  switchesAPI,
  portsAPI,
  computeNodesAPI,
  portMappingsAPI,
  Fabric,
  Switch,
  SwitchPort,
  ComputeNode,
  ComputeNodeInterface,
  PortMapping,
  BulkPortAssignment,
} from '@/lib/api';

interface PortAssignment {
  port: SwitchPort;
  nodeId: string | null;
  interfaceId: string | null;
  originalNodeId: string | null;
  originalInterfaceId: string | null;
  mapping?: PortMapping;
}

export default function SwitchPortsPage() {
  const [fabrics, setFabrics] = useState<Fabric[]>([]);
  const [switches, setSwitches] = useState<Switch[]>([]);
  const [ports, setPorts] = useState<SwitchPort[]>([]);
  const [nodes, setNodes] = useState<ComputeNode[]>([]);
  const [nodeInterfaces, setNodeInterfaces] = useState<Record<string, ComputeNodeInterface[]>>({});
  const [portMappings, setPortMappings] = useState<PortMapping[]>([]);

  const [selectedFabric, setSelectedFabric] = useState<string>('');
  const [selectedSwitch, setSelectedSwitch] = useState<string>('');
  const [assignments, setAssignments] = useState<PortAssignment[]>([]);
  const [hasChanges, setHasChanges] = useState(false);
  const [saving, setSaving] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);

  // Load fabrics on mount
  useEffect(() => {
    fabricsAPI.list().then(setFabrics).catch(console.error);
    computeNodesAPI.list().then(setNodes).catch(console.error);
  }, []);

  // Load switches when fabric changes
  useEffect(() => {
    if (selectedFabric) {
      switchesAPI.list(selectedFabric).then(setSwitches).catch(console.error);
      setSelectedSwitch('');
      setPorts([]);
      setAssignments([]);
    }
  }, [selectedFabric]);

  // Load ports and mappings when switch changes
  useEffect(() => {
    if (selectedFabric && selectedSwitch) {
      setLoading(true);
      Promise.all([
        portsAPI.list(selectedFabric, selectedSwitch),
        portsAPI.getMappingsBySwitch(selectedSwitch),
      ])
        .then(([portsData, mappingsData]) => {
          setPorts(portsData);
          setPortMappings(mappingsData);
        })
        .catch(console.error)
        .finally(() => setLoading(false));
    }
  }, [selectedFabric, selectedSwitch]);

  // Load interfaces for all nodes
  useEffect(() => {
    const loadInterfaces = async () => {
      const interfaces: Record<string, ComputeNodeInterface[]> = {};
      for (const node of nodes) {
        try {
          interfaces[node.id] = await computeNodesAPI.getInterfaces(node.id);
        } catch {
          interfaces[node.id] = [];
        }
      }
      setNodeInterfaces(interfaces);
    };
    if (nodes.length > 0) {
      loadInterfaces();
    }
  }, [nodes]);

  // Build assignments from ports and mappings
  useEffect(() => {
    const newAssignments: PortAssignment[] = ports.map((port) => {
      const mapping = portMappings.find((m) => m.switch_port_id === port.id);
      return {
        port,
        nodeId: mapping?.compute_node_id || null,
        interfaceId: mapping?.interface_id || null,
        originalNodeId: mapping?.compute_node_id || null,
        originalInterfaceId: mapping?.interface_id || null,
        mapping,
      };
    });
    setAssignments(newAssignments);
    setHasChanges(false);
  }, [ports, portMappings]);

  const updateAssignment = useCallback(
    (portId: string, field: 'nodeId' | 'interfaceId', value: string | null) => {
      setAssignments((prev) =>
        prev.map((a) => {
          if (a.port.id !== portId) return a;
          const updated = { ...a, [field]: value };
          // Clear interface if node changes
          if (field === 'nodeId') {
            updated.interfaceId = null;
          }
          return updated;
        })
      );
      setHasChanges(true);
    },
    []
  );

  const getChangedAssignments = useCallback((): BulkPortAssignment[] => {
    return assignments
      .filter(
        (a) =>
          a.nodeId !== a.originalNodeId || a.interfaceId !== a.originalInterfaceId
      )
      .map((a) => ({
        switch_port_id: a.port.id,
        node_id: a.nodeId,
        interface_id: a.interfaceId,
      }));
  }, [assignments]);

  const handleSave = async () => {
    const changes = getChangedAssignments();
    if (changes.length === 0) return;

    setSaving(true);
    setError(null);
    setSuccessMessage(null);

    try {
      const result = await portMappingsAPI.bulkAssign(changes);
      const failed = result.results.filter((r) => !r.success);
      if (failed.length > 0) {
        setError(`${failed.length} assignments failed: ${failed.map((f) => f.error).join(', ')}`);
      } else {
        setSuccessMessage(`Successfully updated ${result.total} port assignments`);
      }
      // Reload mappings
      const mappingsData = await portsAPI.getMappingsBySwitch(selectedSwitch);
      setPortMappings(mappingsData);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save');
    } finally {
      setSaving(false);
    }
  };

  const handleReset = () => {
    setAssignments((prev) =>
      prev.map((a) => ({
        ...a,
        nodeId: a.originalNodeId,
        interfaceId: a.originalInterfaceId,
      }))
    );
    setHasChanges(false);
  };

  const changedCount = getChangedAssignments().length;

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-zinc-100">Switch Ports</h1>
          <p className="text-zinc-400">Bulk assign switch ports to compute nodes and interfaces</p>
        </div>
        <Link
          href="/"
          className="text-blue-400 hover:text-blue-300"
        >
          ← Back to Dashboard
        </Link>
      </div>

      {/* Fabric and Switch Selection */}
      <div className="bg-zinc-900 rounded-lg border border-zinc-800 p-6">
        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="block text-sm font-medium text-zinc-300 mb-1">Fabric</label>
            <select
              value={selectedFabric}
              onChange={(e) => setSelectedFabric(e.target.value)}
              className="w-full border border-zinc-700 bg-zinc-800 text-zinc-100 rounded-md px-3 py-2"
            >
              <option value="">Select a fabric...</option>
              {fabrics.map((f) => (
                <option key={f.id} value={f.id}>
                  {f.name}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label className="block text-sm font-medium text-zinc-300 mb-1">Switch</label>
            <select
              value={selectedSwitch}
              onChange={(e) => setSelectedSwitch(e.target.value)}
              disabled={!selectedFabric}
              className="w-full border border-zinc-700 bg-zinc-800 text-zinc-100 rounded-md px-3 py-2 disabled:bg-zinc-900 disabled:text-zinc-500"
            >
              <option value="">Select a switch...</option>
              {switches.map((s) => (
                <option key={s.id} value={s.id}>
                  {s.name} ({s.serial_number})
                </option>
              ))}
            </select>
          </div>
        </div>
      </div>

      {/* Error/Success Messages */}
      {error && (
        <div className="bg-red-900/50 border border-red-700 text-red-300 px-4 py-3 rounded">
          {error}
        </div>
      )}
      {successMessage && (
        <div className="bg-green-900/50 border border-green-700 text-green-300 px-4 py-3 rounded">
          {successMessage}
        </div>
      )}

      {/* Port Assignments Table */}
      {selectedSwitch && (
        <div className="bg-zinc-900 rounded-lg border border-zinc-800">
          <div className="px-6 py-4 border-b border-zinc-800 flex items-center justify-between">
            <h2 className="text-lg font-semibold text-zinc-100">Port Assignments</h2>
            <div className="flex items-center gap-4">
              {hasChanges && (
                <span className="text-sm text-orange-400">
                  {changedCount} unsaved change{changedCount !== 1 ? 's' : ''}
                </span>
              )}
              <button
                onClick={handleReset}
                disabled={!hasChanges || saving}
                className="px-4 py-2 text-zinc-300 border border-zinc-600 rounded hover:bg-zinc-800 disabled:opacity-50"
              >
                Reset
              </button>
              <button
                onClick={handleSave}
                disabled={!hasChanges || saving}
                className="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 disabled:opacity-50"
              >
                {saving ? 'Saving...' : 'Save Changes'}
              </button>
            </div>
          </div>

          {loading ? (
            <div className="p-8 text-center text-zinc-500">Loading ports...</div>
          ) : assignments.length === 0 ? (
            <div className="p-8 text-center text-zinc-500">No ports found</div>
          ) : (
            <div className="overflow-x-auto">
              <table className="min-w-full divide-y divide-zinc-800">
                <thead className="bg-zinc-800/50">
                  <tr>
                    <th className="px-6 py-3 text-left text-xs font-medium text-zinc-400 uppercase">
                      Port
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-zinc-400 uppercase">
                      Description
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-zinc-400 uppercase">
                      Compute Node
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-zinc-400 uppercase">
                      Interface
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-zinc-400 uppercase">
                      Status
                    </th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-zinc-800">
                  {assignments.map((assignment) => {
                    const isChanged =
                      assignment.nodeId !== assignment.originalNodeId ||
                      assignment.interfaceId !== assignment.originalInterfaceId;
                    const interfaces = assignment.nodeId
                      ? nodeInterfaces[assignment.nodeId] || []
                      : [];

                    return (
                      <tr
                        key={assignment.port.id}
                        className={isChanged ? 'bg-yellow-900/20' : ''}
                      >
                        <td className="px-6 py-4 whitespace-nowrap">
                          <div className="font-medium text-zinc-100">
                            {assignment.port.name}
                          </div>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap text-sm text-zinc-400">
                          {assignment.port.description || '-'}
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap">
                          <select
                            value={assignment.nodeId || ''}
                            onChange={(e) =>
                              updateAssignment(
                                assignment.port.id,
                                'nodeId',
                                e.target.value || null
                              )
                            }
                            className="border border-zinc-700 bg-zinc-800 text-zinc-100 rounded px-2 py-1 text-sm w-48"
                          >
                            <option value="">-- None --</option>
                            {nodes.map((node) => (
                              <option key={node.id} value={node.id}>
                                {node.name}
                              </option>
                            ))}
                          </select>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap">
                          <select
                            value={assignment.interfaceId || ''}
                            onChange={(e) =>
                              updateAssignment(
                                assignment.port.id,
                                'interfaceId',
                                e.target.value || null
                              )
                            }
                            disabled={!assignment.nodeId}
                            className="border border-zinc-700 bg-zinc-800 text-zinc-100 rounded px-2 py-1 text-sm w-32 disabled:bg-zinc-900 disabled:text-zinc-500"
                          >
                            <option value="">-- None --</option>
                            {interfaces.map((iface) => (
                              <option key={iface.id} value={iface.id}>
                                {iface.role}
                              </option>
                            ))}
                          </select>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap">
                          {isChanged ? (
                            <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-yellow-900/50 text-yellow-300">
                              Modified
                            </span>
                          ) : assignment.nodeId ? (
                            <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-900/50 text-green-300">
                              Assigned
                            </span>
                          ) : (
                            <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-zinc-700 text-zinc-300">
                              Unassigned
                            </span>
                          )}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
