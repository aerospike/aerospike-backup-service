import React, { useState } from 'react';
import {
  Database, Clock, Server, HardDrive, Shield, Key, Play, CheckCircle,
  Settings, Plus, Trash2, Copy
} from 'lucide-react';
import { Button } from '@/components/ui/Button';
import { Input, Select } from '@/components/ui/Inputs';
import { toYaml } from '@/utils/yaml';
import { AppConfig } from '@/types';

// --- Local Components ---

const Checkbox = ({ label, checked, onChange }: { label: string, checked: boolean, onChange: (v: boolean) => void }) => (
  <div className="flex items-center mb-3">
    <input
      type="checkbox"
      checked={checked}
      onChange={(e) => onChange(e.target.checked)}
      className="w-4 h-4 text-red-600 bg-gray-900 border-gray-700 rounded focus:ring-red-500 focus:ring-2 focus:ring-offset-gray-900"
    />
    <label className="ml-2 text-sm font-medium text-gray-300 select-none cursor-pointer" onClick={() => onChange(!checked)}>{label}</label>
  </div>
);

const SectionHeader = ({ title, icon: Icon }: { title: string, icon?: any }) => (
    <div className="flex items-center gap-2 mb-4 border-b border-gray-800 pb-2 mt-6 first:mt-0">
        {Icon && <Icon size={16} className="text-red-500" />}
        <h3 className="text-xs font-bold text-gray-400 uppercase tracking-wider">{title}</h3>
    </div>
);

const Card = ({ title, sub, active, onClick, onDelete }: { title: string, sub?: string, active: boolean, onClick: () => void, onDelete?: () => void }) => (
  <div
    onClick={onClick}
    className={`p-3 rounded border cursor-pointer transition-all mb-2 flex justify-between items-center group ${
      active
        ? 'bg-gray-800 border-red-500 shadow-sm'
        : 'bg-gray-900/50 border-gray-800 hover:border-gray-700 hover:bg-gray-900'
    }`}
  >
    <div className="overflow-hidden">
      <div className="font-bold text-sm text-gray-200 truncate">{title}</div>
      {sub && <div className="text-xs text-gray-500 truncate">{sub}</div>}
    </div>
    {onDelete && (
      <button
        onClick={(e) => { e.stopPropagation(); onDelete(); }}
        className="text-gray-600 hover:text-red-400 opacity-0 group-hover:opacity-100 transition-opacity p-1"
      >
        <Trash2 size={14} />
      </button>
    )}
  </div>
);

import { api } from '@/api/client';

// --- Main Editor ---

interface ConfigEditorProps {
  config: AppConfig;
  setConfig: React.Dispatch<React.SetStateAction<AppConfig>>;
}

type SectionId = 'routines' | 'clusters' | 'storage' | 'policies' | 'secrets' | 'service' | 'yaml';

