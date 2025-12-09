import React, {useState, useEffect} from 'react';
import {CheckCircle, XCircle} from 'lucide-react';

interface ToastProps {
  message: string;
  type: 'success' | 'error';
  onClose: () => void;
}

export const Toast: React.FC<ToastProps> = ({ message, type, onClose }) => {
  const [isVisible, setIsVisible] = useState(true);

  useEffect(() => {
    const timer = setTimeout(() => {
      setIsVisible(false);
      onClose();
    }, 3000); // Hide after 3 seconds

    return () => clearTimeout(timer);
  }, [onClose]);

  const bgColor = type === 'success' ? 'bg-green-500' : 'bg-red-500';
  const Icon = type === 'success' ? CheckCircle : XCircle;

  if (!isVisible) return null;

  return (
    <div className={`fixed top-4 right-4 z-50 flex items-center p-4 rounded-md shadow-lg text-white ${bgColor}`}>
      <Icon className="mr-2" size={20} />
      <span>{message}</span>
      <button onClick={() => { setIsVisible(false); onClose(); }} className="ml-4 text-white">
        <XCircle size={16} />
      </button>
    </div>
  );
};
