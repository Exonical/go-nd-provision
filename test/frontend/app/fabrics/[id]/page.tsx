'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { useParams } from 'next/navigation';
import { switchesAPI, portsAPI, Switch, SwitchPort, PortMapping } from '@/lib/api';

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

  const loadSwitches = async () => {
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
  };

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
  }, [fabricId]);

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-2 text-sm text-zinc-600 dark:text-zinc-400">
        <Link href="/fabrics" className="hover:text-zinc-900 dark:hover:text-white">Fabrics</Link>
        <span>/</span>
        <span className="text-zinc-900 dark:text-white">{fabricId}</span>
      </div>

      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-zinc-900 dark:text-white">{fabricId}</h1>
        <div className="flex gap-2">
          <button
            onClick={handleSyncSwitches}
            disabled={syncing}
            className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50"
          >
            {syncing ? 'Syncing...' : 'Sync Switches'}
          </button>
          <button
            onClick={handleSyncAllPorts}
            disabled={syncing}
            className="px-4 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700 disabled:opacity-50"
          >
            Sync All Ports
          </button>
        </div>
      </div>

      {error && (
        <div className="p-4 bg-red-100 dark:bg-red-900/20 text-red-700 dark:text-red-400 rounded-lg">
          {error}
        </div>
      )}

      {/* Node Info Modal */}
      {selectedNodeInfo && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white dark:bg-zinc-900 p-6 rounded-lg w-full max-w-md">
            <h2 className="text-xl font-semibold mb-4 text-zinc-900 dark:text-white">
              Compute Node Details
            </h2>
            <div className="space-y-3">
              <div>
                <label className="block text-sm font-medium text-zinc-500 dark:text-zinc-400">Name</label>
                <div className="text-zinc-900 dark:text-white">{selectedNodeInfo.compute_node?.name || 'Unknown'}</div>
              </div>
              <div>
                <label className="block text-sm font-medium text-zinc-500 dark:text-zinc-400">Hostname</label>
                <div className="text-zinc-900 dark:text-white">{selectedNodeInfo.compute_node?.hostname || 'N/A'}</div>
              </div>
              <div>
                <label className="block text-sm font-medium text-zinc-500 dark:text-zinc-400">IP Address</label>
                <div className="text-zinc-900 dark:text-white">{selectedNodeInfo.compute_node?.ip_address || 'N/A'}</div>
              </div>
              <div>
                <label className="block text-sm font-medium text-zinc-500 dark:text-zinc-400">NIC Name</label>
                <div className="text-zinc-900 dark:text-white">{selectedNodeInfo.nic_name}</div>
              </div>
              <div>
                <label className="block text-sm font-medium text-zinc-500 dark:text-zinc-400">Port</label>
                <div className="text-zinc-900 dark:text-white">{selectedNodeInfo.switch_port?.name || 'Unknown'}</div>
              </div>
            </div>
            <div className="flex gap-2 justify-end mt-6">
              <Link
                href="/compute-nodes"
                className="px-4 py-2 text-blue-600 hover:text-blue-700"
              >
                Go to Compute Nodes
              </Link>
              <button
                onClick={() => setSelectedNodeInfo(null)}
                className="px-4 py-2 bg-zinc-200 dark:bg-zinc-700 text-zinc-900 dark:text-white rounded-lg hover:bg-zinc-300 dark:hover:bg-zinc-600"
              >
                Close
              </button>
            </div>
          </div>
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Switches List */}
        <div className="space-y-4">
          <h2 className="text-lg font-semibold text-zinc-900 dark:text-white">Switches</h2>
          {loading ? (
            <div className="text-zinc-600 dark:text-zinc-400">Loading...</div>
          ) : switches.length === 0 ? (
            <div className="text-zinc-600 dark:text-zinc-400">No switches found</div>
          ) : (
            <div className="space-y-2">
              {switches.map((sw) => (
                <button
                  key={sw.id}
                  onClick={() => loadPorts(sw)}
                  className={`w-full text-left p-4 rounded-lg border transition-colors ${
                    selectedSwitch?.id === sw.id
                      ? 'bg-blue-50 dark:bg-blue-900/20 border-blue-500'
                      : 'bg-white dark:bg-zinc-900 border-zinc-200 dark:border-zinc-800 hover:border-blue-500'
                  }`}
                >
                  <div className="font-medium text-zinc-900 dark:text-white">{sw.name}</div>
                  <div className="text-sm text-zinc-600 dark:text-zinc-400">
                    {sw.model} • {sw.ip_address}
                  </div>
                  <div className="text-xs text-zinc-500 dark:text-zinc-500 mt-1">
                    Serial: {sw.serial_number}
                  </div>
                </button>
              ))}
            </div>
          )}
        </div>

        {/* Ports List */}
        <div className="space-y-4">
          <h2 className="text-lg font-semibold text-zinc-900 dark:text-white">
            Ports {selectedSwitch && `(${selectedSwitch.name})`}
          </h2>
          {!selectedSwitch ? (
            <div className="text-zinc-600 dark:text-zinc-400">Select a switch to view ports</div>
          ) : portsLoading ? (
            <div className="text-zinc-600 dark:text-zinc-400">Loading ports...</div>
          ) : ports.length === 0 ? (
            <div className="text-zinc-600 dark:text-zinc-400">No ports found</div>
          ) : (
            <div className="max-h-[600px] overflow-y-auto space-y-1">
              {ports.map((port) => {
                const mapping = portMappings.get(port.id);
                return (
                  <div
                    key={port.id}
                    className={`p-3 rounded border ${
                      mapping
                        ? 'bg-blue-50 dark:bg-blue-900/20 border-blue-300 dark:border-blue-700'
                        : 'bg-white dark:bg-zinc-900 border-zinc-200 dark:border-zinc-800'
                    }`}
                  >
                    <div className="flex items-center justify-between">
                      <span className="font-mono text-sm text-zinc-900 dark:text-white">{port.name}</span>
                      <span className={`text-xs px-2 py-1 rounded ${
                        port.admin_state === 'true'
                          ? 'bg-green-100 dark:bg-green-900/20 text-green-700 dark:text-green-400'
                          : 'bg-zinc-100 dark:bg-zinc-800 text-zinc-600 dark:text-zinc-400'
                      }`}>
                        {port.admin_state === 'true' ? 'Up' : 'Down'}
                      </span>
                    </div>
                    <div className="text-xs text-zinc-500 mt-1">Speed: {port.speed}</div>
                    {mapping && (
                      <button
                        onClick={() => setSelectedNodeInfo(mapping)}
                        className="mt-2 text-xs text-blue-600 dark:text-blue-400 hover:underline flex items-center gap-1"
                      >
                        <span className="inline-block w-2 h-2 bg-blue-500 rounded-full"></span>
                        Mapped: {mapping.compute_node?.name || 'Unknown'} ({mapping.nic_name})
                      </button>
                    )}
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
