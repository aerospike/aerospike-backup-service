import React from 'react';
import {HardDrive} from 'lucide-react';
import {Input} from '@/components/ui/Inputs';
import {SectionHeader} from '../ConfigEditorShared';
import {DtoAzureStorage} from '@/api';

interface ConfigStorageAzureProps {
    data: DtoAzureStorage;
    onChange: (field: keyof DtoAzureStorage, value: any) => void;
}

export const ConfigStorageAzure = ({ data, onChange }: ConfigStorageAzureProps) => {
    return (
        <>
            <SectionHeader title="Azure Blob" icon={HardDrive} />
            <div className="grid grid-cols-2 gap-4">
                <Input label="Container Name" value={data.containerName || ''} onChange={(e: React.ChangeEvent<HTMLInputElement>) => onChange('containerName', e.target.value)} />
                <Input label="Account Name" value={data.accountName || ''} onChange={(e: React.ChangeEvent<HTMLInputElement>) => onChange('accountName', e.target.value)} />
            </div>
        </>
    );
};
