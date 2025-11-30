import React, {useState} from 'react';
import {Clock, HardDrive, Key, Play, Server, Settings, Shield} from 'lucide-react';
import type {DtoConfig} from '@/api';
import {api} from '@/api';
import {ConfigSectionRoutines} from './ConfigSectionRoutines';
import {ConfigSectionClusters} from './ConfigSectionClusters';
import {ConfigSectionStorage} from './ConfigSectionStorage';
import {ConfigSectionPolicies} from './ConfigSectionPolicies';
import {ConfigSectionSecrets} from './ConfigSectionSecrets';
import {ConfigSectionService} from './ConfigSectionService';
import {ConfigSectionYaml} from './ConfigSectionYaml';

// --- Main Editor ---

interface ConfigEditorProps {
    config: DtoConfig;
    setConfig: React.Dispatch<React.SetStateAction<DtoConfig>>;
}

type SectionId = 'routines' | 'clusters' | 'storage' | 'policies' | 'secrets' | 'service' | 'yaml';

export default function ConfigEditor({config, setConfig}: ConfigEditorProps) {
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
    const updateItem = (sectionKey: keyof DtoConfig, id: string, field: string, value: any) => {
        setConfig((prev: DtoConfig) => ({
            ...prev,
            [sectionKey]: {
                ...(prev[sectionKey] as any),
                [id]: {...(prev[sectionKey] as any)[id], [field]: value}
            }
        }));
    };

    // Deep updater for nested properties (e.g. cluster.credentials.user)
    const updateNested = (sectionKey: keyof DtoConfig, id: string, parent: string, field: string, value: any) => {
        setConfig((prev: DtoConfig) => {
            const item = (prev[sectionKey] as any)[id];
            return {
                ...prev,
                [sectionKey]: {
                    ...(prev[sectionKey] as any),
                    [id]: {
                        ...item,
                        [parent]: {...item[parent], [field]: value}
                    }
                }
            };
        });
    };

    const deleteItem = (sectionKey: keyof DtoConfig, id: string) => {
        if (!window.confirm(`Delete ${id}?`)) return;
        setConfig((prev: DtoConfig) => {
            const next = {...(prev[sectionKey] as any)};
            delete next[id];
            return {...prev, [sectionKey]: next};
        });
        if (selectedId === id) setSelectedId(null);
    };

    const renameItem = (sectionKey: keyof DtoConfig, oldId: string, newId: string) => {
        if (oldId === newId || !newId) return;
        setConfig((prev: DtoConfig) => {
            const collection = {...(prev[sectionKey] as any)};
            collection[newId] = collection[oldId];
            delete collection[oldId];
            return {...prev, [sectionKey]: collection};
        });
        setSelectedId(newId);
    }

    const NavItem = ({id, label, icon: Icon}: { id: SectionId; label: string; icon: any }) => (
        <button
            onClick={() => {
                setSection(id);
                setSelectedId(null);
            }}
            className={`flex items-center gap-3 w-full px-3 py-2 rounded-md transition-all text-sm mb-1 ${
                section === id ? 'bg-red-600/10 text-red-500 border-l-2 border-red-500' : 'text-gray-400 hover:bg-gray-800 hover:text-white'
            }`}
        >
            <Icon size={16}/>
            <span>{label}</span>
        </button>
    );

    return (
        <div className="grid grid-cols-12 h-full">
            <div className="col-span-2 border-r border-gray-800 bg-gray-900/30 flex flex-col py-4">
                <div className="px-4 mb-2">
                    <h3 className="text-xs font-bold text-gray-500 uppercase tracking-wider">Definitions</h3>
                </div>
                <NavItem id="routines" label="Routines" icon={Clock}/>
                <NavItem id="clusters" label="Clusters" icon={Server}/>
                <NavItem id="storage" label="Storage" icon={HardDrive}/>
                <NavItem id="policies" label="Policies" icon={Shield}/>
                <NavItem id="secrets" label="Secrets" icon={Key}/>
                <NavItem id="service" label="Service" icon={Settings}/>
                <div className="my-4 border-t border-gray-800"></div>
                <NavItem id="yaml" label="Apply Config" icon={Play}/>
            </div>
            <div className="col-span-10 bg-gray-950 overflow-hidden">
                {section === 'routines' &&
                    <ConfigSectionRoutines
                        config={config}
                        setConfig={setConfig}
                        selectedId={selectedId}
                        setSelectedId={setSelectedId}
                        generateId={generateId}
                        updateItem={updateItem}
                        deleteItem={deleteItem}
                        renameItem={renameItem}
                    />}
                {section === 'clusters' &&
                    <ConfigSectionClusters
                        config={config}
                        setConfig={setConfig}
                        selectedId={selectedId}
                        setSelectedId={setSelectedId}
                        generateId={generateId}
                        updateItem={updateItem}
                        updateNested={updateNested}
                        deleteItem={deleteItem}
                        renameItem={renameItem}
                    />
                }
                {section === 'storage' &&
                    <ConfigSectionStorage
                        config={config}
                        setConfig={setConfig}
                        selectedId={selectedId}
                        setSelectedId={setSelectedId}
                        generateId={generateId}
                        updateItem={updateItem}
                        updateNested={updateNested}
                        deleteItem={deleteItem}
                        renameItem={renameItem}
                    />
                }
                {section === 'policies' &&
                    <ConfigSectionPolicies
                        config={config}
                        setConfig={setConfig}
                        selectedId={selectedId}
                        setSelectedId={setSelectedId}
                        generateId={generateId}
                        updateItem={updateItem}
                        updateNested={updateNested}
                        deleteItem={deleteItem}
                        renameItem={renameItem}
                    />
                }
                {section === 'secrets' &&
                    <ConfigSectionSecrets
                        config={config}
                        setConfig={setConfig}
                        selectedId={selectedId}
                        setSelectedId={setSelectedId}
                        generateId={generateId}
                        updateItem={updateItem}
                        deleteItem={deleteItem}
                        renameItem={renameItem}
                    />
                }
                {section === 'service' &&
                    <ConfigSectionService
                        config={config}
                        setConfig={setConfig}
                    />
                }
                {section === 'yaml' &&
                    <ConfigSectionYaml
                        config={config}
                        handleApply={handleApply}
                        isSaving={isSaving}
                        saveError={saveError}
                    />
                }
            </div>
        </div>
    );
}