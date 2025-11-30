import React from 'react';
import {Key, Plus, Server, Trash2} from 'lucide-react';
import {Button} from '@/components/ui/Button';
import {Input, Select} from '@/components/ui/Inputs';
import {DtoAerospikeCluster, DtoConfig} from '@/api';
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
  { config, setConfig, selectedId, setSelectedId, generateId, updateItem, updateNested, deleteItem, renameItem }: ConfigSectionClustersProps
) => {
  const items = config.aerospikeClusters || {};
  return (
    <div className="flex h-full">
      <div className="w-1/3 border-r border-gray-800 p-4 flex flex-col">
        <Button
          className="mb-4 w-full justify-center"
          icon={Plus}
          onClick={() => {
            const id = generateId('cluster');
            setConfig((p: DtoConfig) => ({...p, aerospikeClusters: {...p.aerospikeClusters, [id]: { seedNodes: [{ hostName: "localhost", port: 3000 }], credentials: { user: "" } }}}));
            setSelectedId(id);
          }}
        >
          New Cluster
        </Button>
        <div className="overflow-y-auto flex-1 custom-scroll">
          {Object.entries(items).map(([k, v]: [string, DtoAerospikeCluster]) => (
            <Card
              key={k}
              title={k}
              sub={`${v.seedNodes?.length || 0} nodes`}
              active={selectedId === k}
              onClick={() => setSelectedId(k)}
              onDelete={() => deleteItem('aerospikeClusters', k)}
            />
          ))}
        </div>
      </div>
      <div className="w-2/3 p-6 overflow-y-auto custom-scroll">
        {selectedId && items[selectedId] ? (
          <div className="animate-in fade-in slide-in-from-right-4 duration-200">
             <div className="mb-6">
                <Input label="Cluster Name" value={selectedId} onChange={(e: React.ChangeEvent<HTMLInputElement>) => renameItem('aerospikeClusters', selectedId, e.target.value)} />
             </div>

             <SectionHeader title="Seed Nodes" icon={Server} />
             <div className="space-y-2 mb-6">
               {items[selectedId].seedNodes?.map((node: { hostName: string; port: number; }, idx: number) => (
                 <div key={idx} className="flex gap-2">
                    <Input className="flex-1" placeholder="Hostname" value={node.hostName} onChange={(e: React.ChangeEvent<HTMLInputElement>) => {
                      const nodes = [...items[selectedId].seedNodes || []];
                      nodes[idx].hostName = e.target.value;
                      updateItem('aerospikeClusters', selectedId, 'seedNodes', nodes);
                    }} />
                    <Input className="w-24" type="number" placeholder="Port" value={node.port} onChange={(e: React.ChangeEvent<HTMLInputElement>) => {
                      const nodes = [...items[selectedId].seedNodes || []];
                      nodes[idx].port = Number(e.target.value);
                      updateItem('aerospikeClusters', selectedId, 'seedNodes', nodes);
                    }} />
                    <Button variant="ghost" className="px-2 text-gray-500 hover:text-red-500" onClick={() => {
                       const nodes = (items[selectedId].seedNodes || []).filter((_: { hostName: string; port: number; }, i: number) => i !== idx);
                       updateItem('aerospikeClusters', selectedId, 'seedNodes', nodes);
                    }}><Trash2 size={16}/></Button>
                 </div>
               ))}
               <Button variant="secondary" className="w-full justify-center py-1 text-xs" onClick={() => {
                  const nodes = [...items[selectedId].seedNodes || [], { hostName: "localhost", port: 3000 }];
                  updateItem('aerospikeClusters', selectedId, 'seedNodes', nodes);
               }}>+ Add Node</Button>
             </div>

             <SectionHeader title="Authentication" icon={Key} />
             <Input label="User" value={items[selectedId].credentials?.user} onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateNested('aerospikeClusters', selectedId, 'credentials', 'user', e.target.value)} />
             <Input label="Password" type="password" value={items[selectedId].credentials?.password} onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateNested('aerospikeClusters', selectedId, 'credentials', 'password', e.target.value)} />
             <Select
               label="Auth Mode"
               value={items[selectedId].credentials?.authMode || "INTERNAL"}
               options={[{label: "INTERNAL", value: "INTERNAL"}, {label: "EXTERNAL", value: "EXTERNAL"}, {label: "PKI", value: "PKI"}]}
               onChange={(e: React.ChangeEvent<HTMLSelectElement>) => updateNested('aerospikeClusters', selectedId, 'credentials', 'authMode', e.target.value)}
             />

             <SectionHeader title="Advanced" />
             <div className="grid grid-cols-2 gap-4">
               <Input label="Max Parallel Scans" type="number" value={items[selectedId].maxParallelScans} onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateItem('aerospikeClusters', selectedId, 'maxParallelScans', Number(e.target.value))} />
               <Input label="Connection Timeout (ms)" type="number" value={items[selectedId].connTimeout} onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateItem('aerospikeClusters', selectedId, 'connTimeout', Number(e.target.value))} />
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
