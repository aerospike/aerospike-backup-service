import React, { useState } from 'react';
import {Activity, ChevronDown, ChevronUp, Clock, Database, Gauge, RotateCcw, Server, AlertTriangle, FileCheck, Filter} from 'lucide-react';
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

const Stat = ({icon: Icon, label, value, className}: { icon: any, label: string, value: string | number, className?: string }) => (
    <div className={`flex items-center gap-2 text-sm text-gray-600 ${className}`}>
        <Icon size={14} className="text-gray-400 flex-shrink-0"/>
        <div className="truncate">
            <span className="font-bold text-gray-900">{value}</span>
            <span className="text-gray-500 ml-1 text-xs">{label}</span>
        </div>
    </div>
);

const DetailStat = ({label, value, highlight = false}: { label: string, value?: number, highlight?: boolean }) => {
    if (value === undefined) return null;
    const valueClass = highlight && value > 0 ? 'text-red-700 font-bold' : 'text-gray-900 font-bold';
    const labelClass = highlight && value > 0 ? 'text-red-600' : 'text-gray-700';

    return (
        <div className="flex items-baseline gap-1 text-sm">
            <span className={`${labelClass} font-medium`}>{label}:</span>
            <span className={valueClass}>{value.toLocaleString()}</span>
        </div>
    );
};

