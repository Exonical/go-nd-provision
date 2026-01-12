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
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle, CardAction } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';

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
          <h1 className="text-2xl font-bold text-foreground">Switch Ports</h1>
          <p className="text-muted-foreground">Bulk assign switch ports to compute nodes and interfaces</p>
        </div>
        <Button variant="outline" asChild>
          <Link href="/">← Back to Dashboard</Link>
        </Button>
      </div>

      {/* Fabric and Switch Selection */}
      <Card>
        <CardContent className="pt-6">
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-muted-foreground mb-1">Fabric</label>
              <select
                value={selectedFabric}
                onChange={(e) => setSelectedFabric(e.target.value)}
                className="w-full border border-input bg-background text-foreground rounded-md px-3 py-2"
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
              <label className="block text-sm font-medium text-muted-foreground mb-1">Switch</label>
              <select
                value={selectedSwitch}
                onChange={(e) => setSelectedSwitch(e.target.value)}
                disabled={!selectedFabric}
                className="w-full border border-input bg-background text-foreground rounded-md px-3 py-2 disabled:opacity-50"
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
        </CardContent>
      </Card>

      {/* Error/Success Messages */}
      {error && (
        <div className="bg-destructive/20 border border-destructive text-destructive px-4 py-3 rounded-md">
          {error}
        </div>
      )}
      {successMessage && (
        <div className="bg-green-900/50 border border-green-700 text-green-300 px-4 py-3 rounded-md">
          {successMessage}
        </div>
      )}

      {/* Port Assignments Table */}
      {selectedSwitch && (
        <Card>
          <CardHeader className="flex-row items-center justify-between">
            <CardTitle>Port Assignments</CardTitle>
            <CardAction>
              <div className="flex items-center gap-4">
                {hasChanges && (
                  <span className="text-sm text-orange-400">
                    {changedCount} unsaved change{changedCount !== 1 ? 's' : ''}
                  </span>
                )}
                <Button variant="outline" onClick={handleReset} disabled={!hasChanges || saving}>
                  Reset
                </Button>
                <Button onClick={handleSave} disabled={!hasChanges || saving}>
                  {saving ? 'Saving...' : 'Save Changes'}
                </Button>
              </div>
            </CardAction>
          </CardHeader>
          <CardContent>
            {loading ? (
              <div className="p-8 text-center text-muted-foreground">Loading ports...</div>
            ) : assignments.length === 0 ? (
              <div className="p-8 text-center text-muted-foreground">No ports found</div>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Port</TableHead>
                    <TableHead>Description</TableHead>
                    <TableHead>Compute Node</TableHead>
                    <TableHead>Interface</TableHead>
                    <TableHead>Status</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {assignments.map((assignment) => {
                    const isChanged =
                      assignment.nodeId !== assignment.originalNodeId ||
                      assignment.interfaceId !== assignment.originalInterfaceId;
                    const interfaces = assignment.nodeId
                      ? nodeInterfaces[assignment.nodeId] || []
                      : [];

                    return (
                      <TableRow key={assignment.port.id} className={isChanged ? 'bg-yellow-900/20' : ''}>
                        <TableCell className="font-medium">{assignment.port.name}</TableCell>
                        <TableCell className="text-muted-foreground">{assignment.port.description || '-'}</TableCell>
                        <TableCell>
                          <select
                            value={assignment.nodeId || ''}
                            onChange={(e) =>
                              updateAssignment(
                                assignment.port.id,
                                'nodeId',
                                e.target.value || null
                              )
                            }
                            className="border border-input bg-background text-foreground rounded px-2 py-1 text-sm w-48"
                          >
                            <option value="">-- None --</option>
                            {nodes.map((node) => (
                              <option key={node.id} value={node.id}>
                                {node.name}
                              </option>
                            ))}
                          </select>
                        </TableCell>
                        <TableCell>
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
                            className="border border-input bg-background text-foreground rounded px-2 py-1 text-sm w-32 disabled:opacity-50"
                          >
                            <option value="">-- None --</option>
                            {interfaces.map((iface) => (
                              <option key={iface.id} value={iface.id}>
                                {iface.role}
                              </option>
                            ))}
                          </select>
                        </TableCell>
                        <TableCell>
                          {isChanged ? (
                            <Badge variant="outline" className="bg-yellow-900/50 text-yellow-300 border-yellow-700">
                              Modified
                            </Badge>
                          ) : assignment.nodeId ? (
                            <Badge variant="success">Assigned</Badge>
                          ) : (
                            <Badge variant="secondary">Unassigned</Badge>
                          )}
                        </TableCell>
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>
      )}
    </div>
  );
}
