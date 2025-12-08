import React from 'react';
import {HardDrive} from 'lucide-react';
import {Input} from '@/components/ui/Inputs';
import {SectionHeader} from '../ConfigEditorShared';
import {DtoLocalStorage} from '@/api';

interface ConfigStorageLocalProps {
    data: DtoLocalStorage;
    onChange: (field: keyof DtoLocalStorage, value: any) => void;
}

export const ConfigStorageLocal = ({ data, onChange }: ConfigStorageLocalProps) => {
    return (
        <>
            <SectionHeader title="Local Filesystem" icon={HardDrive} />
            <Input label="Root Path" value={data.path || ''} onChange={(e: React.ChangeEvent<HTMLInputElement>) => onChange('path', e.target.value)} />
        </>
    );
};
