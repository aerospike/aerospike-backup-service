import React, {useState, useEffect} from 'react';
import {Key, Plus, Server, Trash2, Lock, Shield} from 'lucide-react';
import {Button} from '@/components/ui/Button';
import {Input, Select, Checkbox} from '@/components/ui/Inputs';
import {api, DtoAerospikeCluster, DtoConfig} from '@/api';
import {Card, SectionHeader} from './ConfigEditorShared';
import {ConnectivityCheckButton} from '@/components/ui/ConnectivityCheckButton';

interface ConfigSectionClustersProps {
    config: DtoConfig;
    setConfig: React.Dispatch<React.SetStateAction<DtoConfig>>;
    selectedId: string | null;
    setSelectedId: (id: string | null) => void;
    generateId: (prefix: string) => string;
    updateItem: (sectionKey: keyof DtoConfig, id: string, field: string, value: any) => void;
    updateNested: (sectionKey: keyof DtoConfig, id: string, parent: string, field: string, value: any) => void;
    deleteItem: (sectionKey: keyof DtoConfig, id: string) => void;
    renameItem: (sectionKey: keyof DtoConfig, oldId: string, newId: string) => void;
}

export const ConfigSectionClusters = (
    {
        config,
        setConfig,
        selectedId,
        setSelectedId,
        generateId,
        updateItem,
        updateNested,
        deleteItem,
        renameItem
    }: ConfigSectionClustersProps
) => {
    const items = config.aerospikeClusters || {};
    
    // Authentication mode state (including NONE)
    const [authModeSelection, setAuthModeSelection] = useState<'NONE' | 'INTERNAL' | 'EXTERNAL' | 'PKI'>('NONE');

    useEffect(() => {
        if (!selectedId || !items[selectedId]) return;
        
        const creds = items[selectedId].credentials;
        if (!creds) {
            setAuthModeSelection('NONE');
        } else {
             // If credentials exist but authMode is undefined, default to INTERNAL in the UI selection if user/pass are present, or just INTERNAL as default.
             // However, DtoCredentials usually has authMode set.
             setAuthModeSelection(creds.authMode as 'INTERNAL' | 'EXTERNAL' | 'PKI' || 'INTERNAL');
        }
    }, [selectedId, items]);

    const handleAuthModeChange = (newMode: 'NONE' | 'INTERNAL' | 'EXTERNAL' | 'PKI') => {
        setAuthModeSelection(newMode);
        if (!selectedId) return;

        if (newMode === 'NONE') {
            updateItem('aerospikeClusters', selectedId, 'credentials', null);
        } else {
            // Initialize credentials if null, or update authMode
            const cluster = items[selectedId];
            if (!cluster) return;
            const currentCreds = cluster.credentials || {};
            const newCreds = {
                ...currentCreds,
                authMode: newMode,
                // Ensure user is initialized if empty
                user: currentCreds.user || '', 
            };
            updateItem('aerospikeClusters', selectedId, 'credentials', newCreds);
        }
    };

    return (
        <div className="flex h-full">
            <div className="w-1/3 border-r border-gray-200 p-4 flex flex-col">
                <Button
                    className="mb-4 w-full justify-center"
                    icon={Plus}
                    onClick={() => {
                        const id = generateId('cluster');
                        setConfig((p: DtoConfig) => ({
                            ...p,
                            aerospikeClusters: {
                                ...p.aerospikeClusters,
                                [id]: {seedNodes: [{hostName: "localhost", port: 3000}], credentials: {user: ""}}
                            }
                        }));
                        setSelectedId(id);
                    }}
                >
                    New Cluster
                </Button>
                <div className="overflow-y-auto flex-1 custom-scroll">
                    {Object.entries(items)
                        .sort((a, b) => a[0].localeCompare(b[0]))
                        .map(([k, v]: [string, DtoAerospikeCluster]) => (
                            <Card
                                key={k}
                                title={k}
                                sub={`${v.seedNodes?.length || 0} nodes`}
                                active={selectedId === k}
                                onClick={() => {
                                    setSelectedId(k);
                                }}
                                onDelete={() => deleteItem('aerospikeClusters', k)}
                            />
                        ))}
                </div>
            </div>
            <div className="w-2/3 p-6 overflow-y-auto custom-scroll">
                {selectedId && items[selectedId] ? (
                    <div className="animate-in fade-in slide-in-from-right-4 duration-200">
                        <div className="mb-6 flex items-end gap-4">
                            <div className="flex-1">
                                <Input label="Cluster Name" value={selectedId}
                                       onChange={(e: React.ChangeEvent<HTMLInputElement>) => renameItem('aerospikeClusters', selectedId, e.target.value)}/>
                            </div>
                            <ConnectivityCheckButton
                                className="mb-3"
                                onCheck={async () => {
                                    if (selectedId && items[selectedId]) {
                                        await api.checkClusterConnectivity(items[selectedId]);
                                    }
                                }}
                            />
                        </div>

                        <SectionHeader title="Seed Nodes" icon={Server}/>
                        <div className="space-y-2 mb-6">
                            {items[selectedId].seedNodes?.map((node: {
                                hostName: string;
                                port: number;
                            }, idx: number) => (
                                <div key={idx} className="flex gap-2">
                                    <Input className="flex-1" placeholder="Hostname" value={node.hostName}
                                           onChange={(e: React.ChangeEvent<HTMLInputElement>) => {
                                               if (!selectedId) return;
                                               const item = items[selectedId];
                                               if (!item) return;
                                               const nodes = [...item.seedNodes || []];
                                               const nodeToUpdate = nodes[idx];
                                               if (nodeToUpdate) {
                                                   nodeToUpdate.hostName = e.target.value;
                                               }
                                               updateItem('aerospikeClusters', selectedId, 'seedNodes', nodes);
                                           }}/>
                                    <Input className="w-24" type="number" placeholder="Port" value={node.port}
                                           onChange={(e: React.ChangeEvent<HTMLInputElement>) => {
                                               if (!selectedId) return;
                                               const item = items[selectedId];
                                               if (!item) return;
                                               const nodes = [...item.seedNodes || []];
                                               const nodeToUpdate = nodes[idx];
                                               if (nodeToUpdate) {
                                                   nodeToUpdate.port = Number(e.target.value);
                                               }
                                               updateItem('aerospikeClusters', selectedId, 'seedNodes', nodes);
                                           }}/>
                                    <Button variant="ghost" className="px-2 text-gray-500 hover:text-red-500"
                                            onClick={() => {
                                                if (!selectedId) return;
                                                const item = items[selectedId];
                                                if (!item) return;
                                                const nodes = (item.seedNodes || []).filter((_: {
                                                    hostName: string;
                                                    port: number;
                                                }, i: number) => i !== idx);
                                                updateItem('aerospikeClusters', selectedId, 'seedNodes', nodes);
                                            }}><Trash2 size={16}/></Button>
                                </div>
                            ))}
                            <Button variant="secondary" className="w-full justify-center py-1 text-xs" onClick={() => {
                                if (!selectedId) return;
                                const item = items[selectedId];
                                if (!item) return;
                                const nodes = [...item.seedNodes || [], {hostName: "localhost", port: 3000}];
                                updateItem('aerospikeClusters', selectedId, 'seedNodes', nodes);
                            }}>+ Add Node</Button>
                        </div>

                        <SectionHeader title="Authentication" icon={Key}/>
                        <Select
                            label="Auth Mode"
                            value={authModeSelection}
                            options={[
                                {label: "NONE", value: "NONE"},
                                {label: "INTERNAL", value: "INTERNAL"},
                                {label: "EXTERNAL", value: "EXTERNAL"},
                                {label: "PKI", value: "PKI"}
                            ]}
                            onChange={(e: React.ChangeEvent<HTMLSelectElement>) => handleAuthModeChange(e.target.value as any)}
                        />

                        {authModeSelection !== 'NONE' && (
                            <div className="pl-4 border-l-2 border-gray-100 mt-4">
                                <Input label="User" value={items[selectedId].credentials?.user || ''}
                                       onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateNested('aerospikeClusters', selectedId, 'credentials', 'user', e.target.value)}/>
                                
                                <Input
                                    label="Password"
                                    type="password"
                                    value={items[selectedId].credentials?.password || ''}
                                    disabled={!!items[selectedId].credentials?.passwordPath}
                                    onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateNested('aerospikeClusters', selectedId, 'credentials', 'password', e.target.value)}
                                />
                                <Input
                                    label="Password File Path"
                                    value={items[selectedId].credentials?.passwordPath || ''}
                                    disabled={!!items[selectedId].credentials?.password}
                                    onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateNested('aerospikeClusters', selectedId, 'credentials', 'passwordPath', e.target.value)}
                                />
                            </div>
                        )}

                        <SectionHeader title="TLS" icon={Lock}/>
                        <p className="text-sm text-gray-600 mb-4">Transport Layer Security (TLS) configuration.</p>
                        <div className="grid grid-cols-2 gap-4 mb-6">
                            <Input label="Name" value={items[selectedId].tls?.name || ''}
                                   onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateNested('aerospikeClusters', selectedId, 'tls', 'name', e.target.value)}/>
                            <Input label="Protocols" value={items[selectedId].tls?.protocols || ''}
                                   placeholder="e.g. TLSv1.2"
                                   onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateNested('aerospikeClusters', selectedId, 'tls', 'protocols', e.target.value)}/>
                        </div>
                        <div className="grid grid-cols-2 gap-4 mb-6">
                            <Input label="CA File" value={items[selectedId].tls?.caFile || ''}
                                   onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateNested('aerospikeClusters', selectedId, 'tls', 'caFile', e.target.value)}/>
                            <Input label="CA Path" value={items[selectedId].tls?.caPath || ''}
                                   onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateNested('aerospikeClusters', selectedId, 'tls', 'caPath', e.target.value)}/>
                        </div>
                        <div className="grid grid-cols-2 gap-4 mb-6">
                            <Input label="Cert File" value={items[selectedId].tls?.certFile || ''}
                                   onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateNested('aerospikeClusters', selectedId, 'tls', 'certFile', e.target.value)}/>
                             <Input label="Key File" value={items[selectedId].tls?.keyFile || ''}
                                   onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateNested('aerospikeClusters', selectedId, 'tls', 'keyFile', e.target.value)}/>
                        </div>
                         <div className="grid grid-cols-2 gap-4 mb-6">
                            <Input label="Key File Password" type="password" value={items[selectedId].tls?.keyFilePassword || ''}
                                   onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateNested('aerospikeClusters', selectedId, 'tls', 'keyFilePassword', e.target.value)}/>
                            <Input label="Cipher Suite" value={items[selectedId].tls?.cipherSuite || ''}
                                   onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateNested('aerospikeClusters', selectedId, 'tls', 'cipherSuite', e.target.value)}/>
                        </div>

                        <SectionHeader title="Advanced"/>
                        <div className="mb-4">
                            <Checkbox 
                                label="Use Services Alternate" 
                                checked={!!items[selectedId].useServicesAlternate}
                                onChange={(checked: boolean) => updateItem('aerospikeClusters', selectedId, 'useServicesAlternate', checked)}
                            />
                        </div>
                        <div className="grid grid-cols-2 gap-4">
                            <Input label="Max Parallel Scans" type="number"
                                   value={items[selectedId].maxParallelScans || ''}
                                   onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateItem('aerospikeClusters', selectedId, 'maxParallelScans', Number(e.target.value))}/>
                            <Input label="Connection Timeout (ms)" type="number" value={items[selectedId].connTimeout}
                                   onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateItem('aerospikeClusters', selectedId, 'connTimeout', Number(e.target.value))}/>
                        </div>
                    </div>
                ) : (
                    <div className="flex flex-col items-center justify-center h-full text-gray-500">
                        <Server size={48} className="mb-4 opacity-20"/>
                        <p>Select a cluster to configure</p>
                    </div>
                )}
            </div>
        </div>
    );
};
