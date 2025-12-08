import React from 'react';
import {HardDrive} from 'lucide-react';
import {Input} from '@/components/ui/Inputs';
import {SectionHeader} from '../ConfigEditorShared';
import {DtoS3Storage} from '@/api';

interface ConfigStorageS3Props {
    data: DtoS3Storage;
    onChange: (field: keyof DtoS3Storage, value: any) => void;
}

export const ConfigStorageS3 = ({ data, onChange }: ConfigStorageS3Props) => {
    return (
        <>
            <SectionHeader title="S3 Configuration" icon={HardDrive} />
            <div className="grid grid-cols-2 gap-4">
                <Input label="Bucket" value={data.bucket || ''} onChange={(e: React.ChangeEvent<HTMLInputElement>) => onChange('bucket', e.target.value)} />
                <Input label="Region" value={data.s3Region || ''} onChange={(e: React.ChangeEvent<HTMLInputElement>) => onChange('s3Region', e.target.value)} />
                <Input label="Path Prefix" value={data.path || ''} onChange={(e: React.ChangeEvent<HTMLInputElement>) => onChange('path', e.target.value)} />
                <Input label="Endpoint (Optional)" value={data.s3EndpointOverride || ''} onChange={(e: React.ChangeEvent<HTMLInputElement>) => onChange('s3EndpointOverride', e.target.value)} placeholder="e.g. localhost:9000" />
            </div>
            <SectionHeader title="Credentials" />
            <div className="grid grid-cols-2 gap-4">
                <Input label="Access Key ID" value={data.accessKeyId || ''} onChange={(e: React.ChangeEvent<HTMLInputElement>) => onChange('accessKeyId', e.target.value)} />
                <Input label="Secret Access Key" type="password" value={data.secretAccessKey || ''} onChange={(e: React.ChangeEvent<HTMLInputElement>) => onChange('secretAccessKey', e.target.value)} />
            </div>
        </>
    );
};
