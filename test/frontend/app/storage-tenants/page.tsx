'use client';

import { useState, useEffect } from 'react';
import { storageTenantsAPI, StorageTenant } from '@/lib/api';

export default function StorageTenantsPage() {
  const [tenants, setTenants] = useState<StorageTenant[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [editingTenant, setEditingTenant] = useState<StorageTenant | null>(null);

  // Form state
  const [formData, setFormData] = useState({
    key: '',
    description: '',
    storage_network_name: '',
    storage_dst_group_name: '',
    storage_contract_name: '',
  });

  useEffect(() => {
    loadTenants();
  }, []);

  const loadTenants = async () => {
    try {
      setLoading(true);
      const data = await storageTenantsAPI.list();
      setTenants(data || []);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load tenants');
    } finally {
      setLoading(false);
    }
  };

  const handleCreate = async () => {
    try {
      await storageTenantsAPI.create(formData);
      setShowCreateModal(false);
      resetForm();
      loadTenants();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create tenant');
    }
  };

  const handleUpdate = async () => {
    if (!editingTenant) return;
    try {
      await storageTenantsAPI.update(editingTenant.key, formData);
      setEditingTenant(null);
      resetForm();
      loadTenants();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update tenant');
    }
  };

  const handleDelete = async (key: string) => {
    if (!confirm(`Delete storage tenant "${key}"?`)) return;
    try {
      await storageTenantsAPI.delete(key);
      loadTenants();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete tenant');
    }
  };

  const resetForm = () => {
    setFormData({
      key: '',
      description: '',
      storage_network_name: '',
      storage_dst_group_name: '',
      storage_contract_name: '',
    });
  };

  const openEditModal = (tenant: StorageTenant) => {
    setFormData({
      key: tenant.key,
      description: tenant.description,
      storage_network_name: tenant.storage_network_name,
      storage_dst_group_name: tenant.storage_dst_group_name,
      storage_contract_name: tenant.storage_contract_name,
    });
    setEditingTenant(tenant);
  };

  const openCreateModal = () => {
    resetForm();
    setShowCreateModal(true);
  };

  if (loading) {
    return (
      <div className="p-8">
        <h1 className="text-2xl font-bold mb-4">Storage Tenants</h1>
        <p>Loading...</p>
      </div>
    );
  }

  return (
    <div className="p-8">
      <div className="flex justify-between items-center mb-6">
        <h1 className="text-2xl font-bold">Storage Tenants</h1>
        <button
          onClick={openCreateModal}
          className="bg-blue-600 text-white px-4 py-2 rounded hover:bg-blue-700"
        >
          Add Tenant
        </button>
      </div>

      {error && (
        <div className="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded mb-4">
          {error}
          <button onClick={() => setError(null)} className="float-right font-bold">×</button>
        </div>
      )}

      <div className="bg-white shadow rounded-lg overflow-hidden">
        <table className="min-w-full divide-y divide-gray-200">
          <thead className="bg-gray-50">
            <tr>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Key</th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Description</th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Storage Network</th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Dst Group</th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Contract</th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Actions</th>
            </tr>
          </thead>
          <tbody className="bg-white divide-y divide-gray-200">
            {tenants.length === 0 ? (
              <tr>
                <td colSpan={6} className="px-6 py-4 text-center text-gray-500">
                  No storage tenants configured
                </td>
              </tr>
            ) : (
              tenants.map((tenant) => (
                <tr key={tenant.id}>
                  <td className="px-6 py-4 whitespace-nowrap font-medium">{tenant.key}</td>
                  <td className="px-6 py-4 whitespace-nowrap text-gray-500">{tenant.description || '-'}</td>
                  <td className="px-6 py-4 whitespace-nowrap font-mono text-sm">{tenant.storage_network_name}</td>
                  <td className="px-6 py-4 whitespace-nowrap font-mono text-sm">{tenant.storage_dst_group_name}</td>
                  <td className="px-6 py-4 whitespace-nowrap font-mono text-sm">{tenant.storage_contract_name}</td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm">
                    <button
                      onClick={() => openEditModal(tenant)}
                      className="text-blue-600 hover:text-blue-900 mr-4"
                    >
                      Edit
                    </button>
                    <button
                      onClick={() => handleDelete(tenant.key)}
                      className="text-red-600 hover:text-red-900"
                    >
                      Delete
                    </button>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Create/Edit Modal */}
      {(showCreateModal || editingTenant) && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-white rounded-lg p-6 w-full max-w-md">
            <h2 className="text-xl font-bold mb-4">
              {editingTenant ? 'Edit Storage Tenant' : 'Create Storage Tenant'}
            </h2>
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Tenant Key *
                </label>
                <input
                  type="text"
                  value={formData.key}
                  onChange={(e) => setFormData({ ...formData, key: e.target.value })}
                  className="w-full border rounded px-3 py-2"
                  placeholder="e.g., tenant1"
                  disabled={!!editingTenant}
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Description
                </label>
                <input
                  type="text"
                  value={formData.description}
                  onChange={(e) => setFormData({ ...formData, description: e.target.value })}
                  className="w-full border rounded px-3 py-2"
                  placeholder="Optional description"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Storage Network Name *
                </label>
                <input
                  type="text"
                  value={formData.storage_network_name}
                  onChange={(e) => setFormData({ ...formData, storage_network_name: e.target.value })}
                  className="w-full border rounded px-3 py-2"
                  placeholder="e.g., tenant1_storage_net"
                />
                <p className="text-xs text-gray-500 mt-1">NDFC network name for tenant storage</p>
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Storage Destination Group *
                </label>
                <input
                  type="text"
                  value={formData.storage_dst_group_name}
                  onChange={(e) => setFormData({ ...formData, storage_dst_group_name: e.target.value })}
                  className="w-full border rounded px-3 py-2"
                  placeholder="e.g., tenant1_storage_services"
                />
                <p className="text-xs text-gray-500 mt-1">Security group for tenant storage services</p>
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Storage Contract Name *
                </label>
                <input
                  type="text"
                  value={formData.storage_contract_name}
                  onChange={(e) => setFormData({ ...formData, storage_contract_name: e.target.value })}
                  className="w-full border rounded px-3 py-2"
                  placeholder="e.g., tenant1_storage_access"
                />
                <p className="text-xs text-gray-500 mt-1">Contract for node to tenant storage access</p>
              </div>
            </div>
            <div className="flex justify-end gap-2 mt-6">
              <button
                onClick={() => {
                  setShowCreateModal(false);
                  setEditingTenant(null);
                  resetForm();
                }}
                className="px-4 py-2 border rounded hover:bg-gray-50"
              >
                Cancel
              </button>
              <button
                onClick={editingTenant ? handleUpdate : handleCreate}
                className="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700"
                disabled={!formData.key || !formData.storage_network_name || !formData.storage_dst_group_name || !formData.storage_contract_name}
              >
                {editingTenant ? 'Update' : 'Create'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
