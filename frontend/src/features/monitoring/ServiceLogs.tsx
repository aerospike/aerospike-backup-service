import React, {useState, useEffect, useMemo, useCallback} from 'react';
import {api, LogEntry} from '@/api';
import {Input, Select, MultiSelect} from '@/components/ui/Inputs';
import { motion } from 'framer-motion';
import { Play, Pause, Download, Copy, Ban, Trash2 } from 'lucide-react';

type Severity = 'ALL' | 'INFO' | 'WARN' | 'ERROR' | 'DEBUG' | 'TRACE';

const severityValues: Record<string, number> = {
    'ALL': 0,
    'TRACE': 10,
    'DEBUG': 20,
    'INFO': 30,
    'WARN': 40,
    'ERROR': 50
};

const LogLevel = ({ level }: { level: Severity | string }) => {
    let colorClass = '';
    const normalizedLevel = level?.toUpperCase() || 'INFO';
    
    switch (normalizedLevel) {
        case 'INFO':
            colorClass = 'text-blue-500';
            break;
        case 'WARN':
            colorClass = 'text-yellow-500';
            break;
        case 'ERROR':
            colorClass = 'text-red-500';
            break;
        case 'DEBUG':
            colorClass = 'text-purple-500';
            break;
        case 'TRACE':
            colorClass = 'text-gray-500';
            break;
        default:
            colorClass = 'text-gray-700';
            break;
    }
    return <span className={`font-semibold ${colorClass}`}>{normalizedLevel}</span>;
};

