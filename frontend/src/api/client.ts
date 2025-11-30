import { AppConfig, Job, Backup } from '../types';

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
    fetchRunningJobs: async (): Promise<Job[]> => {
        const res = await fetch('/v1/jobs');
        if (!res.ok) throw new Error('Failed to fetch jobs');
        return res.json();
    },
    fetchHistory: async (routine: string): Promise<Backup[]> => {
        const res = await fetch(`/v1/history/${routine}`);
        if (!res.ok) throw new Error('Failed to fetch history');
        return res.json();
    },
    restoreBackup: async (routine: string, timestamp: number): Promise<Response> => {
        const res = await fetch(`/v1/restore/${routine}/${timestamp}`, { method: 'POST' });
        if (!res.ok) throw new Error('Failed to trigger restore');
        return res;
    }
};

export const api = RealApi;