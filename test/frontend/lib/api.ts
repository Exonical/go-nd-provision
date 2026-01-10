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
  interface_id?: string;
  switch_port?: SwitchPort & { switch?: Switch };
  compute_node?: ComputeNode;
  nic_name: string;
  vlan: number;
  created_at: string;
  updated_at: string;
}

export interface ComputeNodeInterface {
  id: string;
  compute_node_id: string;
  role: 'compute' | 'storage';
  hostname?: string;
  ip_address?: string;
  mac_address?: string;
  port_mappings?: PortMapping[];
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
  getMappingsBySwitch: (switchId: string) => fetchAPI<PortMapping[]>(`/api/v1/switches/${switchId}/compute-nodes`),
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
  updatePortMapping: (nodeId: string, mappingId: string, data: { nic_name?: string; switch?: string; port_name?: string }) =>
    fetchAPI<PortMapping>(`/api/v1/compute-nodes/${nodeId}/port-mappings/${mappingId}`, { method: 'PUT', body: JSON.stringify(data) }),
  deletePortMapping: (nodeId: string, mappingId: string) =>
    fetchAPI<{ message: string }>(`/api/v1/compute-nodes/${nodeId}/port-mappings/${mappingId}`, { method: 'DELETE' }),
  // Interface management
  getInterfaces: (id: string) => fetchAPI<ComputeNodeInterface[]>(`/api/v1/compute-nodes/${id}/interfaces`),
  createInterface: (id: string, data: { role: 'compute' | 'storage'; hostname?: string; ip_address?: string; mac_address?: string }) =>
    fetchAPI<ComputeNodeInterface>(`/api/v1/compute-nodes/${id}/interfaces`, { method: 'POST', body: JSON.stringify(data) }),
  updateInterface: (nodeId: string, interfaceId: string, data: { hostname?: string; ip_address?: string; mac_address?: string }) =>
    fetchAPI<ComputeNodeInterface>(`/api/v1/compute-nodes/${nodeId}/interfaces/${interfaceId}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteInterface: (nodeId: string, interfaceId: string) =>
    fetchAPI<{ message: string }>(`/api/v1/compute-nodes/${nodeId}/interfaces/${interfaceId}`, { method: 'DELETE' }),
  assignMappingToInterface: (nodeId: string, mappingId: string, interfaceId: string | null) =>
    fetchAPI<PortMapping>(`/api/v1/compute-nodes/${nodeId}/port-mappings/${mappingId}/interface`, { method: 'PUT', body: JSON.stringify({ interface_id: interfaceId }) }),
};

// Storage Tenant types
export interface StorageTenant {
  id: string;
  key: string;
  description: string;
  storage_network_name: string;
  storage_dst_group_name: string;
  storage_contract_name: string;
  created_at: string;
  updated_at: string;
}

// Storage Tenants API
export const storageTenantsAPI = {
  list: () => fetchAPI<StorageTenant[]>('/api/v1/storage-tenants'),
  get: (key: string) => fetchAPI<StorageTenant>(`/api/v1/storage-tenants/${key}`),
  create: (data: Omit<StorageTenant, 'id' | 'created_at' | 'updated_at'>) =>
    fetchAPI<StorageTenant>('/api/v1/storage-tenants', { method: 'POST', body: JSON.stringify(data) }),
  update: (key: string, data: Omit<StorageTenant, 'id' | 'created_at' | 'updated_at'>) =>
    fetchAPI<StorageTenant>(`/api/v1/storage-tenants/${key}`, { method: 'PUT', body: JSON.stringify(data) }),
  delete: (key: string) => fetchAPI<{ message: string }>(`/api/v1/storage-tenants/${key}`, { method: 'DELETE' }),
};

// Job types
export interface Job {
  id: string;
  slurm_job_id: string;
  name: string;
  tenant_key?: string;
  status: string;
  error_message?: string;
  fabric_name: string;
  vrf_name: string;
  contract_name: string;
  submitted_at: string;
  provisioned_at?: string;
  completed_at?: string;
  expires_at?: string;
  compute_nodes?: JobComputeNode[];
  security_group_id?: string;
}

export interface JobComputeNode {
  id: string;
  job_id: string;
  compute_node_id: string;
  compute_node?: ComputeNode;
}

// Jobs API
export const jobsAPI = {
  list: (status?: string) => fetchAPI<Job[]>(`/api/v1/jobs${status ? `?status=${status}` : ''}`),
  get: (slurmJobId: string) => fetchAPI<Job>(`/api/v1/jobs/${slurmJobId}`),
  submit: (data: { slurm_job_id: string; name?: string; tenant?: string; compute_nodes: string[] }) =>
    fetchAPI<Job>('/api/v1/jobs', { method: 'POST', body: JSON.stringify(data) }),
  complete: (slurmJobId: string) =>
    fetchAPI<Job>(`/api/v1/jobs/${slurmJobId}/complete`, { method: 'POST' }),
};

// Bulk port mapping types
export interface BulkPortAssignment {
  switch_port_id: string;
  node_id?: string | null;
  interface_id?: string | null;
}

export interface BulkAssignmentResult {
  switch_port_id: string;
  success: boolean;
  action?: string;
  mapping_id?: string;
  error?: string;
}

// Bulk port mappings API
export const portMappingsAPI = {
  bulkAssign: (assignments: BulkPortAssignment[]) =>
    fetchAPI<{ results: BulkAssignmentResult[]; total: number }>('/api/v1/port-mappings/bulk', {
      method: 'POST',
      body: JSON.stringify({ assignments }),
    }),
};
