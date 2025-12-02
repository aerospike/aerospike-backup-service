import React from 'react';
import {ChevronDown, Trash2} from 'lucide-react';

interface SectionHeaderProps {
  title: string;
  icon?: any;
  isCollapsible?: boolean;
  isCollapsed?: boolean;
  onToggle?: () => void;
}

export const SectionHeader: React.FC<SectionHeaderProps> = ({ title, icon: Icon, isCollapsible, isCollapsed, onToggle }) => {
  return (
    <div
      className={`flex items-center gap-2 mb-4 border-b border-gray-800 pb-2 mt-6 first:mt-0 ${isCollapsible ? 'cursor-pointer hover:text-white' : ''}`}
      onClick={isCollapsible ? onToggle : undefined}
    >
      {Icon && <Icon size={16} className="text-red-500" />}
      <h3 className="text-xs font-bold text-gray-400 uppercase tracking-wider flex-1">{title}</h3>
      {isCollapsible && (
        <ChevronDown
          size={16}
          className={`text-gray-500 transition-transform ${isCollapsed ? '' : 'rotate-180'}`}
        />
      )}
    </div>
  );
};

export const Card = ({ title, sub, active, onClick, onDelete }: { title: string, sub?: string, active: boolean, onClick: () => void, onDelete?: () => void }) => (
  <div
    onClick={onClick}
    className={`p-3 rounded border cursor-pointer transition-all mb-2 flex justify-between items-center group ${
      active
        ? 'bg-gray-800 border-red-500 shadow-sm'
        : 'bg-gray-900/50 border-gray-800 hover:border-gray-700 hover:bg-gray-900'
    }`}
  >
    <div className="overflow-hidden">
      <div className="font-bold text-sm text-gray-200 truncate">{title}</div>
      {sub && <div className="text-xs text-500 truncate">{sub}</div>}
    </div>
    {onDelete && (
      <button
        onClick={(e) => { e.stopPropagation(); onDelete(); }}
        className="text-gray-600 hover:text-red-400 opacity-0 group-hover:opacity-100 transition-opacity p-1"
      >
        <Trash2 size={14} />
      </button>
    )}
  </div>
);