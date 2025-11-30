import { AppConfig, CurrentBackup, Backup } from '../types';

const RealApi = {
    // --- Config Endpoints ---
    fetchConfig: async (): Promise<AppConfig> => {
        const res = await fetch('/v1/config');
        if (!res.ok) throw new Error('Failed to fetch configuration');
        return res.json();
    },
    applyConfig: async (config: AppConfig): Promise<Response> => {
        const res = await fetch('/v1/config', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(config)
        });
        if (!res.ok) throw new Error('Failed to apply configuration');
        return res;
    },

    // --- Monitoring Endpoints ---
    fetchCurrentBackup: async (routine: string): Promise<CurrentBackup> => {
        const res = await fetch(`/v1/backups/currentBackup/${routine}`);
        if (!res.ok) throw new Error('Failed to fetch current backup');
        return res.json();
    },
    fetchHistory: async (routine: string): Promise<Backup[]> => {
        // Based on the new API, we need to fetch full and incremental backups separately
        // and then combine them into a single list for the UI.

        const mapBackupDetailsToBackup = (b: any, type: 'Full' | 'Incremental'): Backup => ({
            ...b,
            type: type,
            status: 'Success', // Assuming finished backups are successful
        });

        try {
            const [fullBackupsRes, incrementalBackupsRes] = await Promise.all([
                fetch(`/v1/backups/full/${routine}`),
                fetch(`/v1/backups/incremental/${routine}`)
            ]);

            if (!fullBackupsRes.ok) throw new Error(`Failed to fetch full backups: ${fullBackupsRes.statusText}`);
            if (!incrementalBackupsRes.ok) throw new Error(`Failed to fetch incremental backups: ${incrementalBackupsRes.statusText}`);

            const fullBackups = await fullBackupsRes.json();
            const incrementalBackups = await incrementalBackupsRes.json();

            const allBackups = [
                ...fullBackups.map((b: any) => mapBackupDetailsToBackup(b, 'Full')),
                ...incrementalBackups.map((b: any) => mapBackupDetailsToBackup(b, 'Incremental')),
            ];

            return allBackups;
        } catch (error) {
            console.error("Failed to fetch backup history:", error);
            throw new Error('Failed to fetch backup history');
        }
    },
    restoreBackup: async (routine: string, timestamp: number): Promise<Response> => {
        const res = await fetch(`/v1/restore/${routine}/${timestamp}`, { method: 'POST' });
        if (!res.ok) throw new Error('Failed to trigger restore');
        return res;
    }
};

export const api = RealApi;