import React from 'react';
import {Key, Plus} from 'lucide-react';
import {Button} from '@/components/ui/Button';
import {Input} from '@/components/ui/Inputs';
import {DtoConfig, DtoSecretAgent} from '@/api';
import {Card, SectionHeader} from './ConfigEditorShared';

interface ConfigSectionSecretsProps {
  config: DtoConfig;
  setConfig: React.Dispatch<React.SetStateAction<DtoConfig>>;
  selectedId: string | null;
  setSelectedId: (id: string | null) => void;
  generateId: (prefix: string) => string;
  updateItem: (sectionKey: keyof DtoConfig, id: string, field: string, value: any) => void;
  deleteItem: (sectionKey: keyof DtoConfig, id: string) => void;
  renameItem: (sectionKey: keyof DtoConfig, oldId: string, newId: string) => void;
}

export const ConfigSectionSecrets = (
  { config, setConfig, selectedId, setSelectedId, generateId, updateItem, deleteItem, renameItem }: ConfigSectionSecretsProps
) => {
  const items = config.secretAgents || {};
  return (
    <div className="flex h-full">
      <div className="w-1/3 border-r border-gray-800 p-4 flex flex-col">
        <Button
          className="mb-4 w-full justify-center"
          icon={Plus}
          onClick={() => {
            const id = generateId('agent');
            setConfig((p: DtoConfig) => ({...p, secretAgents: {...p.secretAgents, [id]: { address: "localhost", port: 8081, connectionType: "tcp" }}}));
            setSelectedId(id);
          }}
        >
          New Agent
        </Button>
        <div className="overflow-y-auto flex-1 custom-scroll">
          {Object.entries(items).map(([k, v]: [string, DtoSecretAgent]) => (
            <Card
              key={k}
              title={k}
              sub={`${v.address}:${v.port}`}
              active={selectedId === k}
              onClick={() => setSelectedId(k)}
              onDelete={() => deleteItem('secretAgents', k)}
            />
          ))}
        </div>
      </div>
      <div className="w-2/3 p-6 overflow-y-auto custom-scroll">
        {selectedId && items[selectedId] ? (
          <div className="animate-in fade-in slide-in-from-right-4 duration-200">
             <div className="mb-6">
                <Input label="Agent Name" value={selectedId} onChange={(e: React.ChangeEvent<HTMLInputElement>) => renameItem('secretAgents', selectedId, e.target.value)} />
             </div>
             <SectionHeader title="Connection Info" icon={Key} />
             <Input label="Address" value={items[selectedId].address} onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateItem('secretAgents', selectedId, 'address', e.target.value)} />
             <Input label="Port" type="number" value={items[selectedId].port || ''} onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateItem('secretAgents', selectedId, 'port', Number(e.target.value))} />
             <Input label="TLS CA File" value={items[selectedId].tlsCaFile || ''} onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateItem('secretAgents', selectedId, 'tlsCaFile', e.target.value)} />
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
