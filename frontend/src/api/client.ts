import { Job, Backup } from '../types';

// Switch this to false when embedding in Go
const USE_MOCK = true;

const MockApi = {
    fetchRunningJobs: async (): Promise<Job[]> => {
        return new Promise(resolve => setTimeout(() => {
            resolve([
                { id: 'job-1', routine: 'daily-s3', type: 'Full', progress: 45, speed: '125 MB/s', records: '4.5M', started: '10 mins ago' },
                { id: 'job-2', routine: 'hourly-local', type: 'Incremental', progress: 89, speed: '45 MB/s', records: '120K', started: '2 mins ago' }
            ]);
        }, 500));
    },
    fetchHistory: async (routineName: string): Promise<Backup[]> => {
        return new Promise(resolve => setTimeout(() => {
            const now = Date.now();
            const hour = 3600000;
            resolve([
                { id: 'bck-5', timestamp: now - (hour * 1), type: 'Incremental', size: '12 MB', duration: '2m', status: 'Success' },
                { id: 'bck-4', timestamp: now - (hour * 2), type: 'Full', size: '450 GB', duration: '1h 15m', status: 'Success' },
                { id: 'bck-3', timestamp: now - (hour * 3), type: 'Incremental', size: '45 MB', duration: '5m', status: 'Success' },
                { id: 'bck-2', timestamp: now - (hour * 4), type: 'Incremental', size: '40 MB', duration: '4m', status: 'Success' },
                { id: 'bck-1', timestamp: now - (hour * 5), type: 'Full', size: '448 GB', duration: '1h 20m', status: 'Success' },
            ]);
        }, 300));
    }
};

const RealApi = {
    fetchRunningJobs: async (): Promise<Job[]> => {
        const res = await fetch('/v1/jobs');
        if (!res.ok) throw new Error('Failed to fetch jobs');
        return res.json();
    },
    fetchHistory: async (routine: string): Promise<Backup[]> => {
        const res = await fetch(`/v1/history/${routine}`);
        if (!res.ok) throw new Error('Failed to fetch history');
        return res.json();
    }
};

export const api = USE_MOCK ? MockApi : RealApi;