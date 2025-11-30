import React, {useEffect, useState} from 'react';
import type {Backup, DtoConfig, DtoRoutineState, DtoRunningJob} from '@/api';
import {api, BackupApi} from '@/api';
import {MonitoringSidebar} from './MonitoringSidebar';
import {LiveActivity} from './LiveActivity';
import {BackupHistory} from './BackupHistory';

interface MonitoringDashboardProps {
    config: DtoConfig;
}

export default function MonitoringDashboard({config}: MonitoringDashboardProps) {
    // We can safely cast the keys because we know the shape of config
    const routineKeys = Object.keys(config.backupRoutines || {});
    const [activeRoutine, setActiveRoutine] = useState<string>(routineKeys[0] || '');

    const [currentBackup, setCurrentBackup] = useState<DtoRoutineState | null>(null);
    const [history, setHistory] = useState<Backup[]>([]);
    const [isSchedulingBackup, setIsSchedulingBackup] = useState(false);

    const [selectedBackupId, setSelectedBackupId] = useState<string | null>(null);
    const [chain, setChain] = useState<string[]>([]);
    const [isRestoreModalOpen, setRestoreModalOpen] = useState<boolean>(false);
    const [isRestoring, setIsRestoring] = useState(false);
    const [restoreError, setRestoreError] = useState<string | null>(null);

    const backupApi = new BackupApi();

    const loadData = async () => {
        try {
            if (activeRoutine) {
                const [backup, hist] = await Promise.all([
                    api.fetchCurrentBackup(activeRoutine),
                    api.fetchHistory(activeRoutine)
                ]);
                setCurrentBackup(backup);
                setHistory(hist.sort((a, b) => (b.timestamp || 0) - (a.timestamp || 0)));
            } else {
                setCurrentBackup(null);
                setHistory([]);
            }
        } catch (error) {
            console.error("Failed to load monitoring data", error);
            setCurrentBackup(null);
            setHistory([]);
        }
    };


    const handleRestore = async () => {
        if (!activeRoutine || !selectedBackupId) return;

        setIsRestoring(true);
        setRestoreError(null);
        try {
            await api.restoreBackup(activeRoutine, getSelectedTimestamp());
            setRestoreModalOpen(false);
            await loadData(); // Refresh data after restore
        } catch (e: any) {
            setRestoreError(e.message || 'An unknown error occurred.');
        } finally {
            setIsRestoring(false);
        }
    };

    const handleScheduleFullBackup = async () => {
        if (!activeRoutine) return;
        setIsSchedulingBackup(true);
        try {
            await backupApi.scheduleFullBackup({name: activeRoutine});
            // Delay refresh by 100ms
            setTimeout(() => {
                loadData(); //refresh active jobs after scheduling
            }, 100);
        } catch (error: any) {
            console.error("Failed to schedule full backup", error);
            // Optionally, use a less intrusive toast/notification here if available, or just log
        } finally {
            setIsSchedulingBackup(false);
        }
    };

    useEffect(() => {
        loadData();
        const interval = setInterval(loadData, 5000);
        return () => clearInterval(interval);
    }, [activeRoutine]);

    const handleBackupSelect = (key: string) => { // Changed id to key
        if (selectedBackupId === key) { // Use key here
            setSelectedBackupId(null);
            setChain([]);
            return;
        }
        setSelectedBackupId(key); // Use key here

        // Chain Logic
        const selectedIndex = history.findIndex(b => b.key === key); // Use key here
        if (selectedIndex === -1) return;
        const selectedBackup = history[selectedIndex];
        if (!selectedBackup.key) return;
        const newChain = [selectedBackup.key]; // Use key here

        if (selectedBackup.type === 'Incremental') {
            // Look forward in the array (backward in time) to find the nearest Full backup
            for (let i = selectedIndex + 1; i < history.length; i++) {
                const b = history[i];
                if (!b.key) continue;
                newChain.push(b.key); // Use key here
                if (b.type === 'Full') break;
            }
        }
        setChain(newChain);
    };

    const getSelectedTimestamp = () => {
        const b = history.find(h => h.key === selectedBackupId); // Use key here
        return b?.timestamp || 0;
    };

    const runningJobs: ({ type: string } & DtoRunningJob)[] = [];
    if (currentBackup?.full) runningJobs.push({...currentBackup.full, type: 'Full'});
    if (currentBackup?.incremental) runningJobs.push({...currentBackup.incremental, type: 'Incremental'});

    return (
        <div className="flex h-full">
            <MonitoringSidebar
                routineKeys={routineKeys}
                activeRoutine={activeRoutine}
                setActiveRoutine={(r) => {
                    setActiveRoutine(r);
                    setSelectedBackupId(null);
                    setChain([]);
                }}
                currentBackup={currentBackup}
            />

            <div className="flex-1 p-6 overflow-y-auto">
                <LiveActivity
                    activeRoutine={activeRoutine}
                    runningJobs={runningJobs}
                    isSchedulingBackup={isSchedulingBackup}
                    handleScheduleFullBackup={handleScheduleFullBackup}
                />

                <BackupHistory
                    activeRoutine={activeRoutine}
                    history={history}
                    selectedBackupId={selectedBackupId}
                    setSelectedBackupId={setSelectedBackupId}
                    chain={chain}
                    setChain={setChain}
                    isRestoreModalOpen={isRestoreModalOpen}
                    setRestoreModalOpen={setRestoreModalOpen}
                    isRestoring={isRestoring}
                    restoreError={restoreError}
                    handleRestore={handleRestore}
                    handleBackupSelect={handleBackupSelect}
                    getSelectedTimestamp={getSelectedTimestamp}
                />
            </div>
        </div>
    );
}