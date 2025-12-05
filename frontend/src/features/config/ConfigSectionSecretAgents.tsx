import React from 'react';
import {Key, Plus} from 'lucide-react';
import {Button} from '@/components/ui/Button';
import {Input, RadioGroup, Select} from '@/components/ui/Inputs';
import {api, DtoConfig, DtoSecretAgent} from '@/api';
import {Card, SectionHeader} from './ConfigEditorShared';
import {ConnectivityCheckButton} from '@/components/ui/ConnectivityCheckButton';

interface ConfigSectionSecretAgentsProps {
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

export const ConfigSectionSecretAgents = (
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
    }: ConfigSectionSecretAgentsProps
) => {
    const items = config.secretAgents || {};

    return (
        <div className="flex h-full">
            <div className="w-1/3 border-r border-gray-800 p-4 flex flex-col">
                <Button
                    className="mb-4 w-full justify-center"
                    icon={Plus}
                    onClick={() => {
                        const id = generateId('secret-agent');
                        setConfig((p: DtoConfig) => ({
                            ...p,
                            secretAgents: {
                                ...p.secretAgents,
                                [id]: {
                                    connectionType: "tcp",
                                    address: "localhost",
                                    port: 8080,
                                    timeout: 1000,
                                    tlsCaFile: "",
                                    isBase64: false
                                }
                            }
                        }));
                        setSelectedId(id);
                    }}
                >
                    New Secret Agent
                </Button>
                <div className="overflow-y-auto flex-1 custom-scroll">
                    {Object.entries(items)
                        .sort((a, b) => a[0].localeCompare(b[0]))
                        .map(([k, v]: [string, DtoSecretAgent]) => (
                            <Card
                                key={k}
                                title={k}
                                sub={v.address}
                                active={selectedId === k}
                                onClick={() => {
                                    setSelectedId(k);
                                }}
                                onDelete={() => deleteItem('secretAgents', k)}
                            />
                        ))}
                </div>
            </div>
            <div className="w-2/3 p-6 overflow-y-auto custom-scroll">
                {selectedId && items[selectedId] ? (
                    <div className="animate-in fade-in slide-in-from-right-4 duration-200">
                        <div className="mb-6 flex items-end gap-4">
                            <div className="flex-1">
                                <Input label="Secret Agent Name" value={selectedId}
                                       onChange={(e: React.ChangeEvent<HTMLInputElement>) => renameItem('secretAgents', selectedId, e.target.value)}/>
                            </div>
                            <ConnectivityCheckButton
                                className="mb-3"
                                onCheck={async () => {
                                    if (selectedId && items[selectedId]) {
                                        await api.checkSecretAgentConnectivity(items[selectedId]);
                                    }
                                }}
                                label="Check Connection"
                            />
                        </div>

                        <SectionHeader title="Connection" icon={Key}/>
                        <RadioGroup
                            label="Connection Type"
                            value={items[selectedId].connectionType || "tcp"}
                            options={[
                                { label: "TCP", value: "tcp" },
                                { label: "Unix Socket", value: "unix" }
                            ]}
                            onChange={(value: string) => updateItem('secretAgents', selectedId, 'connectionType', value)}
                        />
                        <Input label="Address" value={items[selectedId].address || ''}
                               onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateItem('secretAgents', selectedId, 'address', e.target.value)}/>
                        {items[selectedId].connectionType === "tcp" && (
                            <Input label="Port" type="number" value={items[selectedId].port || ''}
                                   onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateItem('secretAgents', selectedId, 'port', Number(e.target.value))}/>
                        )}
                        <Input label="Timeout (ms)" type="number" value={items[selectedId].timeout || ''}
                               onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateItem('secretAgents', selectedId, 'timeout', Number(e.target.value))}/>
                        <Input label="TLS CA File Path" value={items[selectedId].tlsCaFile || ''}
                               onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateItem('secretAgents', selectedId, 'tlsCaFile', e.target.value)}/>
                        <Input label="Is Base64 Encrypted" type="checkbox" checked={items[selectedId].isBase64 || false}
                               onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateItem('secretAgents', selectedId, 'isBase64', e.target.checked)}/>

                    </div>
                ) : (
                    <div className="flex flex-col items-center justify-center h-full text-gray-500">
                        <Key size={48} className="mb-4 opacity-20"/>
                        <p>Select a Secret Agent to configure</p>
                    </div>
                )}
            </div>
        </div>
    );
};
