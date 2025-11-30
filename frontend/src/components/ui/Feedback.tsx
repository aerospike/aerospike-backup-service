import React from 'react';
import { X } from 'lucide-react';

interface BadgeProps {
  status?: 'Success' | 'Failed' | 'Running' | 'Warning';
  type?: string;
}

export const Badge = ({ status, type }: BadgeProps) => {
  if (type) {
    return (
      <span className={`px-2 py-0.5 rounded text-xs font-medium border ${type === 'Full' ? 'bg-purple-900/30 text-purple-400 border-purple-800' : 'bg-gray-800 text-gray-400 border-gray-700'}`}>
        {type}
      </span>
    )
  }

  const styles: Record<string, string> = {
    Success: "bg-green-900/30 text-green-400 border-green-800",
    Failed: "bg-red-900/30 text-red-400 border-red-800",
    Running: "bg-blue-900/30 text-blue-400 border-blue-800",
    Warning: "bg-yellow-900/30 text-yellow-400 border-yellow-800"
  };

  // Default to Running style if status matches nothing
  const statusStyle = status && styles[status] ? styles[status] : styles.Running;

  return (
    <span className={`px-2 py-0.5 rounded text-xs font-medium border ${statusStyle}`}>
      {status}
    </span>
  );
};

interface ModalProps {
  isOpen: boolean;
  onClose: () => void;
  title: string;
  children: React.ReactNode;
}

export const Modal = ({ isOpen, onClose, title, children }: ModalProps) => {
  if (!isOpen) return null;
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm p-4">
      <div className="bg-gray-900 border border-gray-700 rounded-lg shadow-2xl max-w-lg w-full overflow-hidden animate-in fade-in zoom-in-95 duration-200">
        <div className="p-4 border-b border-gray-800 flex justify-between items-center bg-gray-850">
          <h3 className="font-bold text-lg text-white">{title}</h3>
          <button onClick={onClose} className="text-gray-400 hover:text-white"><X size={20}/></button>
        </div>
        <div className="p-6">
          {children}
        </div>
      </div>
    </div>
  );
};