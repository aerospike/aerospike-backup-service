import * as allGenerated from './generated';
import {type DtoConfig} from './generated';

export * from './generated';

const backupApi = new allGenerated.BackupApi();
const configurationApi = new allGenerated.ConfigurationApi();
const restoreApi = new allGenerated.RestoreApi();
const systemApi = new allGenerated.SystemApi();

// Define LogEntry interface
export type LogEntry = allGenerated.LogLogEntry;

// We will replace the old Backup type with DtoBackupDetails and add the type property.
export type Backup = allGenerated.DtoBackupDetails & {
    type: 'Full' | 'Incremental';
};

export const api = {
    fetchConfig: async (): Promise<allGenerated.DtoConfig> => {
        return await configurationApi.readConfig({});
    },

    applyConfig: async (config: DtoConfig): Promise<Response> => {
        const response = await configurationApi.updateConfigRaw({dtoConfig: config});
        return response.raw;
    },

    fetchCurrentBackup: async (routine: string): Promise<allGenerated.DtoRoutineState> => {
        return await backupApi.getCurrentBackup({name: routine});
    },

    fetchHistory: async (routine: string): Promise<Backup[]> => {
        const mapBackupDetailsToBackup = (b: allGenerated.DtoBackupDetails, type: 'Full' | 'Incremental'): Backup => ({
            ...b,
            type: type,
        });

        try {
            const [fullBackups, incrementalBackups] = await Promise.all([
                backupApi.getFullBackupsForRoutine({name: routine}),
                backupApi.getIncrementalBackupsForRoutine({name: routine})
            ]);

            return [
                ...fullBackups.map((b) => mapBackupDetailsToBackup(b, 'Full')),
                ...incrementalBackups.map((b) => mapBackupDetailsToBackup(b, 'Incremental')),
            ];
        } catch (error) {
            console.error("Failed to fetch backup history:", error);
            throw new Error('Failed to fetch backup history');
        }
    },

    restoreBackup: async (request: allGenerated.DtoRestoreTimestampRequest): Promise<Response> => {
        const response = await restoreApi.restoreTimestampRaw({dtoRestoreTimestampRequest: request});
        return response.raw;
    },

    fetchRestoreJobs: async (): Promise<{ [key: string]: allGenerated.DtoRestoreJobStatus }> => {
        return await restoreApi.retrieveRestoreJobs({});
    },

    cancelRestore: async (jobId: number): Promise<void> => {
        await restoreApi.cancelRestore({jobId});
    },

    cancelBackup: async (routine: string): Promise<void> => {
        await backupApi.cancelCurrentBackup({name: routine});
    },

    checkClusterConnectivity: async (cluster: allGenerated.DtoAerospikeCluster): Promise<Record<string, string[]>> => {
        return await configurationApi.checkClusterConnectivity({dtoAerospikeCluster: cluster}) as unknown as Record<string, string[]>;
    },

    checkStorageConnectivity: async (storage: allGenerated.DtoStorage): Promise<string> => {
        return await configurationApi.checkStorageConnectivity({dtoStorage: storage});
    },

    checkSecretAgentConnectivity: async (secretAgent: allGenerated.DtoSecretAgent): Promise<string> => {
        return await configurationApi.checkSecretAgentConnectivity({dtoSecretAgent: secretAgent});
    },

    fetchLogs: async (): Promise<LogEntry[]> => {
        return await systemApi.logs();
    }
};