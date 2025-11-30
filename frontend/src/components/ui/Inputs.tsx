import React from 'react';
import { ChevronDown } from 'lucide-react';

interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> {
  label?: string;
}

export const Input = ({ label, className, ...props }: InputProps) => (
  <div className="mb-3">
    {label && <label className="block text-xs font-semibold text-gray-500 mb-1 uppercase tracking-wide">{label}</label>}
    <input
      className={`w-full bg-gray-900 border border-gray-700 rounded p-2.5 text-sm text-gray-200 focus:border-red-500 focus:ring-1 focus:ring-red-500 focus:outline-none transition-colors ${className || ''}`}
      {...props}
    />
  </div>
);

interface SelectOption {
  label: string;
  value: string | number;
}

interface SelectProps extends React.SelectHTMLAttributes<HTMLSelectElement> {
  label?: string;
  options: SelectOption[];
}

export const Select = ({ label, options, ...props }: SelectProps) => (
  <div className="mb-3">
    {label && <label className="block text-xs font-semibold text-gray-500 mb-1 uppercase tracking-wide">{label}</label>}
    <div className="relative">
      <select
        className="w-full bg-gray-900 border border-gray-700 rounded p-2.5 text-sm text-gray-200 focus:border-red-500 focus:outline-none appearance-none"
        {...props}
      >
        {options.map(opt => (
          <option key={opt.value} value={opt.value}>{opt.label}</option>
        ))}
      </select>
      <ChevronDown className="absolute right-3 top-3 text-gray-500 pointer-events-none" size={14} />
    </div>
  </div>
);