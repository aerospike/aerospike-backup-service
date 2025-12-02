import React from 'react';
import {ChevronDown, Info} from 'lucide-react';

interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> {
    label?: string;
    hint?: string;
    description?: string;
}

export const Input = ({label, className, hint, description, ...props}: InputProps) => (
    <div className="mb-3">
        {label && (
            <label
                className="block text-xs font-semibold text-gray-500 mb-1 uppercase tracking-wide flex items-center gap-1">
                {label}
                {hint && (
                    <span className="relative group">
            <Info size={14} className="text-gray-500 cursor-help"/>
            <span
                className="absolute left-1/2 -translate-x-1/2 bottom-full mb-2 w-48 p-2 text-xs text-white bg-gray-700 rounded-md opacity-0 group-hover:opacity-100 transition-opacity duration-200 z-50 normal-case">
              {hint}
            </span>
          </span>
                )}
            </label>
        )}
        <input
            className={`w-full bg-gray-900 border border-gray-700 rounded p-2.5 text-sm text-gray-200 focus:border-red-500 focus:ring-1 focus:ring-red-500 focus:outline-none transition-colors ${className || ''}`}
            {...props}
        />
        {description && <p className="mt-1 text-xs text-gray-500">{description}</p>}
    </div>
);

interface SelectOption {
    label: string;
    value: string | number;
}

interface SelectProps extends React.SelectHTMLAttributes<HTMLSelectElement> {
    label?: string;
    options: SelectOption[];
    hint?: string;
    description?: string;
}

export const Select = ({label, options, hint, description, ...props}: SelectProps) => (
    <div className="mb-3">
        {label && (
            <label
                className="block text-xs font-semibold text-gray-500 mb-1 uppercase tracking-wide flex items-center gap-1">
                {label}
                {hint && (
                    <span className="relative group">
            <Info size={14} className="text-gray-500 cursor-help"/>
            <span
                className="absolute left-1/2 -translate-x-1/2 bottom-full mb-2 w-48 p-2 text-xs text-white bg-gray-700 rounded-md opacity-0 group-hover:opacity-100 transition-opacity duration-200 z-50 normal-case">
              {hint}
            </span>
          </span>
                )}
            </label>
        )}
        <div className="relative">
            <select
                className="w-full bg-gray-900 border border-gray-700 rounded p-2.5 text-sm text-gray-200 focus:border-red-500 focus:outline-none appearance-none"
                {...props}
            >
                {options.map(opt => (
                    <option key={opt.value} value={opt.value}>{opt.label}</option>
                ))}
            </select>
            <ChevronDown className="absolute right-3 top-3 text-gray-500 pointer-events-none" size={14}/>
        </div>
        {description && <p className="mt-1 text-xs text-gray-500">{description}</p>}
    </div>
);

interface CheckboxProps {
    label: string;
    checked: boolean;
    onChange: (v: boolean) => void;
    hint?: string;
    description?: string;
}

export const Checkbox = ({label, checked, onChange, hint, description}: CheckboxProps) => (
    <div className="mb-3">
        <div className="flex items-center">
            <input
                type="checkbox"
                checked={checked}
                onChange={(e) => onChange(e.target.checked)}
                className="w-4 h-4 text-red-600 bg-gray-900 border-gray-700 rounded focus:ring-red-500 focus:ring-2 focus:ring-offset-2 focus:ring-offset-gray-900"
            />
            <label className="ml-2 text-sm font-medium text-gray-300 select-none cursor-pointer flex items-center gap-1"
                   onClick={() => onChange(!checked)}>
                {label}
                {hint && (
                    <span className="relative group">
                  <Info size={14} className="text-gray-500 cursor-help"/>
                  <span
                      className="absolute left-1/2 -translate-x-1/2 bottom-full mb-2 w-48 p-2 text-xs text-white bg-gray-700 rounded-md opacity-0 group-hover:opacity-100 transition-opacity duration-200 z-50 normal-case">
                      {hint}
                  </span>
              </span>
                )}
            </label>
        </div>
        {description && <p className="mt-1 ml-6 text-xs text-gray-500">{description}</p>}
    </div>
);

interface RadioOption {
    label: string;
    value: string;
    description?: string;
}

interface RadioGroupProps {
    label?: string;
    options: RadioOption[];
    value: string;
    onChange: (value: string) => void;
    hint?: string;
}

export const RadioGroup = ({label, options, value, onChange, hint}: RadioGroupProps) => (
    <div className="mb-3">
        {label && (
            <label
                className="block text-xs font-semibold text-gray-500 mb-2 uppercase tracking-wide flex items-center gap-1">
                {label}
                {hint && (
                    <span className="relative group">
            <Info size={14} className="text-gray-500 cursor-help"/>
            <span
                className="absolute left-1/2 -translate-x-1/2 bottom-full mb-2 w-48 p-2 text-xs text-white bg-gray-700 rounded-md opacity-0 group-hover:opacity-100 transition-opacity duration-200 z-50 normal-case">
              {hint}
            </span>
          </span>
                )}
            </label>
        )}
        <div className="space-y-2">
            {options.map((option) => (
                <div key={option.value} className="flex items-start">
                    <div className="flex items-center h-5">
                        <input
                            type="radio"
                            name={label}
                            value={option.value}
                            checked={value === option.value}
                            onChange={(e) => onChange(e.target.value)}
                            className="w-4 h-4 text-red-600 bg-gray-900 border-gray-700 focus:ring-red-500 focus:ring-2 focus:ring-offset-2 focus:ring-offset-gray-900"
                        />
                    </div>
                    <div className="ml-3 text-sm" onClick={() => onChange(option.value)}>
                        <label className="font-medium text-gray-300 cursor-pointer">
                            {option.label}
                        </label>
                        {option.description && (
                            <p className="text-gray-500 text-xs">{option.description}</p>
                        )}
                    </div>
                </div>
            ))}
        </div>
    </div>
);