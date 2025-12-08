import React from 'react';
import {Clock, Plus} from 'lucide-react';
import {Button} from '@/components/ui/Button';
import {Input, MultiSelect, Select} from '@/components/ui/Inputs';
import {CronInput} from '@/components/ui/CronInput';
import {DtoBackupRoutine, DtoConfig, api} from '@/api';
import {Card, SectionHeader} from './ConfigEditorShared';

interface ConfigSectionRoutinesProps {
  config: DtoConfig;
  setConfig: React.Dispatch<React.SetStateAction<DtoConfig>>;
  selectedId: string | null;
  setSelectedId: (id: string | null) => void;
  generateId: (prefix: string) => string;
  updateItem: (sectionKey: keyof DtoConfig, id: string, field: string, value: any) => void;
  deleteItem: (sectionKey: keyof DtoConfig, id: string) => void;
  renameItem: (sectionKey: keyof DtoConfig, oldId: string, newId: string) => void;
}

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

export const ConfigSectionRoutines = (
  { config, setConfig, selectedId, setSelectedId, generateId, updateItem, deleteItem, renameItem }: ConfigSectionRoutinesProps
) => {
  const items = config.backupRoutines || {};
  const [connectivity, setConnectivity] = React.useState<Record<string, string[]>>({});
  const currentCluster = selectedId && items[selectedId] ? items[selectedId].sourceCluster : "";

  React.useEffect(() => {
    if (currentCluster && config.aerospikeClusters && config.aerospikeClusters[currentCluster]) {
      api.checkClusterConnectivity(config.aerospikeClusters[currentCluster])
          .then((data) => setConnectivity(data as any))
          .catch(e => console.error("Failed to check connectivity", e));
    } else {
      setConnectivity({});
    }
  }, [currentCluster, config.aerospikeClusters]);

  return (
    <div className="flex h-full">
      <div className="w-1/3 border-r border-gray-800 p-4 flex flex-col h-full">
        <Button
          className="mb-4 w-full justify-center"
          icon={Plus}
          onClick={() => {
            const id = generateId('routine');
            setConfig((p: DtoConfig) => ({...p, backupRoutines: {...p.backupRoutines, [id]: { sourceCluster: "", storage: "", intervalCron: "@daily", namespaces: [] }}}));
            setSelectedId(id);
          }}
        >
          New Routine
        </Button>
        <div className="overflow-y-auto flex-1 custom-scroll">
          {Object.entries(items)
            .sort((a, b) => a[0].localeCompare(b[0]))
            .map(([k, v]: [string, DtoBackupRoutine]) => (
            <Card
              key={k}
              title={k}
              active={selectedId === k}
              onClick={() => setSelectedId(k)}
              onDelete={() => deleteItem('backupRoutines', k)}
            />
          ))}
        </div>
      </div>
      <div className="w-2/3 p-6 overflow-y-auto custom-scroll">
        {selectedId && items[selectedId] ? (
          <div className="animate-in fade-in slide-in-from-right-4 duration-200">
             <div className="mb-6">
                <Input label="Routine Name" value={selectedId} onChange={(e: React.ChangeEvent<HTMLInputElement>) => renameItem('backupRoutines', selectedId, e.target.value)} />
             </div>

             <SectionHeader title="Core Bindings" icon={Clock} />
             <div className="grid grid-cols-2 gap-4">
                <Select
                  label="Source Cluster"
                  value={items[selectedId].sourceCluster}
                  options={[{label: "Select...", value: ""}, ...Object.keys(config.aerospikeClusters || {}).map(k => ({ label: k, value: k }))]}
                  onChange={(e: React.ChangeEvent<HTMLSelectElement>) => updateItem('backupRoutines', selectedId, 'sourceCluster', e.target.value)}
                />
                <Select
                  label="Storage"
                  value={items[selectedId].storage}
                  options={[{label: "Select...", value: ""}, ...Object.keys(config.storage || {}).map(k => ({ label: k, value: k }))]}
                  onChange={(e: React.ChangeEvent<HTMLSelectElement>) => updateItem('backupRoutines', selectedId, 'storage', e.target.value)}
                />
                <Select
                  label="Policy (Optional)"
                  value={items[selectedId].backupPolicy || ''}
                  options={[{label: "None (Default)", value: ""}, ...Object.keys(config.backupPolicies || {}).map(k => ({ label: k, value: k }))]}
                  onChange={(e: React.ChangeEvent<HTMLSelectElement>) => updateItem('backupRoutines', selectedId, 'backupPolicy', e.target.value)}
                />
                <Select
                  label="Secret Agent (Optional)"
                  value={(items[selectedId] as any).secretAgent || ''}
                  options={[{label: "None", value: ""}, ...Object.keys(config.secretAgents || {}).map(k => ({ label: k, value: k }))]}
                  onChange={(e: React.ChangeEvent<HTMLSelectElement>) => updateItem('backupRoutines', selectedId, 'secretAgent', e.target.value)}
                />
             </div>

             <SectionHeader title="Scheduling" />
             <div className="grid grid-cols-2 gap-4">
                <CronInput label="Full Backup Cron" value={items[selectedId].intervalCron} onChange={(value) => updateItem('backupRoutines', selectedId, 'intervalCron', value)} placeholder="e.g. 0 0 12 * * ?" />
                <CronInput label="Incremental Cron" value={items[selectedId].incrIntervalCron || ''} onChange={(value) => updateItem('backupRoutines', selectedId, 'incrIntervalCron', value)} placeholder="Optional" />
             </div>

             <SectionHeader title="Scope" />
             <MultiSelect
               label="Namespaces"
               value={items[selectedId].namespaces || []}
               onChange={(newValues) => updateItem('backupRoutines', selectedId, 'namespaces', newValues)}
               options={Object.keys(connectivity)}
               placeholder="All namespaces if empty"
             />

             <MultiSelect
               label="Set List"
               value={items[selectedId].setList || []}
               onChange={(newValues) => updateItem('backupRoutines', selectedId, 'setList', newValues)}
               options={Array.from(new Set(
                  (items[selectedId].namespaces && items[selectedId].namespaces.length > 0
                      ? items[selectedId].namespaces
                      : Object.keys(connectivity)
                  ).flatMap(ns => connectivity[ns] || [])
               ))}
               placeholder="Optional"
             />

             <MultiSelect
               label="Bin List"
               value={items[selectedId].binList || []}
               onChange={(newValues) => updateItem('backupRoutines', selectedId, 'binList', newValues)}
               options={[]}
               placeholder="Optional"
             />

             <Checkbox
               label="Disable Routine"
               checked={items[selectedId].disabled || false}
               onChange={v => updateItem('backupRoutines', selectedId, 'disabled', v)}
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