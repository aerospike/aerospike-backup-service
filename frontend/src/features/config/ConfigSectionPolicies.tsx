import React from 'react';
import {Archive, Lock, Minimize, Plus, Settings, Shield} from 'lucide-react';
import {Button} from '@/components/ui/Button';
import {Input, Select} from '@/components/ui/Inputs';
import {Slider} from '@/components/ui/Slider';
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
            <div className="w-1/3 border-r border-gray-200 p-4 flex flex-col">
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
                    {Object.entries(items)
                        .sort((a, b) => a[0].localeCompare(b[0]))
                        .map(([k, v]: [string, DtoBackupPolicy]) => (
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
                            <Input label="Parallel Scans" type="number" value={items[selectedId].parallel || ''}
                                   onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateItem('backupPolicies', selectedId, 'parallel', Number(e.target.value))}/>
                            <Input label="Parallel Writes" type="number" value={items[selectedId].parallelWrite || ''}
                                   onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateItem('backupPolicies', selectedId, 'parallelWrite', Number(e.target.value))}/>
                        </div>

                        {/* Retention Section */}
                        <SectionHeader title="Retention" icon={Archive}/>
                        <p className="text-sm text-gray-600 mb-4">Specifies how long to retain full and incremental
                            backups. Cleanup runs asynchronously after each successful full backup, never deleting
                            backups preemptively. Ensure storage capacity for at least one extra full backup beyond the
                            retention configuration.</p>
                        <div className="p-4 bg-gray-50 rounded border border-gray-200 mb-6">
                            <div className="grid grid-cols-2 gap-4">
                                <Input label="Keep Full Backups" type="number"
                                       value={items[selectedId].retention?.full || ''}
                                       onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateNested('backupPolicies', selectedId, 'retention', 'full', Number(e.target.value))}/>
                                <Input label="Keep Incremental" type="number"
                                       value={items[selectedId].retention?.incremental || ''}
                                       onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateNested('backupPolicies', selectedId, 'retention', 'incremental', Number(e.target.value))}/>
                            </div>
                        </div>

                        {/* Compression Section */}
                        <SectionHeader title="Compression" icon={Minimize}/>
                        <p className="text-sm text-gray-600 mb-4">Compression details (algorithm and mode). Default is
                            no compression. Enabling compression reduces storage and network usage, but increases CPU
                            usage during the backup. Depending on the system configuration, compression may improve or
                            degrade overall performance.</p>
                        <div className="grid grid-cols-2 gap-4 mb-6">
                            <Select
                                label="Compression"
                                value={items[selectedId].compression?.mode || "NONE"}
                                options={[{label: "None", value: "NONE"}, {label: "ZSTD", value: "ZSTD"}]}
                                onChange={(e: React.ChangeEvent<HTMLSelectElement>) => updateNested('backupPolicies', selectedId, 'compression', 'mode', e.target.value)}
                            />
                            {items[selectedId].compression?.mode === "ZSTD" && (
                                <Slider
                                    label="Level (1-22)"
                                    value={items[selectedId].compression?.level || 1}
                                    min={1}
                                    max={22}
                                    step={1}
                                    onChange={(val: number) => updateNested('backupPolicies', selectedId, 'compression', 'level', val)}
                                    description="Lower value: faster compression; Higher value: max compressed."
                                />
                            )}
                        </div>

                        {/* Encryption Section */}
                        <SectionHeader title="Encryption" icon={Lock}/>
                        <p className="text-sm text-gray-600 mb-4">Encryption details (algorithm and key). Default is no
                            encryption.</p>
                        <div className="grid grid-cols-2 gap-4 mb-6">
                            <Select
                                label="Encryption"
                                value={items[selectedId].encryption?.mode || "NONE"}
                                options={[{label: "None", value: "NONE"}, {
                                    label: "AES128",
                                    value: "AES128"
                                }, {label: "AES256", value: "AES256"}]}
                                onChange={(e: React.ChangeEvent<HTMLSelectElement>) => updateNested('backupPolicies', selectedId, 'encryption', 'mode', e.target.value)}
                            />
                            {items[selectedId].encryption && items[selectedId].encryption.mode !== "NONE" && (
                                <>
                                    <Input
                                        label="Key Environment Variable"
                                        type="text"
                                        value={items[selectedId].encryption?.keyEnv || ''}
                                        disabled={!!items[selectedId].encryption?.keyFile || !!items[selectedId].encryption?.keySecret}
                                        onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateNested('backupPolicies', selectedId, 'encryption', 'keyEnv', e.target.value)}
                                    />
                                    <Input
                                        label="Key File Path"
                                        type="text"
                                        value={items[selectedId].encryption?.keyFile || ''}
                                        disabled={!!items[selectedId].encryption?.keyEnv || !!items[selectedId].encryption?.keySecret}
                                        onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateNested('backupPolicies', selectedId, 'encryption', 'keyFile', e.target.value)}
                                    />
                                    <Input
                                        label="Key Secret Agent Keyword"
                                        type="text"
                                        value={items[selectedId].encryption?.keySecret || ''}
                                        disabled={!!items[selectedId].encryption?.keyEnv || !!items[selectedId].encryption?.keyFile}
                                        onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateNested('backupPolicies', selectedId, 'encryption', 'keySecret', e.target.value)}
                                    />
                                </>
                            )}
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
