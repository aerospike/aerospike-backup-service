import React from 'react';
import {motion} from 'framer-motion';
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
      className={`flex items-center gap-2 mb-4 border-b border-gray-200 pb-2 mt-6 first:mt-0 ${isCollapsible ? 'cursor-pointer hover:text-gray-900' : ''}`}
      onClick={isCollapsible ? onToggle : undefined}
    >
      {Icon && <Icon size={16} className="text-aerospike-primary" />}
      <h3 className="text-xs font-bold text-gray-700 uppercase tracking-wider flex-1">{title}</h3>
      {isCollapsible && (
        <ChevronDown
          size={16}
          className={`text-gray-400 transition-transform ${isCollapsed ? '' : 'rotate-180'}`}
        />
      )}
    </div>
  );
};

export const Card = ({ title, sub, active, onClick, onDelete }: { title: string, sub?: string, active: boolean, onClick: () => void, onDelete?: () => void }) => (
  <motion.div
    onClick={onClick}
    className={`p-3 rounded border cursor-pointer transition-all mb-2 flex justify-between items-center group ${
      active
        ? 'bg-white border-aerospike-primary shadow-sm'
        : 'bg-gray-50 border-gray-200 hover:border-gray-300 hover:bg-gray-100'
    }`}
    whileHover={{ scale: 1.02 }}
    whileTap={{ scale: 0.98 }}
    animate={{
        scale: active ? 1.01 : 1,
        backgroundColor: active ? 'rgba(255, 255, 255, 1)' : 'rgba(249, 250, 251, 1)', // Tailwind's bg-white and bg-gray-50
        borderColor: active ? '#ef4444' : 'rgb(229 231 235)', // Tailwind's border-aerospike-primary and border-gray-200
        boxShadow: active ? '0 1px 2px 0 rgba(0, 0, 0, 0.05)' : 'none',
    }}
    transition={{ duration: 0.1 }}
  >
    <div className="overflow-hidden">
      <div className="font-bold text-sm text-gray-900 truncate">{title}</div>
      {sub && <div className="text-xs text-gray-600 truncate">{sub}</div>}
    </div>
    {onDelete && (
      <button
        onClick={(e) => { e.stopPropagation(); onDelete(); }}
        className="text-gray-500 hover:text-red-600 opacity-0 group-hover:opacity-100 transition-opacity p-1"
      >
        <Trash2 size={14} />
      </button>
    )}
  </motion.div>
);