import React from 'react';
import {X} from 'lucide-react';

interface BadgeProps {
  status?: 'Success' | 'Failed' | 'Running' | 'Warning';
  type?: string;
}

export const Badge = ({ status, type }: BadgeProps) => {
  if (type) {
    return (
      <span className={`px-2 py-0.5 rounded text-xs font-medium border ${type === 'Full' ? 'bg-purple-100 text-purple-700 border-purple-200' : 'bg-gray-100 text-gray-600 border-gray-200'}`}>
        {type}
      </span>
    )
  }

  const styles: Record<string, string> = {
    Success: "bg-green-100 text-green-700 border-green-200",
    Failed: "bg-red-100 text-red-700 border-red-200",
    Running: "bg-blue-100 text-blue-700 border-blue-200",
    Warning: "bg-yellow-100 text-yellow-700 border-yellow-200"
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
  maxWidth?: string;
}

export const Modal = ({ isOpen, onClose, title, children, maxWidth = "max-w-lg" }: ModalProps) => {
  if (!isOpen) return null;
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm p-4">
      <div className={`bg-white border border-gray-200 rounded-lg shadow-2xl w-full overflow-hidden animate-in fade-in zoom-in-95 duration-200 ${maxWidth}`}>
        <div className="p-4 border-b border-gray-200 flex justify-between items-center bg-gray-50">
          <h3 className="font-bold text-lg text-gray-900">{title}</h3>
          <button onClick={onClose} className="text-gray-500 hover:text-gray-900"><X size={20}/></button>
        </div>
        <div className="p-6">
          {children}
        </div>
      </div>
    </div>
  );
};