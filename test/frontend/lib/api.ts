const API_BASE = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

async function fetchAPI<T>(endpoint: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${endpoint}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...options?.headers,
    },
  });
  if (!res.ok) {
    const error = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(error.error || res.statusText);
  }
  return res.json();
}

// Types
export interface Fabric {
  id: string;
  name: string;
  type: string;
  created_at: string;
  updated_at: string;
}

export interface Switch {
  id: string;
  name: string;
  serial_number: string;
  model: string;
  ip_address: string;
  fabric_id: string;
  created_at: string;
  updated_at: string;
}

export interface SwitchPort {
  id: string;
  name: string;
  port_number: string;
  description: string;
  admin_state: string;
  speed: string;
  is_present: boolean;
  switch_id: string;
  created_at: string;
  updated_at: string;
}

export interface ComputeNode {
  id: string;
  name: string;
  hostname: string;
  ip_address: string;
  mac_address: string;
  description: string;
  port_mappings?: PortMapping[];
  created_at: string;
  updated_at: string;
}

export interface PortMapping {
  id: string;
  compute_node_id: string;
  switch_port_id: string;
  switch_port?: SwitchPort & { switch?: Switch };
  nic_name: string;
  vlan: number;
  created_at: string;
  updated_at: string;
}

// Fabrics API
export const fabricsAPI = {
  list: () => fetchAPI<Fabric[]>('/api/v1/fabrics'),
  get: (id: string) => fetchAPI<Fabric>(`/api/v1/fabrics/${id}`),
  sync: () => fetchAPI<{ count: number; message: string }>('/api/v1/fabrics/sync', { method: 'POST' }),
};

// Switches API
export const switchesAPI = {
  list: (fabricId: string) => fetchAPI<Switch[]>(`/api/v1/fabrics/${fabricId}/switches`),
  get: (fabricId: string, switchId: string) => fetchAPI<Switch>(`/api/v1/fabrics/${fabricId}/switches/${switchId}`),
  sync: (fabricId: string) => fetchAPI<{ count: number; message: string }>(`/api/v1/fabrics/${fabricId}/switches/sync`, { method: 'POST' }),
};

// Ports API
export const portsAPI = {
  list: (fabricId: string, switchId: string) => fetchAPI<SwitchPort[]>(`/api/v1/fabrics/${fabricId}/switches/${switchId}/ports`),
  syncAll: (fabricId: string) => fetchAPI<{ ports: number; switches: number }>(`/api/v1/fabrics/${fabricId}/ports/sync`, { method: 'POST' }),
};

// Compute Nodes API
export const computeNodesAPI = {
  list: () => fetchAPI<ComputeNode[]>('/api/v1/compute-nodes'),
  get: (id: string) => fetchAPI<ComputeNode>(`/api/v1/compute-nodes/${id}`),
  create: (data: { name: string; hostname?: string; ip_address?: string }) =>
    fetchAPI<ComputeNode>('/api/v1/compute-nodes', { method: 'POST', body: JSON.stringify(data) }),
  update: (id: string, data: Partial<ComputeNode>) =>
    fetchAPI<ComputeNode>(`/api/v1/compute-nodes/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  delete: (id: string) => fetchAPI<{ message: string }>(`/api/v1/compute-nodes/${id}`, { method: 'DELETE' }),
  getPortMappings: (id: string) => fetchAPI<PortMapping[]>(`/api/v1/compute-nodes/${id}/port-mappings`),
  addPortMapping: (id: string, data: { switch: string; port_name: string; nic_name: string }) =>
    fetchAPI<PortMapping>(`/api/v1/compute-nodes/${id}/port-mappings`, { method: 'POST', body: JSON.stringify(data) }),
  updatePortMapping: (nodeId: string, mappingId: string, data: { nic_name: string }) =>
    fetchAPI<PortMapping>(`/api/v1/compute-nodes/${nodeId}/port-mappings/${mappingId}`, { method: 'PUT', body: JSON.stringify(data) }),
  deletePortMapping: (nodeId: string, mappingId: string) =>
    fetchAPI<{ message: string }>(`/api/v1/compute-nodes/${nodeId}/port-mappings/${mappingId}`, { method: 'DELETE' }),
};
