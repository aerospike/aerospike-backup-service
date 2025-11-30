import React, {useEffect, useState} from 'react';
import {Activity, History, Info, RotateCcw} from 'lucide-react';
import type {Backup, DtoConfig, DtoRoutineState, DtoRunningJob} from '@/api';
import {api} from '@/api';
import {Button} from '@/components/ui/Button';
import {Badge, Modal} from '@/components/ui/Feedback';

interface MonitoringDashboardProps {
  config: DtoConfig;
}

const formatBytes = (bytes?: number, decimals = 2) => {
    if (!bytes || bytes === 0) return '0 Bytes';
    const k = 1024;
    const dm = decimals < 0 ? 0 : decimals;
    const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i];
}

const JobCard = ({ routine, type, job }: { routine: string, type: string, job: DtoRunningJob }) => (
    <div className="bg-gray-900 border border-gray-800 p-4 rounded-lg shadow-lg">
       <div className="flex justify-between mb-2">
          <span className="font-bold">{routine} - {type}</span>
          <Badge status="Running" />
       </div>
       <div className="w-full bg-gray-800 rounded-full h-1.5">
          <div className="bg-red-600 h-1.5 rounded-full" style={{ width: `${job.percentageDone}%` }}></div>
       </div>
       <div className="flex justify-between mt-2 text-xs text-gray-400">
            <span>{job.metrics?.recordsPerSecond} rps</span>
            <span>{job.doneRecords} / {job.totalRecords} records</span>
       </div>
    </div>
);

