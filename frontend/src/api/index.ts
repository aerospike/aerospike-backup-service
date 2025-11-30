import type {DtoBackupDetails, DtoConfig, DtoRestoreTimestampRequest, DtoRoutineState} from './generated';
import {BackupApi, ConfigurationApi, RestoreApi,} from './generated';

export type {
    DtoConfig,
    DtoRoutineState,
    DtoBackupDetails,
    DtoRestoreTimestampRequest
};

export {
    BackupApi
};

const backupApi = new BackupApi();
const configurationApi = new ConfigurationApi();
const restoreApi = new RestoreApi();

// We will replace the old Backup type with DtoBackupDetails and add the type property.
export type Backup = DtoBackupDetails & {
    type: 'Full' | 'Incremental';
};

export const api = {
    fetchConfig: async (): Promise<DtoConfig> => {
        return await configurationApi.readConfig({});
    },

    applyConfig: async (config: DtoConfig): Promise<Response> => {
        const response = await configurationApi.updateConfigRaw({ dtoConfig: config });
        return response.raw;
    },

    fetchCurrentBackup: async (routine: string): Promise<DtoRoutineState> => {
        return await backupApi.getCurrentBackup({ name: routine });
    },

    fetchHistory: async (routine: string): Promise<Backup[]> => {
        const mapBackupDetailsToBackup = (b: DtoBackupDetails, type: 'Full' | 'Incremental'): Backup => ({
            ...b,
            type: type,
        });

        try {
            const [fullBackups, incrementalBackups] = await Promise.all([
                backupApi.getFullBackupsForRoutine({ name: routine }),
                backupApi.getIncrementalBackupsForRoutine({ name: routine })
            ]);

            const allBackups = [
                ...fullBackups.map((b) => mapBackupDetailsToBackup(b, 'Full')),
                ...incrementalBackups.map((b) => mapBackupDetailsToBackup(b, 'Incremental')),
            ];

            return allBackups;
        } catch (error) {
            console.error("Failed to fetch backup history:", error);
            throw new Error('Failed to fetch backup history');
        }
    },

    restoreBackup: async (routine: string, timestamp: number): Promise<Response> => {
        const request: DtoRestoreTimestampRequest = {
            routine: routine,
            time: timestamp
        };
        const response = await restoreApi.restoreTimestampRaw({ dtoRestoreTimestampRequest: request });
        return response.raw;
    }
};
