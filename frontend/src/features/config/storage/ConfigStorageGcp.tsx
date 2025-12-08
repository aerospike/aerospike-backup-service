import React from 'react';
import {HardDrive} from 'lucide-react';
import {Input} from '@/components/ui/Inputs';
import {SectionHeader} from '../ConfigEditorShared';
import {DtoGcpStorage} from '@/api';

interface ConfigStorageGcpProps {
    data: DtoGcpStorage;
    onChange: (field: keyof DtoGcpStorage, value: any) => void;
}

export const ConfigStorageGcp = ({ data, onChange }: ConfigStorageGcpProps) => {
    return (
        <>
            <SectionHeader title="Google Cloud" icon={HardDrive} />
            <Input label="Bucket Name" value={data.bucketName || ''} onChange={(e: React.ChangeEvent<HTMLInputElement>) => onChange('bucketName', e.target.value)} />
            <Input label="Key File Path" value={data.keyFilePath || ''} onChange={(e: React.ChangeEvent<HTMLInputElement>) => onChange('keyFilePath', e.target.value)} />
        </>
    );
};
