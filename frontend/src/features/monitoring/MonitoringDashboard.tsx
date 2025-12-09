import React, {useEffect, useState} from 'react';
import type {Backup, DtoConfig, DtoRestoreJobStatus, DtoRoutineState, DtoRunningJob} from '@/api';
import * as allGenerated from '@/api'; // Import allGenerated here
import {api} from '@/api';
import {MonitoringSidebar} from './MonitoringSidebar';
import {LiveActivity} from './LiveActivity';
import {BackupHistory} from './BackupHistory';
import {ServiceLogs} from './ServiceLogs';

type MonitoringViewMode = 'routines' | 'logs';

interface MonitoringDashboardProps {
    config: DtoConfig;
}

export default function MonitoringDashboard({config}: MonitoringDashboardProps) {
    // We can safely cast the keys because we know the shape of config
    const routineKeys = Object.keys(config.backupRoutines || {});
    
    // Initialize activeRoutine from localStorage or default to first routine
    const [activeRoutine, setActiveRoutine] = useState<string>(() => {
        const saved = localStorage.getItem('abs_sel_routines');
        if (saved && routineKeys.includes(saved)) {
            return saved;
        }
        return routineKeys[0] || '';
    });

    const [viewMode, setViewMode] = useState<MonitoringViewMode>('routines');

    const [currentBackup, setCurrentBackup] = useState<DtoRoutineState | null>(null);
    const [restoreJobs, setRestoreJobs] = useState<{ [key: string]: DtoRestoreJobStatus }>({});
    const [history, setHistory] = useState<Backup[]>([]);
    const [isSchedulingBackup, setIsSchedulingBackup] = useState(false);
    const [isCancellingBackup, setIsCancellingBackup] = useState(false);
    const [isCancellingRestore, setIsCancellingRestore] = useState(false);

    const [selectedBackupId, setSelectedBackupId] = useState<string | null>(null);
    const [chain, setChain] = useState<string[]>([]);
    const [isRestoreModalOpen, setRestoreModalOpen] = useState<boolean>(false);
    const [isRestoring, setIsRestoring] = useState(false);
    const [restoreError, setRestoreError] = useState<string | null>(null);

    // Persist activeRoutine to localStorage to share with ConfigEditor
    useEffect(() => {
        if (activeRoutine) {
            localStorage.setItem('abs_sel_routines', activeRoutine);
        }
    }, [activeRoutine]);

    const loadData = async () => {
        if (viewMode !== 'routines') return; // Only load routine data if in routines view

        try {
            const rJobs = await api.fetchRestoreJobs();
            setRestoreJobs(rJobs);

            if (activeRoutine) {
                const [backup, hist] = await Promise.all([
                    api.fetchCurrentBackup(activeRoutine),
                    api.fetchHistory(activeRoutine)
                ]);
                setCurrentBackup(backup);
                setHistory(hist.sort((a: Backup, b: Backup) => (b.timestamp || 0) - (a.timestamp || 0)));
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


    const handleRestore = async (request: allGenerated.DtoRestoreTimestampRequest) => {
        if (viewMode !== 'routines' || !activeRoutine || !selectedBackupId) return;

        setIsRestoring(true);
        setRestoreError(null);
        try {
            await api.restoreBackup(request);
            setRestoreModalOpen(false);
            await loadData(); // Refresh data after restore
        } catch (e: any) {
            setRestoreError(e.message || 'An unknown error occurred.');
        } finally {
            setIsRestoring(false);
        }
    };

    const handleScheduleFullBackup = async () => {
        if (viewMode !== 'routines' || !activeRoutine) return;
        setIsSchedulingBackup(true);
        try {
            await api.scheduleFullBackup(activeRoutine);
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

    const handleCancelBackup = async () => {
        if (viewMode !== 'routines' || !activeRoutine) return;
        setIsCancellingBackup(true);
        try {
            await api.cancelBackup(activeRoutine);
            // Delay refresh by 100ms
            setTimeout(() => {
                loadData(); //refresh active jobs after cancelling
            }, 100);
        } catch (error: any) {
            console.error("Failed to cancel backup", error);
        } finally {
            setIsCancellingBackup(false);
        }
    };

    const handleCancelRestore = async (jobId: number) => {
        if (viewMode !== 'routines') return; // Should not happen, but defensive check
        setIsCancellingRestore(true);
        try {
            await api.cancelRestore(jobId);
            setTimeout(() => {
                loadData();
            }, 100);
        } catch (error: any) {
            console.error("Failed to cancel restore", error);
        } finally {
            setIsCancellingRestore(false);
        }
    };

    useEffect(() => {
        loadData();
        const interval = setInterval(loadData, 5000);
        return () => clearInterval(interval);
    }, [activeRoutine, viewMode]); // Added viewMode to dependencies

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
        if (!selectedBackup || !selectedBackup.key) return;
        const newChain = [selectedBackup.key]; // Use key here

        if (selectedBackup.type === 'Incremental') {
            // Look forward in the array (backward in time) to find the nearest Full backup
            for (let i = selectedIndex + 1; i < history.length; i++) {
                const b = history[i];
                if (!b || !b.key) continue;
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
                    setViewMode('routines'); // Ensure viewMode is 'routines' when a routine is selected
                    setSelectedBackupId(null);
                    setChain([]);
                }}
                viewMode={viewMode}
                setViewMode={setViewMode}
                currentBackup={currentBackup}
            />

            {viewMode === 'logs' ? (
                <div className="flex-1 h-full overflow-hidden">
                    <ServiceLogs />
                </div>
            ) : (
                <div className="flex-1 p-6 overflow-y-auto">
                    <LiveActivity
                        activeRoutine={activeRoutine}
                        runningJobs={runningJobs}
                        restoreJobs={restoreJobs}
                        isSchedulingBackup={isSchedulingBackup}
                        isCancellingBackup={isCancellingBackup}
                        isCancellingRestore={isCancellingRestore}
                        handleScheduleFullBackup={handleScheduleFullBackup}
                        handleCancelBackup={handleCancelBackup}
                        handleCancelRestore={handleCancelRestore}
                    />

                    <BackupHistory
                        config={config}
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
            )}
        </div>
    );
}