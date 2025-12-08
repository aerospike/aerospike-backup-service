import React from 'react';
import {HardDrive} from 'lucide-react';
import {Input, Select} from '@/components/ui/Inputs';
import {SectionHeader} from '../ConfigEditorShared';
import {DtoS3Storage, DtoS3StorageClass, DtoS3StorageS3LogLevelEnum} from '@/api';
import {ConfigS3StorageClass} from './ConfigS3StorageClass';

interface ConfigStorageS3Props {
    data: DtoS3Storage;
    onChange: (field: keyof DtoS3Storage, value: any) => void;
    secretAgentOptions: { label: string, value: string }[];
}

const s3LogLevelOptions = Object.values(DtoS3StorageS3LogLevelEnum).map(val => ({
    label: val,
    value: val
}));

export const ConfigStorageS3 = ({data, onChange, secretAgentOptions}: ConfigStorageS3Props) => {
    const handleStorageClassChange = (field: keyof DtoS3StorageClass, value: any) => {
        onChange('storageClass', {...data.storageClass, [field]: value});
    };

    // Calculate if credentials are mismatched for validation hint
    const isCredentialMismatched = (!!data.accessKeyId !== !!data.secretAccessKey);

    return (
        <>
            <SectionHeader title="S3 Configuration" icon={HardDrive}/>
            <div className="grid grid-cols-2 gap-4">
                <Input label="Bucket" value={data.bucket || ''}
                       onChange={(e: React.ChangeEvent<HTMLInputElement>) => onChange('bucket', e.target.value)}
                       hint="Required: The S3 bucket name."/>
                <Input label="Region" value={data.s3Region || ''}
                       onChange={(e: React.ChangeEvent<HTMLInputElement>) => onChange('s3Region', e.target.value)}
                       hint="Required: The S3 region string (e.g., eu-central-1)."/>
                <Input label="Path Prefix" value={data.path || ''}
                       onChange={(e: React.ChangeEvent<HTMLInputElement>) => onChange('path', e.target.value)}
                       hint="The root path for the backup repository within the bucket."/>
                <Input label="S3 Profile" value={data.s3Profile || ''}
                       onChange={(e: React.ChangeEvent<HTMLInputElement>) => onChange('s3Profile', e.target.value)}
                       hint="The S3 profile name (AWS S3 optional)."/>
                <Input label="Endpoint (Optional)" value={data.s3EndpointOverride || ''}
                       onChange={(e: React.ChangeEvent<HTMLInputElement>) => onChange('s3EndpointOverride', e.target.value)}
                       placeholder="e.g. http://host.docker.internal:9000"
                       hint="An alternative endpoint for the S3 SDK to communicate."/>
                <Select
                    label="S3 Log Level"
                    value={data.s3LogLevel || 'FATAL'}
                    options={s3LogLevelOptions}
                    onChange={(e: React.ChangeEvent<HTMLSelectElement>) => onChange('s3LogLevel', e.target.value as DtoS3StorageS3LogLevelEnum)}
                    hint="The log level of the AWS S3 SDK."
                />
                <Input
                    label="Min Part Size (bytes)"
                    type="number"
                    value={data.minPartSize || ''}
                    onChange={(e: React.ChangeEvent<HTMLInputElement>) => onChange('minPartSize', Number(e.target.value))}
                    hint="Minimum size of individual S3 UploadParts (at least 5MB/5242880 bytes)."
                />
                <Input
                    label="Max Concurrent Connections"
                    type="number"
                    value={data.maxAsyncConnections || ''}
                    onChange={(e: React.ChangeEvent<HTMLInputElement>) => onChange('maxAsyncConnections', Number(e.target.value))}
                    hint="Maximum number of simultaneous requests from S3 (must be non-negative)."
                />
            </div>
            <SectionHeader title="Credentials"/>
            <div className="grid grid-cols-3 gap-4">
                <Input
                    label="Access Key ID"
                    value={data.accessKeyId || ''}
                    onChange={(e: React.ChangeEvent<HTMLInputElement>) => onChange('accessKeyId', e.target.value)}
                    hint="Access Key ID for authentication with S3 StaticCredentialsProvider. Can be a path in secret agent or an actual value."
                    description={isCredentialMismatched ? "Both Access Key ID and Secret Access Key must be provided, or neither." : undefined}
                />
                <Input
                    label="Secret Access Key"
                    type="password"
                    value={data.secretAccessKey || ''}
                    onChange={(e: React.ChangeEvent<HTMLInputElement>) => onChange('secretAccessKey', e.target.value)}
                    hint="Secret Access Key for authentication with S3 StaticCredentialsProvider. Can be a path in secret agent or an actual value."
                    description={isCredentialMismatched ? "Both Access Key ID and Secret Access Key must be provided, or neither." : undefined}
                />
                <Select
                    label="Secret Agent (Optional)"
                    value={data.secretAgentName || ''}
                    options={secretAgentOptions}
                    onChange={(e: React.ChangeEvent<HTMLSelectElement>) => onChange('secretAgentName', e.target.value)}
                    hint="Reference to a configured Secret Agent for credentials."
                />
            </div>

            <ConfigS3StorageClass data={data.storageClass || {}} onChange={handleStorageClassChange}/>
        </>
    );
};