export default function MonitoringDashboard({ config }: MonitoringDashboardProps) {
  // We can safely cast the keys because we know the shape of config
  const routineKeys = Object.keys(config.backupRoutines || {});
  const [activeRoutine, setActiveRoutine] = useState<string>(routineKeys[0] || '');

  const [currentBackup, setCurrentBackup] = useState<DtoRoutineState | null>(null);
  const [history, setHistory] = useState<Backup[]>([]);

  const [selectedBackupId, setSelectedBackupId] = useState<string | null>(null);
  const [chain, setChain] = useState<string[]>([]);
  const [isRestoreModalOpen, setRestoreModalOpen] = useState<boolean>(false);
  const [isRestoring, setIsRestoring] = useState(false);
  const [restoreError, setRestoreError] = useState<string|null>(null);

  const handleRestore = async () => {
    if (!activeRoutine || !selectedBackupId) return;

    setIsRestoring(true);
    setRestoreError(null);
    try {
        await api.restoreBackup(activeRoutine, getSelectedTimestamp());
        // Ideally, show a success toast
        setRestoreModalOpen(false);
    } catch(e: any) {
        setRestoreError(e.message || 'An unknown error occurred.');
    } finally {
        setIsRestoring(false);
    }
  };

  useEffect(() => {
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
  if (currentBackup?.full) runningJobs.push({ ...currentBackup.full, type: 'Full' });
  if (currentBackup?.incremental) runningJobs.push({ ...currentBackup.incremental, type: 'Incremental' });

  return (
    <div className="flex h-full">
      {/* Sidebar */}
      <div className="w-64 border-r border-gray-800 bg-gray-900/30 p-4">
        <h3 className="text-xs font-bold text-gray-500 uppercase tracking-wider mb-4">Monitored Routines</h3>
        <div className="space-y-1">
          {routineKeys.map(r => (
            <button
              key={r}
              onClick={() => { setActiveRoutine(r); setSelectedBackupId(null); setChain([]); }}
              className={`w-full text-left px-3 py-2 rounded text-sm font-medium flex justify-between items-center ${
                activeRoutine === r ? 'bg-gray-800 text-white border-l-2 border-red-500' : 'text-gray-400 hover:bg-gray-800'
              }`}
            >
              {r}
              {activeRoutine === r && (currentBackup?.full || currentBackup?.incremental) && (
                <span className="w-2 h-2 rounded-full bg-green-500 animate-pulse"></span>
              )}
            </button>
          ))}
        </div>
      </div>

      {/* Main Content */}
      <div className="flex-1 p-6 overflow-y-auto">
        {/* Active Jobs */}
        <div className="mb-8">
            <h2 className="text-xl font-bold text-white mb-4 flex items-center gap-2">
                <Activity className="text-red-500"/> Live Activity
            </h2>
            {runningJobs.length === 0 ? (
                <div className="p-8 bg-gray-900/50 rounded border border-gray-800 text-center text-gray-500">
                    No active jobs running for {activeRoutine} at the moment.
                </div>
            ) : (
                <div className="grid gap-4">
                    {currentBackup?.full && <JobCard routine={activeRoutine} type="Full" job={currentBackup.full} />}
                    {currentBackup?.incremental && <JobCard routine={activeRoutine} type="Incremental" job={currentBackup.incremental} />}
                </div>
            )}
        </div>

        {/* History Table */}
        <div>
          <div className="flex justify-between items-end mb-4">
            <h2 className="text-xl font-bold text-white flex items-center gap-2">
                <History className="text-gray-400"/> Backup History for {activeRoutine}
            </h2>
            <div className="flex gap-2">
                {selectedBackupId && (
                  <div className="text-xs text-blue-400 flex items-center gap-2 bg-blue-900/20 px-3 py-1 rounded border border-blue-900/30 animate-in fade-in">
                    <Info size={14}/>
                    <span>Selected Chain: {chain.length} backup{chain.length > 1 ? 's' : ''}</span>
                  </div>
                )}
                <Button
                  variant="action"
                  disabled={!selectedBackupId}
                  onClick={() => { setRestoreModalOpen(true); setRestoreError(null); }}
                  icon={RotateCcw}
                >
                  Restore Selection
                </Button>
            </div>
          </div>

          <div className="bg-gray-900 border border-gray-800 rounded-lg overflow-hidden">
            <table className="w-full text-sm text-left border-collapse">
              <thead className="bg-gray-850 text-gray-400 uppercase text-xs font-semibold">
                <tr>
                   <th className="px-6 py-3 w-8"></th>
                   <th className="px-6 py-3">Time</th>
                   <th className="px-6 py-3">Type</th>
                   <th className="px-6 py-3">Size</th>
                   <th className="px-6 py-3">Records</th>
                   <th className="px-6 py-3">Duration</th>
                </tr>
              </thead>
              <tbody>
                {history.map((b) => {
                   if (!b.key) return null;
                   const isSelected = selectedBackupId === b.key;
                   const inChain = chain.includes(b.key);
                   let rowClass = "border-b border-gray-800 transition-colors cursor-pointer ";
                   if (isSelected) rowClass += "bg-blue-900/40 border-blue-500/50 ";
                   else if (inChain) rowClass += "bg-blue-900/10 border-blue-800/30 ";
                   else rowClass += "hover:bg-gray-800/50 ";

                   return (
                     <tr key={b.key} onClick={() => handleBackupSelect(b.key as string)} className={rowClass}>
                        <td className="px-6 py-4 relative">
                            {inChain && (
                                <div className="flex flex-col items-center justify-center h-full absolute inset-0">
                                     {/* Simple visual connector logic can go here (CSS lines) */}
                                    <div className={`relative z-10 w-2.5 h-2.5 rounded-full ${isSelected ? 'bg-blue-400 shadow-[0_0_10px_rgba(96,165,250,0.5)]' : 'bg-blue-600'}`}></div>
                                </div>
                            )}
                        </td>
                        <td className="px-6 py-4 font-medium text-gray-200">{new Date(b.timestamp || 0).toLocaleString()}</td>
                        <td className="px-6 py-4"><Badge type={b.type} /></td>
                        <td className="px-6 py-4 text-gray-400 font-mono">{formatBytes(b.byteCount)}</td>
                        <td className="px-6 py-4 text-gray-400 font-mono">{b.recordCount}</td>
                        <td className="px-6 py-4 text-gray-400 font-mono">{b.duration}s</td>
                     </tr>
                   );
                })}
              </tbody>
            </table>
          </div>
        </div>
      </div>

      <Modal isOpen={isRestoreModalOpen} onClose={() => setRestoreModalOpen(false)} title="Confirm Restore">
         <div className="space-y-4">
             <p className="text-gray-300">
                You are about to restore to point-in-time: <br/>
                <span className="font-mono text-white font-bold">{new Date(getSelectedTimestamp()).toLocaleString()}</span>
             </p>
             <div className="p-3 bg-blue-900/20 border border-blue-800 rounded text-sm text-blue-200">
                This operation will automatically restore {chain.length} backups in sequence.
             </div>
             {restoreError && (
                <div className="bg-red-900/30 border border-red-800 text-red-400 text-sm p-3 rounded">
                    <strong>Error:</strong> {restoreError}
                </div>
             )}
             <div className="flex justify-end gap-2 pt-2">
                <Button variant="ghost" onClick={() => setRestoreModalOpen(false)} disabled={isRestoring}>Cancel</Button>
                <Button variant="action" onClick={handleRestore} disabled={isRestoring}>
                    {isRestoring ? 'Restoring...' : 'Start Restore'}
                </Button>
             </div>
         </div>
      </Modal>
    </div>
  );
}