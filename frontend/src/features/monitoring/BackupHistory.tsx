import React from 'react';
import {History, Info, RotateCcw} from 'lucide-react';
import {Backup} from '@/api';
import {Button} from '@/components/ui/Button';
import {Badge, Modal} from '@/components/ui/Feedback';

interface BackupHistoryProps {
    activeRoutine: string;
    history: Backup[];
    selectedBackupId: string | null;
    setSelectedBackupId: (id: string | null) => void;
    chain: string[];
    setChain: (chain: string[]) => void;
    isRestoreModalOpen: boolean;
    setRestoreModalOpen: (isOpen: boolean) => void;
    isRestoring: boolean;
    restoreError: string | null;
    handleRestore: () => Promise<void>;
    handleBackupSelect: (key: string) => void;
    getSelectedTimestamp: () => number;
}

const formatBytes = (bytes?: number, decimals = 2) => {
    if (!bytes || bytes === 0) return '0 Bytes';
    const k = 1024;
    const dm = decimals < 0 ? 0 : decimals;
    const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i];
}

export const BackupHistory = (
    {
        activeRoutine,
        history,
        selectedBackupId,
        setSelectedBackupId,
        chain,
        setChain,
        isRestoreModalOpen,
        setRestoreModalOpen,
        isRestoring,
        restoreError,
        handleRestore,
        handleBackupSelect,
        getSelectedTimestamp
    }: BackupHistoryProps) => {

    return (
        <div>
            <div className="flex justify-between items-end mb-4">
                <h2 className="text-xl font-bold text-white flex items-center gap-2">
                    <History className="text-gray-400"/> Backup History for {activeRoutine}
                </h2>
                <div className="flex gap-2">
                    {selectedBackupId && (
                        <div
                            className="text-xs text-blue-400 flex items-center gap-2 bg-blue-900/20 px-3 py-1 rounded border border-blue-900/30 animate-in fade-in">
                            <Info size={14}/>
                            <span>Selected Chain: {chain.length} backup{chain.length > 1 ? 's' : ''}</span>
                        </div>
                    )}
                    <Button
                        variant="action"
                        disabled={!selectedBackupId}
                        onClick={() => {
                            setRestoreModalOpen(true);
                            // reset restoreError only when opening the modal
                            // setRestoreError(null);
                        }}
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
                            <tr key={b.key} onClick={() => handleBackupSelect(b.key as string)}
                                className={rowClass}>
                                <td className="px-6 py-4 relative">
                                    {inChain && (
                                        <div
                                            className="flex flex-col items-center justify-center h-full absolute inset-0">
                                            {/* Simple visual connector logic can go here (CSS lines) */}
                                            <div
                                                className={`relative z-10 w-2.5 h-2.5 rounded-full ${isSelected ? 'bg-blue-400 shadow-[0_0_10px_rgba(96,165,250,0.5)]' : 'bg-blue-600'}`}></div>
                                        </div>
                                    )}
                                </td>
                                <td className="px-6 py-4 font-medium text-gray-200">{new Date(b.timestamp || 0).toLocaleString()}</td>
                                <td className="px-6 py-4"><Badge type={b.type}/></td>
                                <td className="px-6 py-4 text-gray-400 font-mono">{formatBytes(b.byteCount)}</td>
                                <td className="px-6 py-4 text-gray-400 font-mono">{b.recordCount}</td>
                                <td className="px-6 py-4 text-gray-400 font-mono">{b.duration}s</td>
                            </tr>
                        );
                    })}
                    </tbody>
                </table>
            </div>

            <Modal isOpen={isRestoreModalOpen} onClose={() => setRestoreModalOpen(false)} title="Confirm Restore">
                <div className="space-y-4">
                    <p className="text-gray-300">
                        You are about to restore to point-in-time: <br/>
                        <span
                            className="font-mono text-white font-bold">{new Date(getSelectedTimestamp()).toLocaleString()}</span>
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
                        <Button variant="ghost" onClick={() => setRestoreModalOpen(false)}
                                disabled={isRestoring}>Cancel</Button>
                        <Button variant="action" onClick={handleRestore} disabled={isRestoring}>
                            {isRestoring ? 'Restoring...' : 'Start Restore'}
                        </Button>
                    </div>
                </div>
            </Modal>
        </div>
    );
};
