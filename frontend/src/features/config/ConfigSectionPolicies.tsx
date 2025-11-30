import React from 'react';
import {Plus, Settings, Shield} from 'lucide-react';
import {Button} from '@/components/ui/Button';
import {Input, Select} from '@/components/ui/Inputs';
import {DtoBackupPolicy, DtoConfig} from '@/api';
import {Card, SectionHeader} from './ConfigEditorShared';

interface ConfigSectionPoliciesProps {
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

export const ConfigSectionPolicies = (
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
    }: ConfigSectionPoliciesProps
) => {
    const items = config.backupPolicies || {};
    return (
        <div className="flex h-full">
            <div className="w-1/3 border-r border-gray-800 p-4 flex flex-col">
                <Button
                    className="mb-4 w-full justify-center"
                    icon={Plus}
                    onClick={() => {
                        const id = generateId('policy');
                        setConfig((p: DtoConfig) => ({
                            ...p,
                            backupPolicies: {
                                ...p.backupPolicies,
                                [id]: {
                                    parallel: 8,
                                    retention: {full: 3, incremental: 0},
                                    compression: {mode: "NONE"},
                                    encryption: {mode: "NONE"}
                                }
                            }
                        }));
                        setSelectedId(id);
                    }}
                >
                    New Policy
                </Button>
                <div className="overflow-y-auto flex-1 custom-scroll">
                    {Object.entries(items).map(([k, v]: [string, DtoBackupPolicy]) => (
                        <Card
                            key={k}
                            title={k}
                            sub={`Parallel: ${v.parallel}`}
                            active={selectedId === k}
                            onClick={() => setSelectedId(k)}
                            onDelete={() => deleteItem('backupPolicies', k)}
                        />
                    ))}
                </div>
            </div>
            <div className="w-2/3 p-6 overflow-y-auto custom-scroll">
                {selectedId && items[selectedId] ? (
                    <div className="animate-in fade-in slide-in-from-right-4 duration-200">
                        <div className="mb-6">
                            <Input label="Policy Name" value={selectedId}
                                   onChange={(e: React.ChangeEvent<HTMLInputElement>) => renameItem('backupPolicies', selectedId, e.target.value)}/>
                        </div>

                        <SectionHeader title="Performance" icon={Settings}/>
                        <div className="grid grid-cols-2 gap-4">
                            <Input label="Parallel Scans" type="number" value={items[selectedId].parallel}
                                   onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateItem('backupPolicies', selectedId, 'parallel', Number(e.target.value))}/>
                            <Input label="Parallel Writes" type="number" value={items[selectedId].parallelWrite}
                                   onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateItem('backupPolicies', selectedId, 'parallelWrite', Number(e.target.value))}/>
                        </div>

                        <SectionHeader title="Retention & Compression" icon={Shield}/>
                        <div className="p-4 bg-gray-900/30 rounded border border-gray-800 mb-4">
                            <div className="grid grid-cols-2 gap-4">
                                <Input label="Keep Full Backups" type="number" value={items[selectedId].retention?.full}
                                       onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateNested('backupPolicies', selectedId, 'retention', 'full', Number(e.target.value))}/>
                                <Input label="Keep Incremental" type="number"
                                       value={items[selectedId].retention?.incremental}
                                       onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateNested('backupPolicies', selectedId, 'retention', 'incremental', Number(e.target.value))}/>
                            </div>
                        </div>
                        <div className="grid grid-cols-2 gap-4">
                            <Select
                                label="Compression"
                                value={items[selectedId].compression?.mode || "NONE"}
                                options={[{label: "None", value: "NONE"}, {label: "ZSTD", value: "ZSTD"}]}
                                onChange={(e: React.ChangeEvent<HTMLSelectElement>) => updateNested('backupPolicies', selectedId, 'compression', 'mode', e.target.value)}
                            />
                            {items[selectedId].compression?.mode === "ZSTD" && (
                                <Input label="Level (1-22)" type="number" value={items[selectedId].compression?.level}
                                       onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateNested('backupPolicies', selectedId, 'compression', 'level', Number(e.target.value))}/>
                            )}
                            <Select
                                label="Encryption"
                                value={items[selectedId].encryption?.mode || "NONE"}
                                options={[{label: "None", value: "NONE"}, {
                                    label: "AES128",
                                    value: "AES128"
                                }, {label: "AES256", value: "AES256"}]}
                                onChange={(e: React.ChangeEvent<HTMLSelectElement>) => updateNested('backupPolicies', selectedId, 'encryption', 'mode', e.target.value)}
                            />
                        </div>
                    </div>
                ) : (
                    <div className="flex flex-col items-center justify-center h-full text-gray-500">
                        <Shield size={48} className="mb-4 opacity-20"/>
                        <p>Select a policy to configure</p>
                    </div>
                )}
            </div>
        </div>
    );
};
