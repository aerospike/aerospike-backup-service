import React from 'react';
import {DtoRoutineState} from '@/api';

interface MonitoringSidebarProps {
  routineKeys: string[];
  activeRoutine: string;
  setActiveRoutine: (routine: string) => void;
  currentBackup: DtoRoutineState | null;
}

export const MonitoringSidebar = ({ routineKeys, activeRoutine, setActiveRoutine, currentBackup }: MonitoringSidebarProps) => {
  return (
    <div className="w-64 border-r border-gray-200 bg-gray-50 p-4">
      <h3 className="text-xs font-bold text-gray-500 uppercase tracking-wider mb-4">Monitored Routines</h3>
      <div className="overflow-auto max-h-full">
        <div className="space-y-1">
        {routineKeys.map(r => (
          <button
            key={r}
            onClick={() => setActiveRoutine(r)}
            className={`w-full text-left px-3 py-2 rounded text-sm font-medium flex justify-between items-center ${
              activeRoutine === r ? 'bg-aerospike-light-blue text-gray-900 border-l-2 border-aerospike-border-blue shadow-sm' : 'text-gray-600 hover:bg-gray-200'
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
    </div>
  );
};
