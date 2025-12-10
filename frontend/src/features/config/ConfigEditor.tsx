import React, {useState} from 'react';
import {motion, AnimatePresence} from 'framer-motion';
import {Clock, HardDrive, Key, Play, Server, Settings, Shield} from 'lucide-react';
import type {DtoConfig} from '@/api';
import {api, ResponseError} from '@/api';
import {ConfigSectionRoutines} from './ConfigSectionRoutines';
import {ConfigSectionClusters} from './ConfigSectionClusters';
import {ConfigSectionStorage} from './ConfigSectionStorage';
import {ConfigSectionPolicies} from './ConfigSectionPolicies';
import {ConfigSectionSecretAgents} from './ConfigSectionSecretAgents';
import {ConfigSectionService} from './ConfigSectionService';
import {ConfigSectionYaml} from './ConfigSectionYaml';

// --- Main Editor ---

interface ConfigEditorProps {
    config: DtoConfig;
    setConfig: React.Dispatch<React.SetStateAction<DtoConfig>>;
}

type SectionId = 'routines' | 'clusters' | 'storage' | 'policies' | 'secrets' | 'service' | 'yaml';

export default function ConfigEditor({config, setConfig}: ConfigEditorProps) {
    const [section, setSection] = useState<SectionId>(() => {
        return (localStorage.getItem('abs_config_section') as SectionId) || 'routines';
    });
    
    // Initialize selectedId based on the loaded section
    const [selectedId, setSelectedId] = useState<string | null>(() => {
        const initialSection = (localStorage.getItem('abs_config_section') as SectionId) || 'routines';
        return localStorage.getItem(`abs_sel_${initialSection}`);
    });
    
    const [isSaving, setIsSaving] = useState(false);
    const [saveError, setSaveError] = useState<string | null>(null);

    // Persist current section
    React.useEffect(() => {
        localStorage.setItem('abs_config_section', section);
    }, [section]);

    // Persist selectedId for the current section
    React.useEffect(() => {
        if (selectedId) {
            localStorage.setItem(`abs_sel_${section}`, selectedId);
        } else {
            localStorage.removeItem(`abs_sel_${section}`);
        }
    }, [selectedId, section]);

    const handleSectionChange = (newSection: SectionId) => {
        if (newSection === section) return;
        
        // Load the saved selection for the new section
        const prevSelection = localStorage.getItem(`abs_sel_${newSection}`);
        
        // Batch update to avoid "render with old ID in new section" race conditions
        setSection(newSection);
        setSelectedId(prevSelection);
    };

    const handleApply = async () => {
        setIsSaving(true);
        setSaveError(null);
        try {
            await api.applyConfig(config);
            // Maybe show a success toast here
        } catch (e: any) {
            let message = e.message || 'Failed to save.';
            if (e instanceof ResponseError) {
                try {
                    const text = await e.response.text();
                    if (text) {
                        message = text;
                    }
                } catch (readErr) {
                    console.warn("Failed to read error response body", readErr);
                }
            }
            setSaveError(message);
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
            onClick={() => handleSectionChange(id)}
            className={`flex items-center gap-3 w-full px-3 py-2 rounded-md transition-all text-sm mb-1 ${
                section === id ? 'bg-aerospike-light-blue text-gray-900 border-l-2 border-aerospike-border-blue' : 'text-gray-600 hover:bg-gray-100 hover:text-gray-900'
            }`}
        >
            <Icon size={16}/>
            <span>{label}</span>
        </button>
    );

    return (
        <div className="grid grid-cols-12 h-full">
            <div className="col-span-2 border-r border-gray-200 bg-gray-50 flex flex-col py-4">
                <div className="px-4 mb-2">
                    <h3 className="text-xs font-bold text-gray-500 uppercase tracking-wider">Definitions</h3>
                </div>
                <NavItem id="routines" label="Routines" icon={Clock}/>
                <NavItem id="clusters" label="Clusters" icon={Server}/>
                <NavItem id="storage" label="Storage" icon={HardDrive}/>
                <NavItem id="policies" label="Policies" icon={Shield}/>
                <NavItem id="secrets" label="Secrets" icon={Key}/>
                <NavItem id="service" label="Service" icon={Settings}/>
                <div className="my-4 border-t border-gray-200"></div>
                <NavItem id="yaml" label="Apply Config" icon={Play}/>
            </div>
            <div className="col-span-10 bg-white h-full overflow-hidden">
                <AnimatePresence mode="wait">
                    {section === 'routines' &&
                        <motion.div
                            key="routines"
                            initial={{x: 10, opacity: 0}}
                            animate={{x: 0, opacity: 1}}
                            exit={{x: -10, opacity: 0}}
                            transition={{duration: 0.2}}
                            className="h-full"
                        >
                            <ConfigSectionRoutines
                                config={config}
                                setConfig={setConfig}
                                selectedId={selectedId}
                                setSelectedId={setSelectedId}
                                generateId={generateId}
                                updateItem={updateItem}
                                deleteItem={deleteItem}
                                renameItem={renameItem}
                            />
                        </motion.div>}
                    {section === 'clusters' &&
                        <motion.div
                            key="clusters"
                            initial={{x: 10, opacity: 0}}
                            animate={{x: 0, opacity: 1}}
                            exit={{x: -10, opacity: 0}}
                            transition={{duration: 0.2}}
                            className="h-full"
                        >
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
                        </motion.div>
                    }
                    {section === 'storage' &&
                        <motion.div
                            key="storage"
                            initial={{x: 10, opacity: 0}}
                            animate={{x: 0, opacity: 1}}
                            exit={{x: -10, opacity: 0}}
                            transition={{duration: 0.2}}
                        >
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
                        </motion.div>
                    }
                    {section === 'policies' &&
                        <motion.div
                            key="policies"
                            initial={{x: 10, opacity: 0}}
                            animate={{x: 0, opacity: 1}}
                            exit={{x: -10, opacity: 0}}
                            transition={{duration: 0.2}}
                        >
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
                        </motion.div>
                    }
                    {section === 'secrets' &&
                        <motion.div
                            key="secrets"
                            initial={{x: 10, opacity: 0}}
                            animate={{x: 0, opacity: 1}}
                            exit={{x: -10, opacity: 0}}
                            transition={{duration: 0.2}}
                        >
                            <ConfigSectionSecretAgents
                                config={config}
                                setConfig={setConfig}
                                selectedId={selectedId}
                                setSelectedId={setSelectedId}
                                generateId={generateId}
                                updateItem={updateItem}
                                updateNested={updateNested} // ConfigSectionSecretAgents requires updateNested
                                deleteItem={deleteItem}
                                renameItem={renameItem}
                            />
                        </motion.div>
                    }
                    {section === 'service' &&
                        <motion.div
                            key="service"
                            initial={{x: 10, opacity: 0}}
                            animate={{x: 0, opacity: 1}}
                            exit={{x: -10, opacity: 0}}
                            transition={{duration: 0.2}}
                        >
                            <ConfigSectionService
                                config={config}
                                setConfig={setConfig}
                            />
                        </motion.div>
                    }
                    {section === 'yaml' &&
                        <motion.div
                            key="yaml"
                            initial={{x: 10, opacity: 0}}
                            animate={{x: 0, opacity: 1}}
                            exit={{x: -10, opacity: 0}}
                            transition={{duration: 0.2}}
                        >
                            <ConfigSectionYaml
                                config={config}
                                handleApply={handleApply}
                                isSaving={isSaving}
                                saveError={saveError}
                            />
                        </motion.div>
                    }
                </AnimatePresence>
            </div>
        </div>
    );
}