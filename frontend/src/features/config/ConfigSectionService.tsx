import React from 'react';
import {Settings} from 'lucide-react';
import {Input, Select} from '@/components/ui/Inputs';
import {DtoConfig} from '@/api';
import {SectionHeader} from './ConfigEditorShared';

interface ConfigSectionServiceProps {
  config: DtoConfig;
  setConfig: React.Dispatch<React.SetStateAction<DtoConfig>>;
}

export const ConfigSectionService = (
  { config, setConfig }: ConfigSectionServiceProps
) => {
  const srv = config.service;
  const updateSrv = (parent: 'http' | 'logger', field: string, value: any) => {
      setConfig(p => ({
          ...p,
          service: { ...p.service, [parent]: { ...p.service?.[parent], [field]: value } }
      }))
  }
  return (
      <div className="max-w-2xl mx-auto p-6">
          <h2 className="text-xl font-bold mb-6 flex items-center gap-2"><Settings className="text-red-500"/> Service Configuration</h2>

          <SectionHeader title="HTTP Server" />
          <div className="grid grid-cols-2 gap-4">
              <Input label="Port" type="number" value={srv?.http?.port} onChange={e => updateSrv('http', 'port', Number(e.target.value))} />
              <Input label="Context Path" value={srv?.http?.contextPath} onChange={e => updateSrv('http', 'contextPath', e.target.value)} placeholder="/" />
          </div>

          <SectionHeader title="Logging" />
          <div className="grid grid-cols-2 gap-4">
              <Select
                label="Level"
                value={srv?.logger?.level}
                options={['DEBUG', 'INFO', 'WARN', 'ERROR'].map(l => ({label: l, value: l}))}
                onChange={e => updateSrv('logger', 'level', e.target.value)}
              />
              <Select
                label="Format"
                value={srv?.logger?.format}
                options={['PLAIN', 'JSON'].map(l => ({label: l, value: l}))}
                onChange={e => updateSrv('logger', 'format', e.target.value)}
              />
          </div>
      </div>
  );
};
