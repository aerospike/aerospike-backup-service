import React, {useState} from 'react';
import {HardDrive, Plus} from 'lucide-react';
import {Button} from '@/components/ui/Button';
import {Input, Select} from '@/components/ui/Inputs';
import {api, DtoConfig, DtoStorage} from '@/api';
import {Card, SectionHeader} from './ConfigEditorShared';
import {ConnectivityCheckButton} from '@/components/ui/ConnectivityCheckButton';

interface ConfigSectionStorageProps {
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

export const ConfigSectionStorage = (
  { config, setConfig, selectedId, setSelectedId, generateId, updateItem, updateNested, deleteItem, renameItem }: ConfigSectionStorageProps
) => {
  const items = config.storage || {};
  const getStorageType = (s: DtoStorage) => {
      if (s.s3Storage) return "s3Storage";
      if (s.localStorage) return "localStorage";
      if (s.gcpStorage) return "gcpStorage";
      if (s.azureStorage) return "azureStorage";
      return "s3Storage";
  };

  const changeStorageType = (id: string, type: string) => {
      const defaults: any = {
          s3Storage: { bucket: "new-bucket", s3Region: "us-east-1" },
          localStorage: { path: "/backups" },
          gcpStorage: { bucketName: "new-bucket" },
          azureStorage: { containerName: "backup-container", endpoint: "" }
      };
      setConfig((prev: DtoConfig) => ({
          ...prev,
          storage: {
              ...prev.storage,
              [id]: { [type]: defaults[type] }
          }
      }));
  };

  return (
    <div className="flex h-full">
      <div className="w-1/3 border-r border-gray-800 p-4 flex flex-col">
        <Button
          className="mb-4 w-full justify-center"
          icon={Plus}
          onClick={() => {
            const id = generateId('storage');
            setConfig((p: DtoConfig) => ({...p, storage: {...p.storage, [id]: { s3Storage: { bucket: "my-bucket", s3Region: "us-east-1" } }}}));
            setSelectedId(id);
          }}
        >
          New Storage
        </Button>
        <div className="overflow-y-auto flex-1 custom-scroll">
          {Object.entries(items)
            .sort((a, b) => a[0].localeCompare(b[0]))
            .map(([k, v]: [string, DtoStorage]) => (
            <Card
              key={k}
              title={k}
              sub={getStorageType(v).replace('Storage','').toUpperCase()}
              active={selectedId === k}
              onClick={() => {
                setSelectedId(k);
              }}
              onDelete={() => deleteItem('storage', k)}
            />
          ))}
        </div>
      </div>
      <div className="w-2/3 p-6 overflow-y-auto custom-scroll">
        {selectedId && items[selectedId] ? (
          <div className="animate-in fade-in slide-in-from-right-4 duration-200">
             <div className="mb-6">
                <div className="flex items-end gap-4 mb-4">
                    <div className="flex-1">
                        <Input label="Storage Name" value={selectedId} onChange={(e: React.ChangeEvent<HTMLInputElement>) => renameItem('storage', selectedId, e.target.value)} />
                    </div>
                    <ConnectivityCheckButton
                        className="mb-3"
                        onCheck={async () => {
                            if (selectedId && items[selectedId]) {
                                await api.checkStorageConnectivity(items[selectedId]);
                            }
                        }}
                    />
                </div>

                <Select
                  label="Type"
                  value={getStorageType(items[selectedId])}
                  options={[
                      { label: "Amazon S3 / MinIO", value: "s3Storage" },
                      { label: "Local Filesystem", value: "localStorage" },
                      { label: "Google Cloud Storage", value: "gcpStorage" },
                      { label: "Azure Blob Storage", value: "azureStorage" },
                  ]}
                  onChange={(e: React.ChangeEvent<HTMLSelectElement>) => changeStorageType(selectedId, e.target.value)}
                />
             </div>

             {items[selectedId].s3Storage && (
                 <>
                   <SectionHeader title="S3 Configuration" icon={HardDrive} />
                   <div className="grid grid-cols-2 gap-4">
                      <Input label="Bucket" value={items[selectedId].s3Storage?.bucket || ''} onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateNested('storage', selectedId, 's3Storage', 'bucket', e.target.value)} />
                      <Input label="Region" value={items[selectedId].s3Storage?.s3Region || ''} onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateNested('storage', selectedId, 's3Storage', 's3Region', e.target.value)} />
                      <Input label="Path Prefix" value={items[selectedId].s3Storage?.path || ''} onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateNested('storage', selectedId, 's3Storage', 'path', e.target.value)} />
                      <Input label="Endpoint (Optional)" value={items[selectedId].s3Storage?.s3EndpointOverride || ''} onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateNested('storage', selectedId, 's3Storage', 's3EndpointOverride', e.target.value)} placeholder="e.g. localhost:9000" />
                   </div>
                   <SectionHeader title="Credentials" />
                   <div className="grid grid-cols-2 gap-4">
                      <Input label="Access Key ID" value={items[selectedId].s3Storage?.accessKeyId || ''} onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateNested('storage', selectedId, 's3Storage', 'accessKeyId', e.target.value)} />
                      <Input label="Secret Access Key" type="password" value={items[selectedId].s3Storage?.secretAccessKey || ''} onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateNested('storage', selectedId, 's3Storage', 'secretAccessKey', e.target.value)} />
                   </div>
                 </>
             )}

             {items[selectedId].localStorage && (
                 <>
                   <SectionHeader title="Local Filesystem" icon={HardDrive} />
                   <Input label="Root Path" value={items[selectedId].localStorage?.path || ''} onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateNested('storage', selectedId, 'localStorage', 'path', e.target.value)} />
                 </>
             )}

             {items[selectedId].gcpStorage && (
                 <>
                   <SectionHeader title="Google Cloud" icon={HardDrive} />
                   <Input label="Bucket Name" value={items[selectedId].gcpStorage?.bucketName || ''} onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateNested('storage', selectedId, 'gcpStorage', 'bucketName', e.target.value)} />
                   <Input label="Key File Path" value={items[selectedId].gcpStorage?.keyFilePath || ''} onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateNested('storage', selectedId, 'gcpStorage', 'keyFilePath', e.target.value)} />
                 </>
             )}

             {items[selectedId].azureStorage && (
                 <>
                   <SectionHeader title="Azure Blob" icon={HardDrive} />
                   <div className="grid grid-cols-2 gap-4">
                      <Input label="Container Name" value={items[selectedId].azureStorage?.containerName || ''} onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateNested('storage', selectedId, 'azureStorage', 'containerName', e.target.value)} />
                      <Input label="Account Name" value={items[selectedId].azureStorage?.accountName || ''} onChange={(e: React.ChangeEvent<HTMLInputElement>) => updateNested('storage', selectedId, 'azureStorage', 'accountName', e.target.value)} />
                   </div>
                 </>
             )}
          </div>
        ) : (
          <div className="flex flex-col items-center justify-center h-full text-gray-500">
              <HardDrive size={48} className="mb-4 opacity-20" />
              <p>Select a storage backend to configure</p>
          </div>
        )}
      </div>
    </div>
  );
};
