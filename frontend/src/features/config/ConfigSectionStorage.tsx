import React, {useState} from 'react';
import {HardDrive, Plus, Cloud} from 'lucide-react';
import {Button} from '@/components/ui/Button';
import {Input} from '@/components/ui/Inputs';
import {api, DtoConfig, DtoStorage} from '@/api';
import {Card, SectionHeader} from './ConfigEditorShared';
import {ConnectivityCheckButton} from '@/components/ui/ConnectivityCheckButton';
import {ConfigStorageS3} from './storage/ConfigStorageS3';
import {ConfigStorageLocal} from './storage/ConfigStorageLocal';
import {ConfigStorageGcp} from './storage/ConfigStorageGcp';
import {ConfigStorageAzure} from './storage/ConfigStorageAzure';

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
  const secretAgentOptions = [{label: "None", value: ""}, ...Object.keys(config.secretAgents || {}).map(k => ({ label: k, value: k }))];
  
  const getActiveStorageType = (s: DtoStorage) => {
      if (s.s3Storage) return "s3Storage";
      if (s.localStorage) return "localStorage";
      if (s.gcpStorage) return "gcpStorage";
      if (s.azureStorage) return "azureStorage";
      return "s3Storage"; // Default to S3 if none found
  };

  const [activeTab, setActiveTab] = useState<string>(() => {
    if (selectedId && items[selectedId]) {
      return getActiveStorageType(items[selectedId]);
    }
    return "s3Storage"; // Default tab
  });

  // Effect to update activeTab when selectedId changes or config changes
  React.useEffect(() => {
    if (selectedId && items[selectedId]) {
      setActiveTab(getActiveStorageType(items[selectedId]));
    } else {
      setActiveTab("s3Storage"); // Reset to default if no item selected
    }
  }, [selectedId, items]);


  const changeStorageType = (id: string, newType: string) => {
      const defaults: any = {
          s3Storage: { bucket: "new-bucket", s3Region: "us-east-1" },
          localStorage: { path: "/backups" },
          gcpStorage: { bucketName: "new-bucket" },
          azureStorage: { containerName: "backup-container", endpoint: "" }
      };

      // Ensure only one storage type object exists and replace if necessary
      const currentStorage: DtoStorage = items[id] || {};
      const newStorage: DtoStorage = {};
      newStorage[newType as keyof DtoStorage] = defaults[newType];

      setConfig((prev: DtoConfig) => ({
          ...prev,
          storage: {
              ...prev.storage,
              [id]: newStorage
          }
      }));
      setActiveTab(newType); // Update the active tab directly
  };

  // Generic updater for specific storage type fields
  const updateStorageField = (storageTypeKey: keyof DtoStorage, field: string, value: any) => {
      if (!selectedId) return;
      updateNested('storage', selectedId, storageTypeKey as string, field, value);
  };

  const storageTypeOptions = [
      { id: "s3Storage", label: "Amazon S3 / MinIO", icon: Cloud },
      { id: "localStorage", label: "Local Filesystem", icon: HardDrive },
      { id: "gcpStorage", label: "Google Cloud Storage", icon: Cloud },
      { id: "azureStorage", label: "Azure Blob Storage", icon: Cloud },
  ];

  const currentItem = selectedId && items[selectedId];
  const currentStorageTypeKey = currentItem ? getActiveStorageType(currentItem) : null;

  return (
    <div className="flex h-full">
      <div className="w-1/3 border-r border-gray-800 p-4 flex flex-col">
        <Button
          className="mb-4 w-full justify-center"
          icon={Plus}
          onClick={() => {
            const id = generateId('storage');
            // Default new storage to S3, which is also the default active tab
            setConfig((p: DtoConfig) => ({...p, storage: {...p.storage, [id]: { s3Storage: { bucket: "my-bucket", s3Region: "us-east-1" } }}}));
            setSelectedId(id);
            setActiveTab("s3Storage"); // Ensure new item opens with S3 tab active
          }}
        >
          New Storage
        </Button>
        <div className="overflow-y-auto flex-1 custom-scroll">
          {Object.entries(items)
            .sort((a, b) => a[0].localeCompare(b[0]))
            .map(([k, v]: [string, DtoStorage]) => {
              const IconComponent = storageTypeOptions.find(opt => opt.id === getActiveStorageType(v))?.icon || HardDrive; // Default to HardDrive
              return (
                <Card
                  key={k}
                  title={k}
                  sub={getActiveStorageType(v).replace('Storage','').toUpperCase()}
                  active={selectedId === k}
                  onClick={() => {
                    setSelectedId(k);
                  }}
                  onDelete={() => deleteItem('storage', k)}
                  icon={IconComponent}
                />
              );
            })}
        </div>
      </div>
      <div className="w-2/3 p-6 overflow-y-auto custom-scroll">
        {selectedId && currentItem ? (
          <div className="animate-in fade-in slide-in-from-right-4 duration-200">
             <div className="mb-6">
                <div className="flex items-end gap-4 mb-4">
                    <div className="flex-1">
                        <Input label="Storage Name" value={selectedId} onChange={(e: React.ChangeEvent<HTMLInputElement>) => renameItem('storage', selectedId, e.target.value)} />
                    </div>
                    {currentStorageTypeKey && currentItem[currentStorageTypeKey] && (
                        <ConnectivityCheckButton
                            className="mb-3"
                            onCheck={async () => {
                                await api.checkStorageConnectivity(currentItem);
                            }}
                        />
                    )}
                </div>

                {/* Tab-based selection for Storage Type */}
                <nav className="flex bg-gray-100 p-1 rounded-lg border border-gray-200 text-sm mb-4">
                    {storageTypeOptions.map((option) => {
                        const Icon = option.icon;
                        return (
                            <button
                                key={option.id}
                                onClick={() => changeStorageType(selectedId, option.id)}
                                className={`px-3 py-1.5 rounded-md font-medium transition-colors flex-1 text-center flex items-center justify-center gap-1 ${
                                    activeTab === option.id
                                        ? 'bg-white text-gray-900 shadow-sm'
                                        : 'text-gray-500 hover:text-gray-900'
                                }`}
                            >
                                {Icon && <Icon size={16} />}
                                <span>{option.label}</span>
                            </button>
                        );
                    })}
                </nav>
             </div>

             {activeTab === "s3Storage" && currentItem.s3Storage && (
                 <ConfigStorageS3 
                    data={currentItem.s3Storage} 
                    onChange={(field, value) => updateStorageField('s3Storage', field, value)}
                    secretAgentOptions={secretAgentOptions}
                 />
             )}

             {activeTab === "localStorage" && currentItem.localStorage && (
                 <ConfigStorageLocal 
                    data={currentItem.localStorage} 
                    onChange={(field, value) => updateStorageField('localStorage', field, value)} 
                 />
             )}

             {activeTab === "gcpStorage" && currentItem.gcpStorage && (
                 <ConfigStorageGcp 
                    data={currentItem.gcpStorage} 
                    onChange={(field, value) => updateStorageField('gcpStorage', field, value)} 
                 />
             )}

             {activeTab === "azureStorage" && currentItem.azureStorage && (
                 <ConfigStorageAzure 
                    data={currentItem.azureStorage} 
                    onChange={(field, value) => updateStorageField('azureStorage', field, value)} 
                 />
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
