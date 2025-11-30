import React from 'react';
import {Activity} from 'lucide-react';
import {DtoRunningJob} from '@/api';
import {Button} from '@/components/ui/Button';
import {Badge} from '@/components/ui/Feedback';

interface LiveActivityProps {
  activeRoutine: string;
  runningJobs: ({ type: string } & DtoRunningJob)[];
  isSchedulingBackup: boolean;
  handleScheduleFullBackup: () => Promise<void>;
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

export const LiveActivity = ({ activeRoutine, runningJobs, isSchedulingBackup, handleScheduleFullBackup }: LiveActivityProps) => {
  return (
    <div className="mb-8">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-xl font-bold text-white flex items-center gap-2">
            <Activity className="text-red-500"/> Live Activity
        </h2>
        <Button
            onClick={handleScheduleFullBackup}
            disabled={!activeRoutine || isSchedulingBackup}
            loading={isSchedulingBackup}
        >
            Backup Now
        </Button>
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
