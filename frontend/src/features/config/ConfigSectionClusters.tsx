import React, {useState} from 'react';
import {Activity, Key, Plus, Server, Trash2} from 'lucide-react';
import {Button} from '@/components/ui/Button';
import {Input, Select} from '@/components/ui/Inputs';
import {api, DtoAerospikeCluster, DtoConfig} from '@/api';
import {Card, SectionHeader} from './ConfigEditorShared';

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
    const [isChecking, setIsChecking] = useState(false);
    const [checkResult, setCheckResult] = useState<{ type: 'success' | 'error', message: string } | null>(null);

    const handleCheckConnectivity = async () => {
        if (!selectedId || !items[selectedId]) return;
        setIsChecking(true);
        setCheckResult(null);
        try {
            await api.checkClusterConnectivity(items[selectedId]);
            setCheckResult({type: 'success', message: 'Connectivity check successful'});
        } catch (e: any) {
            setCheckResult({type: 'error', message: 'Connectivity check failed'});
        } finally {
            setIsChecking(false);
        }
    };

    return (
        <div className="flex h-full">
            <div className="w-1/3 border-r border-gray-800 p-4 flex flex-col">
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
                                    setCheckResult(null);
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
                            <Button
                                variant="secondary"
                                className="mb-3"
                                onClick={handleCheckConnectivity}
                                loading={isChecking}
                                icon={Activity}
                            >
                                Check Connectivity
                            </Button>
                        </div>

                        {checkResult && (
                            <div
                                className={`mb-6 p-3 rounded border text-sm ${checkResult.type === 'success' ? 'bg-green-100 border-green-200 text-green-700' : 'bg-red-100 border-red-200 text-red-700'}`}>
                                {checkResult.message}
                            </div>
                        )}

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
                        <Input label="User" value={items[selectedId].credentials?.user || ''}
                               onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateNested('aerospikeClusters', selectedId, 'credentials', 'user', e.target.value)}/>
                        <Input label="Password" type="password" value={items[selectedId].credentials?.password || ''}
                               onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateNested('aerospikeClusters', selectedId, 'credentials', 'password', e.target.value)}/>
                        <Select
                            label="Auth Mode"
                            value={items[selectedId].credentials?.authMode || "INTERNAL"}
                            options={[{label: "INTERNAL", value: "INTERNAL"}, {
                                label: "EXTERNAL",
                                value: "EXTERNAL"
                            }, {label: "PKI", value: "PKI"}]}
                            onChange={(e: React.ChangeEvent<HTMLSelectElement>) => updateNested('aerospikeClusters', selectedId, 'credentials', 'authMode', e.target.value)}
                        />

                        <SectionHeader title="Advanced"/>
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
