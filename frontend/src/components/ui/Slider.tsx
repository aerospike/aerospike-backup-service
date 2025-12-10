import React from 'react';

interface SliderProps {
  label: string;
  value: number;
  min: number;
  max: number;
  step: number;
  onChange: (value: number) => void;
  description?: string;
}

export const Slider: React.FC<SliderProps> = ({ label, value, min, max, step, onChange, description }) => {
  return (
    <div className="flex flex-col gap-1">
      <label className="text-sm font-medium text-gray-700">{label}</label>
      <div className="flex items-center gap-2">
        <input
          type="range"
          min={min}
          max={max}
          step={step}
          value={value}
          onChange={(e) => onChange(Number(e.target.value))}
          className="flex-1 h-2 bg-gray-200 rounded-lg appearance-none cursor-pointer range-sm dark:bg-gray-700"
        />
        <span className="w-10 text-right text-sm font-medium text-gray-700">{value}</span>
      </div>
      {description && <p className="text-xs text-gray-500 mt-1">{description}</p>}
    </div>
  );
};
