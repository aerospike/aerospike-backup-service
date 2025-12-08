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
                    <span className="relative inline-flex group">
                        <Info size={14} className="text-gray-500 cursor-help"/>
                        <span
                            className="absolute left-1/2 -translate-x-1/2 bottom-full mb-2 w-48 p-2 text-xs text-white bg-gray-700 rounded-md opacity-0 group-hover:opacity-100 transition-opacity duration-200 z-50 normal-case pointer-events-none">
                            {hint}
                        </span>
                    </span>
                )}
            </label>
        )}
        <input
            className={`w-full bg-white border border-gray-300 rounded p-2.5 text-sm text-gray-900 focus:border-aerospike-primary focus:ring-1 focus:ring-aerospike-primary focus:outline-none transition-colors ${className || ''}`}
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
                className="block text-xs font-semibold text-gray-600 mb-1 uppercase tracking-wide flex items-center gap-1">
                {label}
                {hint && (
                    <span className="relative inline-flex group">
                        <Info size={14} className="text-gray-400 cursor-help"/>
                        <span
                            className="absolute left-1/2 -translate-x-1/2 bottom-full mb-2 w-48 p-2 text-xs text-white bg-gray-700 rounded-md opacity-0 group-hover:opacity-100 transition-opacity duration-200 z-50 normal-case pointer-events-none">
                            {hint}
                        </span>
                    </span>
                )}
            </label>
        )}
        <div className="relative">
            <select
                className="w-full bg-white border border-gray-300 rounded p-2.5 text-sm text-gray-900 focus:border-aerospike-primary focus:outline-none appearance-none"
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
                className="w-4 h-4 text-aerospike-primary bg-white border-gray-300 rounded focus:ring-aerospike-primary focus:ring-2 focus:ring-offset-2 focus:ring-offset-white"
            />
            <label className="ml-2 text-sm font-medium text-gray-900 select-none cursor-pointer flex items-center gap-1"
                   onClick={() => onChange(!checked)}>
                {label}
                {hint && (
                    <span className="relative inline-flex group">
                        <Info size={14} className="text-gray-400 cursor-help"/>
                        <span
                            className="absolute left-1/2 -translate-x-1/2 bottom-full mb-2 w-48 p-2 text-xs text-white bg-gray-700 rounded-md opacity-0 group-hover:opacity-100 transition-opacity duration-200 z-50 normal-case pointer-events-none">
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
                className="block text-xs font-semibold text-gray-600 mb-2 uppercase tracking-wide flex items-center gap-1">
                {label}
                {hint && (
                    <span className="relative inline-flex group">
                        <Info size={14} className="text-gray-400 cursor-help"/>
                        <span
                            className="absolute left-1/2 -translate-x-1/2 bottom-full mb-2 w-48 p-2 text-xs text-white bg-gray-700 rounded-md opacity-0 group-hover:opacity-100 transition-opacity duration-200 z-50 normal-case pointer-events-none">
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
                            className="w-4 h-4 text-aerospike-primary bg-white border-gray-300 focus:ring-aerospike-primary focus:ring-2 focus:ring-offset-2 focus:ring-offset-white"
                        />
                    </div>
                    <div className="ml-3 text-sm" onClick={() => onChange(option.value)}>
                        <label className="font-medium text-gray-900 cursor-pointer">
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

interface MultiSelectProps {
    label?: string;
    value: string[];
    onChange: (value: string[]) => void;
    options: string[];
    placeholder?: string;
    hint?: string;
    description?: string;
}

export const MultiSelect = ({ label, value, onChange, options, placeholder, hint, description }: MultiSelectProps) => {
    const [inputValue, setInputValue] = React.useState("");
    const [isOpen, setIsOpen] = React.useState(false);
    const wrapperRef = React.useRef<HTMLDivElement>(null);

    React.useEffect(() => {
        const handleClickOutside = (event: MouseEvent) => {
            if (wrapperRef.current && !wrapperRef.current.contains(event.target as Node)) {
                setIsOpen(false);
            }
        };
        document.addEventListener("mousedown", handleClickOutside);
        return () => {
            document.removeEventListener("mousedown", handleClickOutside);
        };
    }, [wrapperRef]);

    const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
        if (e.key === 'Enter' || e.key === ',') {
            e.preventDefault();
            addTag(inputValue);
        } else if (e.key === 'Backspace' && !inputValue && value.length > 0) {
            const tagToRemove = value[value.length - 1];
            if (tagToRemove) {
                removeTag(tagToRemove);
            }
        }
    };

    const addTag = (tag: string) => {
        const trimmed = tag.trim();
        if (trimmed && !value.includes(trimmed)) {
            onChange([...value, trimmed]);
            setInputValue("");
        }
    };

    const removeTag = (tagToRemove: string) => {
        onChange(value.filter(tag => tag !== tagToRemove));
    };

    const filteredOptions = options.filter(opt =>
        opt.toLowerCase().includes(inputValue.toLowerCase()) && !value.includes(opt)
    );

    return (
        <div className="mb-3" ref={wrapperRef}>
            {label && (
                <label className="block text-xs font-semibold text-gray-600 mb-1 uppercase tracking-wide flex items-center gap-1">
                    {label}
                    {hint && (
                        <span className="relative inline-flex group">
                            <Info size={14} className="text-gray-400 cursor-help"/>
                            <span className="absolute left-1/2 -translate-x-1/2 bottom-full mb-2 w-48 p-2 text-xs text-white bg-gray-700 rounded-md opacity-0 group-hover:opacity-100 transition-opacity duration-200 z-50 normal-case pointer-events-none">
                                {hint}
                            </span>
                        </span>
                    )}
                </label>
            )}
            <div className="relative">
                <div className="min-h-[42px] w-full bg-white border border-gray-300 rounded p-1.5 flex flex-wrap gap-2 focus-within:border-aerospike-primary focus-within:ring-1 focus-within:ring-aerospike-primary transition-colors">
                    {value.map(tag => (
                        <span key={tag} className="bg-gray-100 text-gray-800 text-sm px-2 py-1 rounded-md flex items-center gap-1 border border-gray-200">
                            {tag}
                            <button
                                type="button"
                                onClick={() => removeTag(tag)}
                                className="text-gray-500 hover:text-red-500 focus:outline-none"
                            >
                                &times;
                            </button>
                        </span>
                    ))}
                    <input
                        type="text"
                        className="flex-1 min-w-[120px] bg-transparent outline-none text-sm text-gray-900 p-1"
                        placeholder={value.length === 0 ? placeholder : ""}
                        value={inputValue}
                        onChange={(e) => {
                            setInputValue(e.target.value);
                            setIsOpen(true);
                        }}
                        onFocus={() => setIsOpen(true)}
                        onKeyDown={handleKeyDown}
                    />
                </div>
                {isOpen && filteredOptions.length > 0 && (
                    <div className="absolute z-10 w-full mt-1 bg-white border border-gray-300 rounded-md shadow-lg max-h-60 overflow-auto">
                        {filteredOptions.map(option => (
                            <div
                                key={option}
                                className="px-4 py-2 text-sm text-gray-700 hover:bg-gray-100 cursor-pointer"
                                onClick={() => {
                                    addTag(option);
                                    setInputValue("");
                                    setIsOpen(false);
                                }}
                            >
                                {option}
                            </div>
                        ))}
                    </div>
                )}
            </div>
            {description && <p className="mt-1 text-xs text-gray-500">{description}</p>}
        </div>
    );
};