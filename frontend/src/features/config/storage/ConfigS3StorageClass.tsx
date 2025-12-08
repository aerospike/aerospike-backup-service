import React from 'react';
import {Input, Select} from '@/components/ui/Inputs';
import {SectionHeader} from '../ConfigEditorShared';
import {DtoS3StorageClass} from '@/api';

interface ConfigS3StorageClassProps {
    data: DtoS3StorageClass;
    onChange: (field: keyof DtoS3StorageClass, value: any) => void;
}

// These values are hardcoded from the Go DTO enums
const s3dataClassOptions = [
    { label: "STANDARD", value: "STANDARD" },
    { label: "GLACIER", value: "GLACIER" },
    { label: "STANDARD_IA", value: "STANDARD_IA" },
    { label: "ONEZONE_IA", value: "ONEZONE_IA" },
    { label: "INTELLIGENT_TIERING", value: "INTELLIGENT_TIERING" },
    { label: "DEEP_ARCHIVE", value: "DEEP_ARCHIVE" },
    { label: "OUTPOSTS", value: "OUTPOSTS" },
    { label: "GLACIER_IR", value: "GLACIER_IR" },
    { label: "SNOW", value: "SNOW" },
    { label: "EXPRESS_ONEZONE", value: "EXPRESS_ONEZONE" },
    { label: "None", value: "" }, // Represents x-nullable / omitempty
];

const s3metadataClassOptions = [
    { label: "STANDARD", value: "STANDARD" },
    { label: "STANDARD_IA", value: "STANDARD_IA" },
    { label: "INTELLIGENT_TIERING", value: "INTELLIGENT_TIERING" },
    { label: "EXPRESS_ONEZONE", value: "EXPRESS_ONEZONE" },
    { label: "ONEZONE_IA", value: "ONEZONE_IA" },
    { label: "OUTPOSTS", value: "OUTPOSTS" },
    { label: "None", value: "" }, // Represents x-nullable / omitempty
];


export const ConfigS3StorageClass = ({ data, onChange }: ConfigS3StorageClassProps) => {
    return (
        <>
            <SectionHeader title="S3 Storage Class" />
            <div className="grid grid-cols-2 gap-4">
                <Select
                    label="Data Class"
                    value={data.data || ''}
                    options={s3dataClassOptions}
                    onChange={(e: React.ChangeEvent<HTMLSelectElement>) => onChange('data', e.target.value)}
                    hint="Specifies the storage class for object data."
                />
                <Select
                    label="Metadata Class"
                    value={data.metadata || ''}
                    options={s3metadataClassOptions}
                    onChange={(e: React.ChangeEvent<HTMLSelectElement>) => onChange('metadata', e.target.value)}
                    hint="Specifies the storage class for metadata."
                />
            </div>
        </>
    );
};
