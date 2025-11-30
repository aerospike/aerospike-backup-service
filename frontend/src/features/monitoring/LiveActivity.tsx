import React from 'react';
import {Activity, Clock, Database, Gauge, Server} from 'lucide-react';
import {DtoRunningJob} from '@/api';
import {Button} from '@/components/ui/Button';
import {Badge} from '@/components/ui/Feedback';
import {format as formatTimeAgo} from 'timeago.js';

interface LiveActivityProps {
  activeRoutine: string;
  runningJobs: ({ type: string } & DtoRunningJob)[];
  isSchedulingBackup: boolean;
  isCancellingBackup: boolean;
  handleScheduleFullBackup: () => Promise<void>;
  handleCancelBackup: () => Promise<void>;
}

const Stat = ({ icon: Icon, label, value }: { icon: any, label: string, value: string | number }) => (
    <div className="flex items-center gap-2 text-sm text-gray-400">
        <Icon size={14} className="text-gray-500" />
        <div>
            <span className="font-bold text-white">{value}</span>
            <span className="text-gray-500 ml-1">{label}</span>
        </div>
    </div>
);

const JobCard = ({ routine, type, job }: { routine: string, type: string, job: DtoRunningJob }) => (
    <div className="bg-gray-900 border border-gray-800 p-4 rounded-lg shadow-lg">
       <div className="flex justify-between mb-4">
          <span className="font-bold">{routine} - {type}</span>
          <Badge status="Running" />
       </div>
       <div className="w-full bg-gray-800 rounded-full h-1.5 mb-4">
          <div className="bg-red-600 h-1.5 rounded-full" style={{ width: `${job.percentageDone}%` }}></div>
       </div>
       <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-xs text-gray-400">
            <Stat icon={Database} label="records" value={`${job.doneRecords} / ${job.totalRecords}`} />
            <Stat icon={Gauge} label="rps" value={job.metrics?.recordsPerSecond || 0} />
            <Stat icon={Server} label="MB/s" value={((job.metrics?.kilobytesPerSecond || 0) / 1024).toFixed(2)} />
            {job.estimatedEndTime && <Stat icon={Clock} label="ETA" value={formatTimeAgo(job.estimatedEndTime)} />}
       </div>
    </div>
);

export const LiveActivity = ({ activeRoutine, runningJobs, isSchedulingBackup, isCancellingBackup, handleScheduleFullBackup, handleCancelBackup }: LiveActivityProps) => {
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
                    Cancel
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
      {runningJobs.length === 0 ? (
        <div className="p-8 bg-gray-900/50 rounded border border-gray-800 text-center text-gray-500">
            No active jobs running for {activeRoutine} at the moment.
        </div>
      ) : (
        <div className="grid gap-4">
            {runningJobs.map((job, index) => (
                <JobCard key={index} routine={activeRoutine} type={job.type} job={job} />
            ))}
        </div>
      )}
    </div>
  );
};
