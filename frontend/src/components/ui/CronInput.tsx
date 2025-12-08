import React, { useEffect, useState } from 'react';
import cronstrue from 'cronstrue';
import { Info, Settings, Clock } from 'lucide-react';

interface CronInputProps {
    value: string;
    onChange: (value: string) => void;
    label?: string;
    hint?: string;
    description?: string;
    placeholder?: string;
}

type Mode = 'preset' | 'raw';
type Preset = 'daily' | 'weekly' | 'hourly' | 'interval' | 'none';

export const CronInput = ({ value, onChange, label, hint, description, placeholder }: CronInputProps) => {
    const [mode, setMode] = useState<Mode>('preset');
    const [humanReadable, setHumanReadable] = useState<string>('');
    const [error, setError] = useState<string | null>(null);

    // Preset states
    const [presetType, setPresetType] = useState<Preset>('daily');
    const [time, setTime] = useState('00:00');
    const [weekDay, setWeekDay] = useState('MON');
    const [intervalHours, setIntervalHours] = useState('1');

    useEffect(() => {
        try {
            if (!value) {
                setHumanReadable('Not scheduled');
                setError(null);
                return;
            }
            // cronstrue handles 6-field Quartz cron automatically
            const desc = cronstrue.toString(value, { throwExceptionOnParseError: true, verbose: true });
            setHumanReadable(desc);
            setError(null);
        } catch (err) {
            setHumanReadable('');
            // Only set error if we have a value but it fails to parse
            if (value && value.trim().length > 0) {
                 setError('Invalid Quartz cron expression');
            }
        }
    }, [value]);

    // Parse value to sync internal state
    useEffect(() => {
        if (!value) {
            setMode('preset');
            setPresetType('none');
            return;
        }

        const pad = (n: string | number) => n.toString().padStart(2, '0');

        // Interval: 0 0 0/x * * ?
        const intervalMatch = value.match(/^0 0 0\/(\d+) \* \* \?$/);
        if (intervalMatch) {
            setMode('preset');
            setPresetType('interval');
            setIntervalHours(intervalMatch[1]!);
            return;
        }

        // Weekly: 0 m h ? * DAY
        const weeklyMatch = value.match(/^0 (\d+) (\d+) \? \* ([A-Z]{3})$/);
        if (weeklyMatch) {
            setMode('preset');
            setPresetType('weekly');
            setTime(`${pad(weeklyMatch[2]!)}:${pad(weeklyMatch[1]!)}`);
            setWeekDay(weeklyMatch[3]!);
            return;
        }

        // Daily: 0 m h * * ?
        const dailyMatch = value.match(/^0 (\d+) (\d+) \* \* \?$/);
        if (dailyMatch) {
            setMode('preset');
            setPresetType('daily');
            setTime(`${pad(dailyMatch[2]!)}:${pad(dailyMatch[1]!)}`);
            return;
        }

        // Hourly: 0 0 * * * ?
        if (value === '0 0 * * * ?') {
            setMode('preset');
            setPresetType('hourly');
            return;
        }

        // If no match and not empty, assume raw/custom
        // We only switch to raw if we can't map it to a preset.
        // This effectively resets the view to Raw for complex crons.
        setMode('raw');
    }, [value]);

    const updateCronFromPreset = (type: Preset, t: string, day: string, hours: string) => {
        let cron = '';
        const [h, m] = t.split(':');
        
        // Quartz Format: Seconds Minutes Hours DayOfMonth Month DayOfWeek [Year]
        // We use '?' for the "no specific value" field to avoid conflict
        switch (type) {
            case 'none':
                cron = '';
                break;
            case 'daily':
                // Run at specific time every day
                // 0 30 14 * * ? -> 2:30pm daily
                cron = `0 ${Number(m) || 0} ${Number(h) || 0} * * ?`;
                break;
            case 'weekly':
                // Run at specific time on specific day
                // 0 30 14 ? * MON -> 2:30pm every Monday
                cron = `0 ${Number(m) || 0} ${Number(h) || 0} ? * ${day}`;
                break;
            case 'hourly':
                // Run at minute 0 of every hour
                // 0 0 * * * ?
                cron = `0 0 * * * ?`; 
                break;
             case 'interval':
                // Every X hours
                // 0 0 0/4 * * ? -> Every 4 hours starting at 00:00
                cron = `0 0 0/${hours || '1'} * * ?`;
                break;
        }
        onChange(cron);
    };

    const handlePresetChange = (type: Preset) => {
        setPresetType(type);
        updateCronFromPreset(type, time, weekDay, intervalHours);
    };

    const handleTimeChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        const newTime = e.target.value;
        setTime(newTime);
        updateCronFromPreset(presetType, newTime, weekDay, intervalHours);
    };

    const handleWeekDayChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
        const newDay = e.target.value;
        setWeekDay(newDay);
        updateCronFromPreset(presetType, time, newDay, intervalHours);
    };
    
    const handleIntervalChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        const newHours = e.target.value;
        setIntervalHours(newHours);
        updateCronFromPreset(presetType, time, weekDay, newHours);
    };

    return (
        <div className="mb-4">
             {label && (
                <label className="block text-xs font-semibold text-gray-600 mb-1 uppercase tracking-wide flex items-center gap-1 justify-between">
                    <span className="flex items-center gap-1">
                        {label}
                        {hint && (
                            <span className="relative group">
                                <Info size={14} className="text-gray-400 cursor-help"/>
                                <span className="absolute left-1/2 -translate-x-1/2 bottom-full mb-2 w-48 p-2 text-xs text-white bg-gray-700 rounded-md opacity-0 group-hover:opacity-100 transition-opacity duration-200 z-50 normal-case">
                                {hint}
                                </span>
                            </span>
                        )}
                    </span>
                    <button 
                        onClick={() => setMode(mode === 'preset' ? 'raw' : 'preset')}
                        className="text-aerospike-primary text-xs normal-case flex items-center gap-1 hover:underline"
                    >
                        <Settings size={12} />
                        {mode === 'preset' ? 'Switch to Advanced' : 'Switch to Simple'}
                    </button>
                </label>
            )}

            <div className="bg-gray-50 border border-gray-200 rounded-md p-3">
                {mode === 'preset' ? (
                    <div className="space-y-3">
                         <div className="flex gap-2">
                            <div className="w-1/3">
                                <label className="block text-xs text-gray-500 mb-1">Frequency</label>
                                <select 
                                    className="w-full bg-white border border-gray-300 rounded p-2 text-sm focus:border-aerospike-primary focus:outline-none"
                                    value={presetType}
                                    onChange={(e) => handlePresetChange(e.target.value as Preset)}
                                >
                                    <option value="none">None (Disabled)</option>
                                    <option value="hourly">Hourly</option>
                                    <option value="daily">Daily</option>
                                    <option value="weekly">Weekly</option>
                                    <option value="interval">Every X Hours</option>
                                </select>
                            </div>
                            
                            <div className="flex-1">
                                {presetType === 'none' && (
                                    <div className="flex items-center h-full pt-4 text-sm text-gray-500 italic">
                                        No schedule set
                                    </div>
                                )}
                                {presetType === 'daily' && (
                                    <div>
                                        <label className="block text-xs text-gray-500 mb-1">At Time (HH:MM)</label>
                                        <input 
                                            type="time" 
                                            className="w-full bg-white border border-gray-300 rounded p-2 text-sm"
                                            value={time}
                                            onChange={handleTimeChange}
                                        />
                                    </div>
                                )}
                                {presetType === 'weekly' && (
                                    <div className="flex gap-2">
                                        <div className="flex-1">
                                            <label className="block text-xs text-gray-500 mb-1">On Day</label>
                                            <select 
                                                className="w-full bg-white border border-gray-300 rounded p-2 text-sm"
                                                value={weekDay}
                                                onChange={handleWeekDayChange}
                                            >
                                                <option value="MON">Monday</option>
                                                <option value="TUE">Tuesday</option>
                                                <option value="WED">Wednesday</option>
                                                <option value="THU">Thursday</option>
                                                <option value="FRI">Friday</option>
                                                <option value="SAT">Saturday</option>
                                                <option value="SUN">Sunday</option>
                                            </select>
                                        </div>
                                        <div className="w-32">
                                            <label className="block text-xs text-gray-500 mb-1">At Time</label>
                                            <input 
                                                type="time" 
                                                className="w-full bg-white border border-gray-300 rounded p-2 text-sm"
                                                value={time}
                                                onChange={handleTimeChange}
                                            />
                                        </div>
                                    </div>
                                )}
                                {presetType === 'interval' && (
                                     <div>
                                        <label className="block text-xs text-gray-500 mb-1">Every (Hours)</label>
                                        <input 
                                            type="number" 
                                            min="1"
                                            max="23"
                                            className="w-full bg-white border border-gray-300 rounded p-2 text-sm"
                                            value={intervalHours}
                                            onChange={handleIntervalChange}
                                        />
                                    </div>
                                )}
                                {presetType === 'hourly' && (
                                    <div className="flex items-center h-full pt-4 text-sm text-gray-500 italic">
                                        Runs at minute 0 of every hour
                                    </div>
                                )}
                            </div>
                         </div>
                    </div>
                ) : (
                    <input
                        className="w-full bg-white border border-gray-300 rounded p-2.5 text-sm text-gray-900 focus:border-aerospike-primary focus:ring-1 focus:ring-aerospike-primary focus:outline-none font-mono"
                        value={value}
                        onChange={(e) => onChange(e.target.value)}
                        placeholder={placeholder || "e.g. 0 0 12 * * ?"}
                    />
                )}
                
                {/* Result Preview */}
                <div className="mt-3 pt-3 border-t border-gray-200 flex items-start gap-2">
                    <Clock size={16} className="text-aerospike-primary mt-0.5 shrink-0" />
                    <div>
                        <p className="text-sm font-medium text-gray-900">
                           {humanReadable || <span className="text-gray-400 italic">Not scheduled</span>}
                        </p>
                        {mode === 'preset' && value && (
                            <p className="text-xs text-gray-400 font-mono mt-0.5">{value}</p>
                        )}
                        {error && <p className="text-xs text-red-500 mt-1">{error}</p>}
                    </div>
                </div>
            </div>
             {description && <p className="mt-1 text-xs text-gray-500">{description}</p>}
        </div>
    );
};