export const ServiceLogs = () => {
    const [logs, setLogs] = useState<LogEntry[]>([]);
    const [loading, setLoading] = useState<boolean>(true);
    const [error, setError] = useState<string | null>(null);
    const [severityFilter, setSeverityFilter] = useState<Severity>('ALL');
    const [searchTerm, setSearchTerm] = useState<string>('');
    const [excludeTerms, setExcludeTerms] = useState<string[]>([]); // Changed from excludeTerm (string)
    const [isLive, setIsLive] = useState<boolean>(false);

    const fetchLogs = useCallback(async (isPolling = false) => {
        try {
            if (!isPolling) setLoading(true);
            const fetchedLogs = await api.fetchLogs();
            setLogs(fetchedLogs);
            setError(null);
        } catch (err: any) {
            setError(err.message || 'Failed to fetch logs');
            if (isLive) setIsLive(false); // Stop polling on error
        } finally {
            if (!isPolling) setLoading(false);
        }
    }, [isLive]);

    useEffect(() => {
        fetchLogs();
    }, []); // Initial load

    useEffect(() => {
        let interval: any;
        if (isLive) {
            interval = setInterval(() => fetchLogs(true), 2000);
        }
        return () => clearInterval(interval);
    }, [isLive, fetchLogs]);

    const filteredLogs = useMemo(() => {
        let currentLogs = logs;

        // Min Level Filter
        if (severityFilter !== 'ALL') {
            const minVal = severityValues[severityFilter] || 0;
            currentLogs = currentLogs.filter(log => {
                const levelVal = severityValues[log.level?.toUpperCase() || 'INFO'] || 0;
                return levelVal >= minVal;
            });
        }

        // Search Term (Inclusive)
        if (searchTerm) {
            const lowerCaseSearchTerm = searchTerm.toLowerCase();
            currentLogs = currentLogs.filter(log => {
                const inMsg = log.msg && log.msg.toLowerCase().includes(lowerCaseSearchTerm);
                const inAttrs = log.attrs && Object.values(log.attrs).some(value =>
                    String(value).toLowerCase().includes(lowerCaseSearchTerm)
                );
                return inMsg || inAttrs;
            });
        }

        // Exclude Terms (Exclusive)
        if (excludeTerms.length > 0) {
            currentLogs = currentLogs.filter(log => {
                const shouldExclude = excludeTerms.some(term => {
                    const lowerCaseTerm = term.toLowerCase();
                    const inMsg = log.msg && log.msg.toLowerCase().includes(lowerCaseTerm);
                    const inAttrs = log.attrs && Object.values(log.attrs).some(value =>
                        String(value).toLowerCase().includes(lowerCaseTerm)
                    );
                    return inMsg || inAttrs;
                });
                return !shouldExclude;
            });
        }

        return currentLogs;
    }, [logs, severityFilter, searchTerm, excludeTerms]); // Added excludeTerms to dependencies

    const handleDownload = () => {
        const jsonString = JSON.stringify(filteredLogs, null, 2);
        const blob = new Blob([jsonString], { type: 'application/json' });
        const href = URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = href;
        link.download = `service-logs-${new Date().toISOString()}.json`;
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
    };

    const handleCopy = (log: LogEntry) => {
        const text = JSON.stringify(log, null, 2);
        navigator.clipboard.writeText(text);
        // Optionally add a toast notification here
    };

    return (
        <div className="flex flex-col h-full bg-white">
            <h2 className="text-xl font-semibold mb-4 p-6 border-b border-gray-200 flex justify-between items-center">
                <span>Service Logs</span>
                <div className="flex gap-2">
                    <button
                        onClick={() => setIsLive(!isLive)}
                        className={`flex items-center gap-2 px-3 py-1.5 rounded-md text-sm font-medium transition-colors ${
                            isLive ? 'bg-green-100 text-green-700 hover:bg-green-200' : 'bg-gray-100 text-gray-700 hover:bg-gray-200'
                        }`}
                        title={isLive ? "Pause Live Polling" : "Start Live Polling"}
                    >
                        {isLive ? <Pause size={16} /> : <Play size={16} />}
                        {isLive ? 'Live' : 'Paused'}
                    </button>
                    <button
                        onClick={handleDownload}
                        className="flex items-center gap-2 px-3 py-1.5 rounded-md text-sm font-medium bg-gray-100 text-gray-700 hover:bg-gray-200 transition-colors"
                        title="Download Logs"
                    >
                        <Download size={16} />
                        Export
                    </button>
                </div>
            </h2>

            <div className="p-4 border-b border-gray-200 bg-gray-50/50">
                <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
                    <Select
                        label="Min Level"
                        value={severityFilter}
                        options={[
                            { label: 'ALL', value: 'ALL' },
                            { label: 'TRACE', value: 'TRACE' },
                            { label: 'DEBUG', value: 'DEBUG' },
                            { label: 'INFO', value: 'INFO' },
                            { label: 'WARN', value: 'WARN' },
                            { label: 'ERROR', value: 'ERROR' },
                        ]}
                        onChange={(e) => setSeverityFilter(e.target.value as Severity)}
                    />
                    <Input
                        label="Search"
                        placeholder="Filter..."
                        value={searchTerm}
                        onChange={(e) => setSearchTerm(e.target.value)}
                    />
                    <MultiSelect
                        label="Exclude"
                        placeholder="Add terms to exclude..."
                        value={excludeTerms}
                        onChange={setExcludeTerms}
                        options={[]} // No predefined options, user types freely
                        description="Exclude logs containing any of these terms"
                    />
                    <div className="flex items-end">
                        <span className="text-xs text-gray-500 mb-2">
                            Showing {filteredLogs.length} of {logs.length} events
                        </span>
                    </div>
                </div>
            </div>

            <div className="flex-1 overflow-y-auto p-6 custom-scroll">
                {loading && !isLive && <p className="text-center text-gray-500">Loading logs...</p>}
                {error && <p className="text-center text-red-500">Error: {error}</p>}
                
                {!loading && filteredLogs.length === 0 && !error && (
                    <div className="flex flex-col items-center justify-center h-full text-gray-400">
                        <Trash2 size={32} className="mb-2 opacity-50"/>
                        <p>No logs found matching criteria.</p>
                    </div>
                )}

                {filteredLogs.length > 0 && (
                    <div className="space-y-2">
                        {filteredLogs.map((log, index) => (
                            <motion.div
                                key={index}
                                initial={{ opacity: 0, y: 5 }}
                                animate={{ opacity: 1, y: 0 }}
                                transition={{ duration: 0.1 }}
                                className="group border border-gray-100 p-3 rounded-md shadow-sm bg-gray-50 text-sm break-all relative hover:border-blue-200 transition-colors"
                            >
                                <div className="absolute top-3 right-3 opacity-0 group-hover:opacity-100 transition-opacity">
                                    <button 
                                        onClick={() => handleCopy(log)}
                                        className="p-1.5 text-gray-400 hover:text-blue-600 hover:bg-blue-50 rounded-md"
                                        title="Copy Log JSON"
                                    >
                                        <Copy size={14} />
                                    </button>
                                </div>

                                <div className="flex items-center gap-2 mb-1 pr-8">
                                    <span className="text-xs text-gray-500 font-mono">
                                        {log.time ? new Date(log.time).toLocaleString() : '-'}
                                    </span>
                                    <LogLevel level={log.level || 'INFO'} />
                                    <span className="font-medium text-gray-800">{log.msg || ''}</span>
                                </div>
                                
                                {log.attrs && Object.keys(log.attrs).length > 0 && (
                                    <div className="flex flex-wrap gap-x-2 gap-y-1 mt-1 text-xs">
                                        {Object.keys(log.attrs).map(key => {
                                            const value = log.attrs![key];
                                            let displayValue = '';
                                            if (typeof value === 'object' && value !== null) {
                                                displayValue = JSON.stringify(value);
                                            } else {
                                                displayValue = String(value);
                                            }
                                            return (
                                                <span key={key} className="bg-white border border-gray-200 px-2 py-0.5 rounded text-gray-600 font-mono">
                                                    <span className="font-bold text-gray-500">{key}:</span> {displayValue}
                                                </span>
                                            );
                                        })}
                                    </div>
                                )}
                            </motion.div>
                        ))}
                    </div>
                )}
            </div>
        </div>
    );
};