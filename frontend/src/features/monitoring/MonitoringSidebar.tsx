import React from 'react';
import {DtoRoutineState} from '@/api';
import { FileText } from 'lucide-react';

type MonitoringViewMode = 'routines' | 'logs';

interface MonitoringSidebarProps {
  routineKeys: string[];
  activeRoutine: string;
  setActiveRoutine: (routine: string) => void;
  viewMode: MonitoringViewMode; // New prop
  setViewMode: (mode: MonitoringViewMode) => void; // New prop
  currentBackup: DtoRoutineState | null;
}

const SidebarItem = ({ 
    label, 
    isActive, 
    onClick, 
    icon: Icon, 
    indicator 
}: { 
    label: string, 
    isActive: boolean, 
    onClick: () => void, 
    icon?: any, 
    indicator?: React.ReactNode 
}) => (
    <button
        onClick={onClick}
        className={`w-full text-left px-3 py-2 rounded text-sm font-medium flex items-center justify-between transition-colors ${
            isActive 
            ? 'bg-aerospike-light-blue text-gray-900 border-l-2 border-aerospike-border-blue shadow-sm' 
            : 'text-gray-600 hover:bg-gray-200'
        }`}
    >
        <div className="flex items-center gap-3">
            {Icon && <Icon size={16} />}
            <span className="truncate">{label}</span>
        </div>
        {indicator}
    </button>
);

export const MonitoringSidebar = ({ routineKeys, activeRoutine, setActiveRoutine, viewMode, setViewMode, currentBackup }: MonitoringSidebarProps) => {
  return (
    <div className="w-64 border-r border-gray-200 bg-gray-50 flex flex-col h-full">
      <div className="p-4 flex-1 overflow-y-auto custom-scroll">
        <h3 className="text-xs font-bold text-gray-500 uppercase tracking-wider mb-4">Monitored Routines</h3>
        <div className="space-y-1">
        {routineKeys.map(r => (
          <SidebarItem 
            key={r}
            label={r}
            isActive={viewMode === 'routines' && activeRoutine === r}
            onClick={() => {
                setActiveRoutine(r);
                setViewMode('routines');
            }}
            indicator={activeRoutine === r && (currentBackup?.full || currentBackup?.incremental) && (
                <span className="w-2 h-2 rounded-full bg-green-500 animate-pulse"></span>
            )}
          />
        ))}
        </div>
      </div>

      <div className="p-4 border-t border-gray-200 bg-gray-50">
        <SidebarItem
            label="Logs"
            isActive={viewMode === 'logs'}
            onClick={() => setViewMode('logs')}
            icon={FileText}
        />
      </div>
    </div>
  );
};