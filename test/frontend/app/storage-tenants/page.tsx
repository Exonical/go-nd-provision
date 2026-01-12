'use client';

import { useState, useEffect } from 'react';
import { storageTenantsAPI, StorageTenant } from '@/lib/api';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';

export default function StorageTenantsPage() {
  const [tenants, setTenants] = useState<StorageTenant[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [editingTenant, setEditingTenant] = useState<StorageTenant | null>(null);

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
      <div className="space-y-6">
        <h1 className="text-2xl font-bold text-foreground">Storage Tenants</h1>
        <p className="text-muted-foreground">Loading...</p>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-2xl font-bold text-foreground">Storage Tenants</h1>
          <p className="text-muted-foreground">Manage storage tenant configurations for NDFC provisioning</p>
        </div>
        <Button onClick={openCreateModal}>Add Tenant</Button>
      </div>

      {error && (
        <div className="bg-destructive/20 border border-destructive text-destructive px-4 py-3 rounded-md">
          {error}
          <button onClick={() => setError(null)} className="float-right font-bold">×</button>
        </div>
      )}

      <Card>
        <CardHeader>
          <CardTitle>Tenants</CardTitle>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Key</TableHead>
                <TableHead>Description</TableHead>
                <TableHead>Storage Network</TableHead>
                <TableHead>Dst Group</TableHead>
                <TableHead>Contract</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {tenants.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={6} className="text-center text-muted-foreground">
                    No storage tenants configured
                  </TableCell>
                </TableRow>
              ) : (
                tenants.map((tenant) => (
                  <TableRow key={tenant.id}>
                    <TableCell className="font-medium">{tenant.key}</TableCell>
                    <TableCell className="text-muted-foreground">{tenant.description || '-'}</TableCell>
                    <TableCell className="font-mono text-sm">{tenant.storage_network_name}</TableCell>
                    <TableCell className="font-mono text-sm">{tenant.storage_dst_group_name}</TableCell>
                    <TableCell className="font-mono text-sm">{tenant.storage_contract_name}</TableCell>
                    <TableCell className="text-right">
                      <Button variant="ghost" size="sm" onClick={() => openEditModal(tenant)}>
                        Edit
                      </Button>
                      <Button variant="ghost" size="sm" onClick={() => handleDelete(tenant.key)} className="text-destructive hover:text-destructive">
                        Delete
                      </Button>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      {/* Create/Edit Modal */}
      {(showCreateModal || editingTenant) && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <Card className="w-full max-w-md">
            <CardHeader>
              <CardTitle>{editingTenant ? 'Edit Storage Tenant' : 'Create Storage Tenant'}</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-muted-foreground mb-1">Tenant Key *</label>
                <Input
                  value={formData.key}
                  onChange={(e) => setFormData({ ...formData, key: e.target.value })}
                  placeholder="e.g., tenant1"
                  disabled={!!editingTenant}
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-muted-foreground mb-1">Description</label>
                <Input
                  value={formData.description}
                  onChange={(e) => setFormData({ ...formData, description: e.target.value })}
                  placeholder="Optional description"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-muted-foreground mb-1">Storage Network Name *</label>
                <Input
                  value={formData.storage_network_name}
                  onChange={(e) => setFormData({ ...formData, storage_network_name: e.target.value })}
                  placeholder="e.g., tenant1_storage_net"
                />
                <p className="text-xs text-muted-foreground mt-1">NDFC network name for tenant storage</p>
              </div>
              <div>
                <label className="block text-sm font-medium text-muted-foreground mb-1">Storage Destination Group *</label>
                <Input
                  value={formData.storage_dst_group_name}
                  onChange={(e) => setFormData({ ...formData, storage_dst_group_name: e.target.value })}
                  placeholder="e.g., tenant1_storage_services"
                />
                <p className="text-xs text-muted-foreground mt-1">Security group for tenant storage services</p>
              </div>
              <div>
                <label className="block text-sm font-medium text-muted-foreground mb-1">Storage Contract Name *</label>
                <Input
                  value={formData.storage_contract_name}
                  onChange={(e) => setFormData({ ...formData, storage_contract_name: e.target.value })}
                  placeholder="e.g., tenant1_storage_access"
                />
                <p className="text-xs text-muted-foreground mt-1">Contract for node to tenant storage access</p>
              </div>
              <div className="flex justify-end gap-2 pt-4">
                <Button
                  variant="outline"
                  onClick={() => {
                    setShowCreateModal(false);
                    setEditingTenant(null);
                    resetForm();
                  }}
                >
                  Cancel
                </Button>
                <Button
                  onClick={editingTenant ? handleUpdate : handleCreate}
                  disabled={!formData.key || !formData.storage_network_name || !formData.storage_dst_group_name || !formData.storage_contract_name}
                >
                  {editingTenant ? 'Update' : 'Create'}
                </Button>
              </div>
            </CardContent>
          </Card>
        </div>
      )}
    </div>
  );
}
