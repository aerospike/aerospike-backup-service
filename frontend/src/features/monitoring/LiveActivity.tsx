import React from 'react';
import {Activity, Clock, Database, Gauge, RotateCcw, Server} from 'lucide-react';
import {DtoRestoreJobStatus, DtoRunningJob} from '@/api';
import {Button} from '@/components/ui/Button';
import {Badge} from '@/components/ui/Feedback';
import {format as formatTimeAgo} from 'timeago.js';

interface LiveActivityProps {
    activeRoutine: string;
    runningJobs: ({ type: string } & DtoRunningJob)[];
    restoreJobs: { [key: string]: DtoRestoreJobStatus };
    isSchedulingBackup: boolean;
    isCancellingBackup: boolean;
    isCancellingRestore: boolean;
    handleScheduleFullBackup: () => Promise<void>;
    handleCancelBackup: () => Promise<void>;
    handleCancelRestore: (jobId: number) => Promise<void>;
}

const Stat = ({icon: Icon, label, value}: { icon: any, label: string, value: string | number }) => (
    <div className="flex items-center gap-2 text-sm text-gray-400">
        <Icon size={14} className="text-gray-500"/>
        <div>
            <span className="font-bold text-white">{value}</span>
            <span className="text-gray-500 ml-1">{label}</span>
        </div>
    </div>
);

const JobCard = ({routine, type, job}: { routine: string, type: string, job: DtoRunningJob }) => (
    <div className="bg-gray-900 border border-gray-800 p-4 rounded-lg shadow-lg">
        <div className="flex justify-between mb-4">
            <span className="font-bold">{routine} - {type}</span>
            <Badge status="Running"/>
        </div>
        <div className="w-full bg-gray-800 rounded-full h-1.5 mb-4">
            <div className="bg-red-600 h-1.5 rounded-full" style={{width: `${job.percentageDone}%`}}></div>
        </div>
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-xs text-gray-400">
            <Stat icon={Database} label="records" value={`${job.doneRecords} / ${job.totalRecords}`}/>
            <Stat icon={Gauge} label="rps" value={job.metrics?.recordsPerSecond || 0}/>
            <Stat icon={Server} label="MB/s" value={((job.metrics?.kilobytesPerSecond || 0) / 1024).toFixed(2)}/>
            {job.estimatedEndTime && <Stat icon={Clock} label="ETA" value={formatTimeAgo(job.estimatedEndTime)}/>}
        </div>
    </div>
);

const RestoreJobCard = ({jobId, job, onCancel, isCancelling}: {
    jobId: string,
    job: DtoRestoreJobStatus,
    onCancel: () => void,
    isCancelling: boolean
}) => {
    const currentJob = job.currentJob;
    const status = job.status;

    return (
        <div className="bg-gray-900 border border-gray-800 p-4 rounded-lg shadow-lg">
            <div className="flex justify-between mb-4">
                <span className="font-bold flex items-center gap-2">
                    <RotateCcw size={16} className="text-blue-500"/>
                    Restore Job #{jobId}
                </span>
                <div className="flex items-center gap-2">
                    <Badge status={mapStatus(status)}/>
                    {status === 'Running' && (
                        <Button
                            variant="ghost"
                            onClick={onCancel}
                            loading={isCancelling}
                            className="text-red-400 hover:text-red-300 py-1 px-2 text-xs h-auto"
                        >
                            Cancel
                        </Button>
                    )}
                </div>
            </div>

            {currentJob && (
                <>
                    <div className="w-full bg-gray-800 rounded-full h-1.5 mb-4">
                        <div className="bg-blue-600 h-1.5 rounded-full"
                             style={{width: `${currentJob.percentageDone || 0}%`}}></div>
                    </div>
                    <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-xs text-gray-400">
                        <Stat icon={Database} label="records"
                              value={`${currentJob.doneRecords || 0} / ${currentJob.totalRecords || '?'}`}/>
                        <Stat icon={Gauge} label="rps" value={currentJob.metrics?.recordsPerSecond || 0}/>
                        <Stat icon={Server} label="MB/s"
                              value={((currentJob.metrics?.kilobytesPerSecond || 0) / 1024).toFixed(2)}/>
                        {currentJob.estimatedEndTime &&
                            <Stat icon={Clock} label="ETA" value={formatTimeAgo(currentJob.estimatedEndTime)}/>}
                    </div>
                </>
            )}
            {job.error && (
                <div className="mt-2 text-red-400 text-xs">
                    Error: {job.error}
                </div>
            )}
        </div>
    );
}

const mapStatus = (status?: string): 'Success' | 'Failed' | 'Running' | 'Warning' => {
    switch (status) {
        case 'Running':
            return 'Running';
        case 'Done':
            return 'Success';
        case 'Failed':
            return 'Failed';
        case 'Cancelled':
            return 'Warning';
        default:
            return 'Running';
    }
};

export const LiveActivity = ({
                                 activeRoutine,
                                 runningJobs,
                                 restoreJobs,
                                 isSchedulingBackup,
                                 isCancellingBackup,
                                 isCancellingRestore,
                                 handleScheduleFullBackup,
                                 handleCancelBackup,
                                 handleCancelRestore
                             }: LiveActivityProps) => {
    const restoreJobIds = Object.keys(restoreJobs);
    const hasActiveJobs = runningJobs.length > 0 || restoreJobIds.length > 0;

    return (
        <div className="mb-8">
            <div className="flex items-center justify-between mb-4">
                <h2 className="text-xl font-bold text-white flex items-center gap-2">
                    <Activity className="text-red-500"/> Live Activity
                </h2>
                <div className="flex gap-2">
                    {runningJobs.length > 0 ? (
                        <Button
                            onClick={handleCancelBackup}
                            disabled={!activeRoutine || isCancellingBackup}
                            loading={isCancellingBackup}
                            variant="danger"
                        >
                            Cancel Backup
                        </Button>
                    ) : (
                        <Button
                            onClick={handleScheduleFullBackup}
                            disabled={!activeRoutine || isSchedulingBackup}
                            loading={isSchedulingBackup}
                        >
                            Backup Now
                        </Button>
                    )}
                </div>
            </div>

            {!hasActiveJobs ? (
                <div className="p-8 bg-gray-900/50 rounded border border-gray-800 text-center text-gray-500">
                    No active jobs running at the moment.
                </div>
            ) : (
                <div className="grid gap-4">
                    {runningJobs.map((job, index) => (
                        <JobCard key={`backup-${index}`} routine={activeRoutine} type={job.type} job={job}/>
                    ))}
                    {restoreJobIds.map((jobId) => {
                        const job = restoreJobs[jobId];
                        if (!job) return null;
                        return (
                            <RestoreJobCard
                                key={`restore-${jobId}`}
                                jobId={jobId}
                                job={job}
                                onCancel={() => handleCancelRestore(Number(jobId))}
                                isCancelling={isCancellingRestore}
                            />
                        );
                    })}
                </div>
            )}
        </div>
    );
};