const JobCard = ({routine, type, job}: { routine: string, type: string, job: DtoRunningJob }) => (
    <div className="bg-white border border-gray-200 p-4 rounded-lg shadow-sm">
        <div className="flex justify-between mb-4">
            <span className="font-bold">{routine} - {type}</span>
            <Badge status="Running"/>
        </div>
        <div className="w-full bg-gray-200 rounded-full h-1.5 mb-4">
            <div className="bg-aerospike-primary h-1.5 rounded-full" style={{width: `${job.percentageDone}%`}}></div>
        </div>
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-xs text-gray-600">
            <Stat icon={Database} label="records" value={`${job.doneRecords?.toLocaleString()} / ${job.totalRecords?.toLocaleString()}`}/>
            <Stat icon={Gauge} label="rps" value={job.metrics?.recordsPerSecond?.toLocaleString() || 0}/>
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
    const isRunning = status === 'Running';

    return (
        <div className="bg-white border border-gray-200 p-4 rounded-lg shadow-sm">
            <div className="flex justify-between items-start mb-4">
                <div>
                    <span className="font-bold flex items-center gap-2 text-gray-900">
                        <RotateCcw size={16} className="text-aerospike-primary"/>
                        Restore Job #{jobId}
                    </span>
                    {currentJob?.startTime && (
                        <div className="text-xs text-gray-500 mt-1 ml-6">
                            Started: {new Date(currentJob.startTime).toLocaleString()}
                        </div>
                    )}
                </div>
                <div className="flex items-center gap-2">
                    <Badge status={mapStatus(status)}/>
                    {isRunning && (
                        <Button
                            variant="ghost"
                            onClick={onCancel}
                            loading={isCancelling}
                            className="text-red-600 hover:text-red-700 py-1 px-2 text-xs h-auto"
                        >
                            Cancel
                        </Button>
                    )}
                </div>
            </div>

            {/* Progress Bar for Running Jobs */}
            {isRunning && currentJob && (
                <div className="mb-6">
                    <div className="flex justify-between text-xs text-gray-600 mb-1">
                        <span>Progress</span>
                        <span>{currentJob.percentageDone || 0}%</span>
                    </div>
                    <div className="w-full bg-gray-200 rounded-full h-1.5 mb-2">
                        <div className="bg-aerospike-primary h-1.5 rounded-full transition-all duration-500"
                             style={{width: `${currentJob.percentageDone || 0}%`}}></div>
                    </div>
                     <div className="grid grid-cols-2 gap-4 text-xs text-gray-600">
                         <Stat icon={Clock} label="ETA" value={currentJob.estimatedEndTime ? formatTimeAgo(currentJob.estimatedEndTime) : 'calculating...'}/>
                         <Stat icon={Gauge} label="rps" value={currentJob.metrics?.recordsPerSecond?.toLocaleString() || 0}/>
                     </div>
                </div>
            )}

            {/* Detailed Stats Grid */}
            <div className="space-y-4 border-t border-gray-100 pt-4">
                {/* Primary Stats */}
                <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
                    <DetailStat label="Read" value={job.readRecords} />
                    <DetailStat label="Inserted" value={job.insertedRecords} />
                    <DetailStat label="Total Bytes" value={job.totalBytes} />
                    {/* Placeholder for alignment if needed */}
                </div>

                {/* Filtered / Skipped Stats */}
                <div className="bg-gray-50 p-3 rounded-md">
                    <div className="flex items-center gap-2 mb-2 text-xs font-semibold text-gray-500 uppercase">
                        <Filter size={12} /> Filtered & Skipped
                    </div>
                    <div className="grid grid-cols-2 sm:grid-cols-5 gap-y-3 gap-x-2">
                        <DetailStat label="Existed" value={job.existedRecords} />
                        <DetailStat label="Fresher" value={job.fresherRecords} />
                        <DetailStat label="Expired" value={job.expiredRecords} />
                        <DetailStat label="Skipped" value={job.skippedRecords} />
                        <DetailStat label="Ignored" value={job.ignoredRecords} />
                    </div>
                </div>

                {/* System & Errors */}
                <div className="grid grid-cols-2 sm:grid-cols-3 gap-4">
                    <div className="flex items-center gap-2">
                        <FileCheck size={16} className="text-gray-400" />
                        <DetailStat label="Indexes" value={job.indexCount} />
                    </div>
                    <div className="flex items-center gap-2">
                        <FileCheck size={16} className="text-gray-400" />
                        <DetailStat label="UDF Files" value={job.udfCount} />
                    </div>
                    <div className="flex items-center gap-2">
                        <AlertTriangle size={16} className={job.errorsInDoubt && job.errorsInDoubt > 0 ? "text-red-500" : "text-gray-400"} />
                        <DetailStat label="Errors In Doubt" value={job.errorsInDoubt} highlight />
                    </div>
                </div>

                {job.error && (
                    <div className="mt-2 p-2 bg-red-50 text-red-700 text-sm rounded border border-red-100">
                        <strong>Error:</strong> {job.error}
                    </div>
                )}
            </div>
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
    const [showAllRestores, setShowAllRestores] = useState(false);

    // Sort restore jobs by start time (newest first)
    const sortedRestoreIds = Object.keys(restoreJobs).sort((a, b) => {
        const jobA = restoreJobs[a];
        const jobB = restoreJobs[b];
        const timeA = new Date(jobA?.currentJob?.startTime || 0).getTime();
        const timeB = new Date(jobB?.currentJob?.startTime || 0).getTime();
        return timeB - timeA;
    });

    const visibleRestoreIds = showAllRestores ? sortedRestoreIds : sortedRestoreIds.slice(0, 1);

    return (
        <div className="mb-8 space-y-6">
            <h2 className="text-xl font-bold text-gray-900 flex items-center gap-2">
                <Activity className="text-aerospike-primary"/> Live Activity
            </h2>

            <div className="grid md:grid-cols-2 gap-6">
                {/* Backup Jobs Panel */}
                <div className="bg-white rounded-lg border border-gray-200 shadow-sm flex flex-col h-full">
                    <div className="px-4 py-3 border-b border-gray-200 bg-gray-50 flex justify-between items-center rounded-t-lg min-h-[56px]">
                        <h3 className="font-semibold text-gray-700">Backup Jobs</h3>
                        <div className="flex gap-2">
                            {runningJobs.length > 0 ? (
                                <Button
                                    onClick={handleCancelBackup}
                                    disabled={!activeRoutine || isCancellingBackup}
                                    loading={isCancellingBackup}
                                    variant="danger"
                                    className="py-1 px-3 text-xs h-8"
                                >
                                    Cancel
                                </Button>
                            ) : (
                                <Button
                                    onClick={handleScheduleFullBackup}
                                    disabled={!activeRoutine || isSchedulingBackup}
                                    loading={isSchedulingBackup}
                                    className="py-1 px-3 text-xs h-8"
                                >
                                    Backup Now
                                </Button>
                            )}
                        </div>
                    </div>
                    <div className="p-4 flex-grow bg-gray-50/50">
                        {runningJobs.length === 0 ? (
                            <div className="h-full flex items-center justify-center text-gray-400 text-sm italic py-8">
                                No active backup jobs
                            </div>
                        ) : (
                            <div className="grid gap-4">
                                {runningJobs.map((job, index) => (
                                    <JobCard key={`backup-${index}`} routine={activeRoutine} type={job.type} job={job}/>
                                ))}
                            </div>
                        )}
                    </div>
                </div>

                {/* Restore Jobs Panel */}
                <div className="bg-white rounded-lg border border-gray-200 shadow-sm flex flex-col h-full">
                    <div className="px-4 py-3 border-b border-gray-200 bg-gray-50 rounded-t-lg flex justify-between items-center min-h-[56px]">
                        <h3 className="font-semibold text-gray-700">Restore Jobs</h3>
                        <span className="text-xs text-gray-500 font-normal bg-gray-200 px-2 py-1 rounded-full">
                            {sortedRestoreIds.length} Total
                        </span>
                    </div>
                    <div className="p-4 flex-grow bg-gray-50/50 flex flex-col gap-4">
                        {visibleRestoreIds.length === 0 ? (
                            <div className="h-full flex items-center justify-center text-gray-400 text-sm italic py-8">
                                No recent restore jobs
                            </div>
                        ) : (
                            <>
                                {visibleRestoreIds.map((jobId) => {
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

                                {sortedRestoreIds.length > 1 && (
                                    <div className="flex justify-center mt-2">
                                        <button
                                            onClick={() => setShowAllRestores(!showAllRestores)}
                                            className="flex items-center gap-1 text-sm text-aerospike-primary hover:text-aerospike-primary-dark font-medium transition-colors"
                                        >
                                            {showAllRestores ? (
                                                <>Show Less <ChevronUp size={16}/></>
                                            ) : (
                                                <>Show History ({sortedRestoreIds.length - 1} more) <ChevronDown size={16}/></>
                                            )}
                                        </button>
                                    </div>
                                )}
                            </>
                        )}
                    </div>
                </div>
            </div>
        </div>
    );
};