export default function ConfigEditor({ config, setConfig }: ConfigEditorProps) {
  const [section, setSection] = useState<SectionId>('routines');
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [isSaving, setIsSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);

  const handleApply = async () => {
    setIsSaving(true);
    setSaveError(null);
    try {
      await api.applyConfig(config);
      // Maybe show a success toast here
    } catch (e: any) {
      setSaveError(e.message || 'Failed to save.');
    } finally {
      setIsSaving(false);
    }
  };

  // --- Helpers ---

  const generateId = (prefix: string) => `${prefix}-${Math.random().toString(36).substr(2, 6)}`;

  // Generic updater for top-level maps (clusters, policies, etc.)
  const updateItem = (sectionKey: keyof AppConfig, id: string, field: string, value: any) => {
    setConfig(prev => ({
      ...prev,
      [sectionKey]: {
        ...(prev[sectionKey] as any),
        [id]: { ...(prev[sectionKey] as any)[id], [field]: value }
      }
    }));
  };

  // Deep updater for nested properties (e.g. cluster.credentials.user)
  const updateNested = (sectionKey: keyof AppConfig, id: string, parent: string, field: string, value: any) => {
    setConfig(prev => {
        const item = (prev[sectionKey] as any)[id];
        return {
            ...prev,
            [sectionKey]: {
                ...(prev[sectionKey] as any),
                [id]: {
                    ...item,
                    [parent]: { ...item[parent], [field]: value }
                }
            }
        };
    });
  };

  const deleteItem = (sectionKey: keyof AppConfig, id: string) => {
    if (!window.confirm(`Delete ${id}?`)) return;
    setConfig(prev => {
      const next = { ...(prev[sectionKey] as any) };
      delete next[id];
      return { ...prev, [sectionKey]: next };
    });
    if (selectedId === id) setSelectedId(null);
  };

  const renameItem = (sectionKey: keyof AppConfig, oldId: string, newId: string) => {
    if (oldId === newId || !newId) return;
    setConfig(prev => {
        const collection = { ...(prev[sectionKey] as any) };
        collection[newId] = collection[oldId];
        delete collection[oldId];
        return { ...prev, [sectionKey]: collection };
    });
    setSelectedId(newId);
  }

  // --- Renderers ---

  const renderRoutines = () => {
    const items = config['backup-routines'];
    return (
      <div className="flex h-full">
        <div className="w-1/3 border-r border-gray-800 p-4 flex flex-col">
          <Button
            className="mb-4 w-full justify-center"
            icon={Plus}
            onClick={() => {
              const id = generateId('routine');
              setConfig(p => ({...p, 'backup-routines': {...p['backup-routines'], [id]: { "source-cluster": "", storage: "", "interval-cron": "@daily", namespaces: [] }}}));
              setSelectedId(id);
            }}
          >
            New Routine
          </Button>
          <div className="overflow-y-auto flex-1 custom-scroll">
            {Object.entries(items).map(([k, v]) => (
              <Card
                key={k}
                title={k}
                sub={v['interval-cron']}
                active={selectedId === k}
                onClick={() => setSelectedId(k)}
                onDelete={() => deleteItem('backup-routines', k)}
              />
            ))}
          </div>
        </div>
        <div className="w-2/3 p-6 overflow-y-auto custom-scroll">
          {selectedId && items[selectedId] ? (
            <div className="animate-in fade-in slide-in-from-right-4 duration-200">
               <div className="mb-6">
                  <Input label="Routine Name" value={selectedId} onChange={(e) => renameItem('backup-routines', selectedId, e.target.value)} />
               </div>

               <SectionHeader title="Core Bindings" icon={Clock} />
               <div className="grid grid-cols-2 gap-4">
                  <Select
                    label="Source Cluster"
                    value={items[selectedId]['source-cluster']}
                    options={[{label: "Select...", value: ""}, ...Object.keys(config['aerospike-clusters']).map(k => ({ label: k, value: k }))]}
                    onChange={(e) => updateItem('backup-routines', selectedId, 'source-cluster', e.target.value)}
                  />
                  <Select
                    label="Storage"
                    value={items[selectedId]['storage']}
                    options={[{label: "Select...", value: ""}, ...Object.keys(config['storage']).map(k => ({ label: k, value: k }))]}
                    onChange={(e) => updateItem('backup-routines', selectedId, 'storage', e.target.value)}
                  />
                  <Select
                    label="Policy (Optional)"
                    value={items[selectedId]['backup-policy'] || ''}
                    options={[{label: "None (Default)", value: ""}, ...Object.keys(config['backup-policies']).map(k => ({ label: k, value: k }))]}
                    onChange={(e) => updateItem('backup-routines', selectedId, 'backup-policy', e.target.value)}
                  />
                  <Select
                    label="Secret Agent (Optional)"
                    value={(items[selectedId] as any)['secret-agent'] || ''}
                    options={[{label: "None", value: ""}, ...Object.keys(config['secret-agents']).map(k => ({ label: k, value: k }))]}
                    onChange={(e) => updateItem('backup-routines', selectedId, 'secret-agent', e.target.value)}
                  />
               </div>

               <SectionHeader title="Scheduling" />
               <div className="grid grid-cols-2 gap-4">
                  <Input label="Full Backup Cron" value={items[selectedId]['interval-cron']} onChange={e => updateItem('backup-routines', selectedId, 'interval-cron', e.target.value)} placeholder="@daily" />
                  <Input label="Incremental Cron" value={items[selectedId]['incr-interval-cron']} onChange={e => updateItem('backup-routines', selectedId, 'incr-interval-cron', e.target.value)} placeholder="Optional" />
               </div>

               <SectionHeader title="Scope" />
               <Input
                 label="Namespaces (Comma separated)"
                 value={items[selectedId].namespaces?.join(', ') || ''}
                 onChange={e => updateItem('backup-routines', selectedId, 'namespaces', e.target.value.split(',').map(s => s.trim()).filter(Boolean))}
                 placeholder="All namespaces if empty"
               />
               <Input
                 label="Set List (Comma separated)"
                 value={items[selectedId]["set-list"]?.join(', ') || ''}
                 onChange={e => updateItem('backup-routines', selectedId, 'set-list', e.target.value.split(',').map(s => s.trim()).filter(Boolean))}
                 placeholder="Optional"
               />
               <Checkbox
                 label="Disable Routine"
                 checked={items[selectedId].disabled || false}
                 onChange={v => updateItem('backup-routines', selectedId, 'disabled', v)}
               />
            </div>
          ) : (
            <div className="flex flex-col items-center justify-center h-full text-gray-500">
                <Clock size={48} className="mb-4 opacity-20" />
                <p>Select a routine to configure</p>
            </div>
          )}
        </div>
      </div>
    );
  };

  const renderClusters = () => {
    const items = config['aerospike-clusters'];
    return (
      <div className="flex h-full">
        <div className="w-1/3 border-r border-gray-800 p-4 flex flex-col">
          <Button
            className="mb-4 w-full justify-center"
            icon={Plus}
            onClick={() => {
              const id = generateId('cluster');
              setConfig(p => ({...p, 'aerospike-clusters': {...p['aerospike-clusters'], [id]: { "seed-nodes": [{ "host-name": "localhost", "port": 3000 }], "credentials": { user: "" } }}}));
              setSelectedId(id);
            }}
          >
            New Cluster
          </Button>
          <div className="overflow-y-auto flex-1 custom-scroll">
            {Object.entries(items).map(([k, v]) => (
              <Card
                key={k}
                title={k}
                sub={`${v['seed-nodes']?.length || 0} nodes`}
                active={selectedId === k}
                onClick={() => setSelectedId(k)}
                onDelete={() => deleteItem('aerospike-clusters', k)}
              />
            ))}
          </div>
        </div>
        <div className="w-2/3 p-6 overflow-y-auto custom-scroll">
          {selectedId && items[selectedId] ? (
            <div className="animate-in fade-in slide-in-from-right-4 duration-200">
               <div className="mb-6">
                  <Input label="Cluster Name" value={selectedId} onChange={(e) => renameItem('aerospike-clusters', selectedId, e.target.value)} />
               </div>

               <SectionHeader title="Seed Nodes" icon={Server} />
               <div className="space-y-2 mb-6">
                 {items[selectedId]['seed-nodes'].map((node, idx) => (
                   <div key={idx} className="flex gap-2">
                      <Input className="flex-1" placeholder="Hostname" value={node['host-name']} onChange={e => {
                        const nodes = [...items[selectedId]['seed-nodes']];
                        nodes[idx]['host-name'] = e.target.value;
                        updateItem('aerospike-clusters', selectedId, 'seed-nodes', nodes);
                      }} />
                      <Input className="w-24" type="number" placeholder="Port" value={node.port} onChange={e => {
                        const nodes = [...items[selectedId]['seed-nodes']];
                        nodes[idx].port = Number(e.target.value);
                        updateItem('aerospike-clusters', selectedId, 'seed-nodes', nodes);
                      }} />
                      <Button variant="ghost" className="px-2 text-gray-500 hover:text-red-500" onClick={() => {
                         const nodes = items[selectedId]['seed-nodes'].filter((_, i) => i !== idx);
                         updateItem('aerospike-clusters', selectedId, 'seed-nodes', nodes);
                      }}><Trash2 size={16}/></Button>
                   </div>
                 ))}
                 <Button variant="secondary" className="w-full justify-center py-1 text-xs" onClick={() => {
                    const nodes = [...items[selectedId]['seed-nodes'], { "host-name": "localhost", "port": 3000 }];
                    updateItem('aerospike-clusters', selectedId, 'seed-nodes', nodes);
                 }}>+ Add Node</Button>
               </div>

               <SectionHeader title="Authentication" icon={Key} />
               <Input label="User" value={items[selectedId].credentials?.user} onChange={e => updateNested('aerospike-clusters', selectedId, 'credentials', 'user', e.target.value)} />
               <Input label="Password" type="password" value={items[selectedId].credentials?.password} onChange={e => updateNested('aerospike-clusters', selectedId, 'credentials', 'password', e.target.value)} />
               <Select
                 label="Auth Mode"
                 value={items[selectedId].credentials?.["auth-mode"] || "INTERNAL"}
                 options={[{label: "INTERNAL", value: "INTERNAL"}, {label: "EXTERNAL", value: "EXTERNAL"}, {label: "PKI", value: "PKI"}]}
                 onChange={e => updateNested('aerospike-clusters', selectedId, 'credentials', 'auth-mode', e.target.value)}
               />

               <SectionHeader title="Advanced" />
               <div className="grid grid-cols-2 gap-4">
                 <Input label="Max Parallel Scans" type="number" value={items[selectedId]["max-parallel-scans"]} onChange={e => updateItem('aerospike-clusters', selectedId, 'max-parallel-scans', Number(e.target.value))} />
                 <Input label="Connection Timeout (ms)" type="number" value={items[selectedId]["conn-timeout"]} onChange={e => updateItem('aerospike-clusters', selectedId, 'conn-timeout', Number(e.target.value))} />
               </div>
            </div>
          ) : (
            <div className="flex flex-col items-center justify-center h-full text-gray-500">
                <Server size={48} className="mb-4 opacity-20" />
                <p>Select a cluster to configure</p>
            </div>
          )}
        </div>
      </div>
    );
  };

  const renderStorage = () => {
    const items = config['storage'];
    const getStorageType = (s: any) => {
        if (s["s3-storage"]) return "s3-storage";
        if (s["local-storage"]) return "local-storage";
        if (s["gcp-storage"]) return "gcp-storage";
        if (s["azure-storage"]) return "azure-storage";
        return "s3-storage";
    };

    const changeStorageType = (id: string, type: string) => {
        const defaults: any = {
            "s3-storage": { "bucket": "new-bucket", "s3-region": "us-east-1" },
            "local-storage": { "path": "/backups" },
            "gcp-storage": { "bucket-name": "new-bucket" },
            "azure-storage": { "container-name": "backup-container", "endpoint": "" }
        };
        setConfig(prev => ({
            ...prev,
            storage: {
                ...prev.storage,
                [id]: { [type]: defaults[type] }
            }
        }));
    };

    return (
      <div className="flex h-full">
        <div className="w-1/3 border-r border-gray-800 p-4 flex flex-col">
          <Button
            className="mb-4 w-full justify-center"
            icon={Plus}
            onClick={() => {
              const id = generateId('storage');
              setConfig(p => ({...p, storage: {...p.storage, [id]: { "s3-storage": { bucket: "my-bucket", "s3-region": "us-east-1" } }}}));
              setSelectedId(id);
            }}
          >
            New Storage
          </Button>
          <div className="overflow-y-auto flex-1 custom-scroll">
            {Object.entries(items).map(([k, v]) => (
              <Card
                key={k}
                title={k}
                sub={getStorageType(v).split('-')[0].toUpperCase()}
                active={selectedId === k}
                onClick={() => setSelectedId(k)}
                onDelete={() => deleteItem('storage', k)}
              />
            ))}
          </div>
        </div>
        <div className="w-2/3 p-6 overflow-y-auto custom-scroll">
          {selectedId && items[selectedId] ? (
            <div className="animate-in fade-in slide-in-from-right-4 duration-200">
               <div className="mb-6">
                  <Input label="Storage Name" value={selectedId} onChange={(e) => renameItem('storage', selectedId, e.target.value)} />
                  <Select
                    label="Type"
                    value={getStorageType(items[selectedId])}
                    options={[
                        { label: "Amazon S3 / MinIO", value: "s3-storage" },
                        { label: "Local Filesystem", value: "local-storage" },
                        { label: "Google Cloud Storage", value: "gcp-storage" },
                        { label: "Azure Blob Storage", value: "azure-storage" },
                    ]}
                    onChange={(e) => changeStorageType(selectedId, e.target.value)}
                  />
               </div>

               {items[selectedId]["s3-storage"] && (
                   <>
                     <SectionHeader title="S3 Configuration" icon={HardDrive} />
                     <div className="grid grid-cols-2 gap-4">
                        <Input label="Bucket" value={items[selectedId]["s3-storage"]!.bucket} onChange={e => updateNested('storage', selectedId, 's3-storage', 'bucket', e.target.value)} />
                        <Input label="Region" value={items[selectedId]["s3-storage"]!["s3-region"]} onChange={e => updateNested('storage', selectedId, 's3-storage', 's3-region', e.target.value)} />
                        <Input label="Path Prefix" value={items[selectedId]["s3-storage"]!.path} onChange={e => updateNested('storage', selectedId, 's3-storage', 'path', e.target.value)} />
                        <Input label="Endpoint (Optional)" value={items[selectedId]["s3-storage"]!["s3-endpoint-override"]} onChange={e => updateNested('storage', selectedId, 's3-storage', 's3-endpoint-override', e.target.value)} placeholder="e.g. localhost:9000" />
                     </div>
                     <SectionHeader title="Credentials" />
                     <div className="grid grid-cols-2 gap-4">
                        <Input label="Access Key ID" value={items[selectedId]["s3-storage"]!["access-key-id"]} onChange={e => updateNested('storage', selectedId, 's3-storage', 'access-key-id', e.target.value)} />
                        <Input label="Secret Access Key" type="password" value={items[selectedId]["s3-storage"]!["secret-access-key"]} onChange={e => updateNested('storage', selectedId, 's3-storage', 'secret-access-key', e.target.value)} />
                     </div>
                   </>
               )}

               {items[selectedId]["local-storage"] && (
                   <>
                     <SectionHeader title="Local Filesystem" icon={HardDrive} />
                     <Input label="Root Path" value={items[selectedId]["local-storage"]!.path} onChange={e => updateNested('storage', selectedId, 'local-storage', 'path', e.target.value)} />
                   </>
               )}

               {items[selectedId]["gcp-storage"] && (
                   <>
                     <SectionHeader title="Google Cloud" icon={HardDrive} />
                     <Input label="Bucket Name" value={items[selectedId]["gcp-storage"]!["bucket-name"]} onChange={e => updateNested('storage', selectedId, 'gcp-storage', 'bucket-name', e.target.value)} />
                     <Input label="Key File Path" value={items[selectedId]["gcp-storage"]!["key-file-path"]} onChange={e => updateNested('storage', selectedId, 'gcp-storage', 'key-file-path', e.target.value)} />
                   </>
               )}

               {items[selectedId]["azure-storage"] && (
                   <>
                     <SectionHeader title="Azure Blob" icon={HardDrive} />
                     <div className="grid grid-cols-2 gap-4">
                        <Input label="Container Name" value={items[selectedId]["azure-storage"]!["container-name"]} onChange={e => updateNested('storage', selectedId, 'azure-storage', 'container-name', e.target.value)} />
                        <Input label="Account Name" value={items[selectedId]["azure-storage"]!["account-name"]} onChange={e => updateNested('storage', selectedId, 'azure-storage', 'account-name', e.target.value)} />
                     </div>
                   </>
               )}
            </div>
          ) : (
            <div className="flex flex-col items-center justify-center h-full text-gray-500">
                <HardDrive size={48} className="mb-4 opacity-20" />
                <p>Select a storage backend to configure</p>
            </div>
          )}
        </div>
      </div>
    );
  };

  const renderPolicies = () => {
    const items = config['backup-policies'];
    return (
      <div className="flex h-full">
        <div className="w-1/3 border-r border-gray-800 p-4 flex flex-col">
          <Button
            className="mb-4 w-full justify-center"
            icon={Plus}
            onClick={() => {
              const id = generateId('policy');
              setConfig(p => ({...p, 'backup-policies': {...p['backup-policies'], [id]: { parallel: 8, retention: { full: 3, incremental: 0 }, compression: { mode: "NONE" }, encryption: { mode: "NONE" } }}}));
              setSelectedId(id);
            }}
          >
            New Policy
          </Button>
          <div className="overflow-y-auto flex-1 custom-scroll">
            {Object.entries(items).map(([k, v]) => (
              <Card
                key={k}
                title={k}
                sub={`Parallel: ${v.parallel}`}
                active={selectedId === k}
                onClick={() => setSelectedId(k)}
                onDelete={() => deleteItem('backup-policies', k)}
              />
            ))}
          </div>
        </div>
        <div className="w-2/3 p-6 overflow-y-auto custom-scroll">
          {selectedId && items[selectedId] ? (
            <div className="animate-in fade-in slide-in-from-right-4 duration-200">
               <div className="mb-6">
                  <Input label="Policy Name" value={selectedId} onChange={(e) => renameItem('backup-policies', selectedId, e.target.value)} />
               </div>

               <SectionHeader title="Performance" icon={Settings} />
               <div className="grid grid-cols-2 gap-4">
                  <Input label="Parallel Scans" type="number" value={items[selectedId].parallel} onChange={e => updateItem('backup-policies', selectedId, 'parallel', Number(e.target.value))} />
                  <Input label="Parallel Writes" type="number" value={items[selectedId]["parallel-write"]} onChange={e => updateItem('backup-policies', selectedId, 'parallel-write', Number(e.target.value))} />
               </div>

               <SectionHeader title="Retention & Compression" icon={Shield} />
               <div className="p-4 bg-gray-900/30 rounded border border-gray-800 mb-4">
                   <div className="grid grid-cols-2 gap-4">
                       <Input label="Keep Full Backups" type="number" value={items[selectedId].retention?.full} onChange={e => updateNested('backup-policies', selectedId, 'retention', 'full', Number(e.target.value))} />
                       <Input label="Keep Incremental" type="number" value={items[selectedId].retention?.incremental} onChange={e => updateNested('backup-policies', selectedId, 'retention', 'incremental', Number(e.target.value))} />
                   </div>
               </div>
               <div className="grid grid-cols-2 gap-4">
                   <Select
                     label="Compression"
                     value={items[selectedId].compression?.mode || "NONE"}
                     options={[{label: "None", value: "NONE"}, {label: "ZSTD", value: "ZSTD"}]}
                     onChange={e => updateNested('backup-policies', selectedId, 'compression', 'mode', e.target.value)}
                   />
                   {items[selectedId].compression?.mode === "ZSTD" && (
                       <Input label="Level (1-22)" type="number" value={items[selectedId].compression?.level} onChange={e => updateNested('backup-policies', selectedId, 'compression', 'level', Number(e.target.value))} />
                   )}
                   <Select
                     label="Encryption"
                     value={items[selectedId].encryption?.mode || "NONE"}
                     options={[{label: "None", value: "NONE"}, {label: "AES128", value: "AES128"}, {label: "AES256", value: "AES256"}]}
                     onChange={e => updateNested('backup-policies', selectedId, 'encryption', 'mode', e.target.value)}
                   />
               </div>
            </div>
          ) : (
            <div className="flex flex-col items-center justify-center h-full text-gray-500">
                <Shield size={48} className="mb-4 opacity-20" />
                <p>Select a policy to configure</p>
            </div>
          )}
        </div>
      </div>
    );
  };

  const renderSecrets = () => {
    const items = config['secret-agents'];
    return (
      <div className="flex h-full">
        <div className="w-1/3 border-r border-gray-800 p-4 flex flex-col">
          <Button
            className="mb-4 w-full justify-center"
            icon={Plus}
            onClick={() => {
              const id = generateId('agent');
              setConfig(p => ({...p, 'secret-agents': {...p['secret-agents'], [id]: { address: "localhost", port: 8081 }}}));
              setSelectedId(id);
            }}
          >
            New Agent
          </Button>
          <div className="overflow-y-auto flex-1 custom-scroll">
            {Object.entries(items).map(([k, v]) => (
              <Card
                key={k}
                title={k}
                sub={`${(v as any).address}:${(v as any).port}`}
                active={selectedId === k}
                onClick={() => setSelectedId(k)}
                onDelete={() => deleteItem('secret-agents', k)}
              />
            ))}
          </div>
        </div>
        <div className="w-2/3 p-6 overflow-y-auto custom-scroll">
          {selectedId && items[selectedId] ? (
            <div className="animate-in fade-in slide-in-from-right-4 duration-200">
               <div className="mb-6">
                  <Input label="Agent Name" value={selectedId} onChange={(e) => renameItem('secret-agents', selectedId, e.target.value)} />
               </div>
               <SectionHeader title="Connection Info" icon={Key} />
               <Input label="Address" value={(items[selectedId] as any).address} onChange={e => updateItem('secret-agents', selectedId, 'address', e.target.value)} />
               <Input label="Port" type="number" value={(items[selectedId] as any).port} onChange={e => updateItem('secret-agents', selectedId, 'port', Number(e.target.value))} />
               <Input label="TLS CA File" value={(items[selectedId] as any)["tls-ca-file"]} onChange={e => updateItem('secret-agents', selectedId, 'tls-ca-file', e.target.value)} />
            </div>
          ) : (
            <div className="flex flex-col items-center justify-center h-full text-gray-500">
                <Key size={48} className="mb-4 opacity-20" />
                <p>Select a secret agent to configure</p>
            </div>
          )}
        </div>
      </div>
    );
  };

  const renderService = () => {
      const srv = config.service;
      const updateSrv = (parent: 'http' | 'logger', field: string, value: any) => {
          setConfig(p => ({
              ...p,
              service: { ...p.service, [parent]: { ...p.service[parent], [field]: value } }
          }))
      }
      return (
          <div className="max-w-2xl mx-auto p-6">
              <h2 className="text-xl font-bold mb-6 flex items-center gap-2"><Settings className="text-red-500"/> Service Configuration</h2>

              <SectionHeader title="HTTP Server" />
              <div className="grid grid-cols-2 gap-4">
                  <Input label="Port" type="number" value={srv.http?.port} onChange={e => updateSrv('http', 'port', Number(e.target.value))} />
                  <Input label="Context Path" value={srv.http?.["context-path"]} onChange={e => updateSrv('http', 'context-path', e.target.value)} placeholder="/" />
              </div>

              <SectionHeader title="Logging" />
              <div className="grid grid-cols-2 gap-4">
                  <Select
                    label="Level"
                    value={srv.logger?.level}
                    options={['DEBUG', 'INFO', 'WARN', 'ERROR'].map(l => ({label: l, value: l}))}
                    onChange={e => updateSrv('logger', 'level', e.target.value)}
                  />
                  <Select
                    label="Format"
                    value={srv.logger?.format}
                    options={['PLAIN', 'JSON'].map(l => ({label: l, value: l}))}
                    onChange={e => updateSrv('logger', 'format', e.target.value)}
                  />
              </div>
          </div>
      )
  }

  const renderYaml = () => (
    <div className="max-w-2xl mx-auto text-center animate-in fade-in slide-in-from-bottom-4 pt-10">
        <CheckCircle size={64} className="text-green-500 mx-auto mb-6" />
        <h2 className="text-2xl font-bold text-white mb-2">Ready to Apply?</h2>
        <div className="w-full bg-gray-900 p-4 rounded text-left font-mono text-xs text-gray-400 border border-gray-800 mb-6 h-96 overflow-y-auto">
            <pre>{toYaml(config)}</pre>
        </div>
        {saveError && (
          <div className="bg-red-900/30 border border-red-800 text-red-400 text-sm p-3 rounded mb-4">
            <strong>Error:</strong> {saveError}
          </div>
        )}
        <div className="flex justify-center gap-4">
            <Button variant="secondary" onClick={() => navigator.clipboard.writeText(toYaml(config))} icon={Copy}>Copy to Clipboard</Button>
            <Button variant="primary" onClick={handleApply} icon={isSaving ? Loader : Play} disabled={isSaving}>
              {isSaving ? 'Saving...' : 'Apply Configuration'}
            </Button>
        </div>
    </div>
  );

  const NavItem = ({ id, label, icon: Icon }: { id: SectionId; label: string; icon: any }) => (
    <button
      onClick={() => { setSection(id); setSelectedId(null); }}
      className={`flex items-center gap-3 w-full px-3 py-2 rounded-md transition-all text-sm mb-1 ${
        section === id ? 'bg-red-600/10 text-red-500 border-l-2 border-red-500' : 'text-gray-400 hover:bg-gray-800 hover:text-white'
      }`}
    >
      <Icon size={16} />
      <span>{label}</span>
    </button>
  );

  return (
    <div className="grid grid-cols-12 h-full">
      <div className="col-span-2 border-r border-gray-800 bg-gray-900/30 flex flex-col py-4">
         <div className="px-4 mb-2">
            <h3 className="text-xs font-bold text-gray-500 uppercase tracking-wider">Definitions</h3>
         </div>
         <NavItem id="routines" label="Routines" icon={Clock} />
         <NavItem id="clusters" label="Clusters" icon={Server} />
         <NavItem id="storage" label="Storage" icon={HardDrive} />
         <NavItem id="policies" label="Policies" icon={Shield} />
         <NavItem id="secrets" label="Secrets" icon={Key} />
         <NavItem id="service" label="Service" icon={Settings} />
         <div className="my-4 border-t border-gray-800"></div>
         <NavItem id="yaml" label="Apply Config" icon={Play} />
      </div>
      <div className="col-span-10 bg-gray-950 overflow-hidden">
         {section === 'routines' && renderRoutines()}
         {section === 'clusters' && renderClusters()}
         {section === 'storage' && renderStorage()}
         {section === 'policies' && renderPolicies()}
         {section === 'secrets' && renderSecrets()}
         {section === 'service' && renderService()}
         {section === 'yaml' && renderYaml()}
      </div>
    </div>
  );
}