import React, {useEffect, useState} from 'react';
import {Activity, AlertTriangle, Loader, Settings} from 'lucide-react';
import ConfigEditor from './features/config/ConfigEditor';
import MonitoringDashboard from './features/monitoring/MonitoringDashboard';
import {api} from './api';
import type {DtoConfig} from './api/generated';
import {SystemApi} from './api/generated/apis/SystemApi';
import {Configuration} from './api/generated/runtime';

const systemApi = new SystemApi(new Configuration({basePath: ''}));

export default function App() {
  const [activeTab, setActiveTab] = useState<'monitor' | 'config'>('monitor');
  const [config, setConfig] = useState<DtoConfig | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [isSystemHealthy, setIsSystemHealthy] = useState(false);

  useEffect(() => {
    const loadConfig = async () => {
      try {
        setError(null);
        setIsLoading(true);
        const fetchedConfig = await api.fetchConfig();
        setConfig(fetchedConfig);
      } catch (e: any) {
        setError(e.message || 'An unknown error occurred.');
      } finally {
        setIsLoading(false);
      }
    };
    loadConfig();

    const checkHealth = async () => {
      try {
        await systemApi.health();
        setIsSystemHealthy(true);
      } catch (e) {
        setIsSystemHealthy(false);
      }
    };

    // Initial health check
    checkHealth();

    // Set up interval for periodic health checks
    const healthInterval = setInterval(checkHealth, 5000); // Check every 5 seconds

    // Clean up interval on component unmount
    return () => clearInterval(healthInterval);
  }, []);

  const renderContent = () => {
    if (isLoading) {
      return (
        <div className="flex flex-col items-center justify-center h-full text-gray-500">
          <Loader className="animate-spin mb-4" size={48} />
          <p>Loading Configuration...</p>
        </div>
      );
    }
    if (error) {
      return (
        <div className="flex flex-col items-center justify-center h-full text-red-400">
          <AlertTriangle className="mb-4" size={48} />
          <p className="font-bold">Failed to load configuration</p>
          <p className="text-sm text-gray-500">{error}</p>
        </div>
      );
    }
    if (config) {
       return activeTab === 'config' ? (
          <ConfigEditor config={config} setConfig={setConfig} />
        ) : (
          <MonitoringDashboard config={config} />
        );
    }
    return null;
  }

  return (
    <div className="h-screen flex flex-col bg-gray-950 text-gray-200 font-sans">
       <header className="bg-gray-900 border-b border-gray-800 px-6 h-16 flex items-center justify-between shrink-0">
          <div className="flex items-center gap-3">
             <div className="w-8 h-8 bg-red-600 rounded flex items-center justify-center font-bold text-white shadow-lg shadow-red-900/20">A</div>
             <h1 className="text-lg font-bold tracking-tight">Aerospike Backup Service</h1>
          </div>
          <nav className="flex bg-gray-950 p-1 rounded-lg border border-gray-800">
             <button onClick={() => setActiveTab('monitor')} className={`px-4 py-1.5 rounded-md text-sm font-medium flex items-center gap-2 transition-colors ${activeTab === 'monitor' ? 'bg-gray-800 text-white shadow-sm' : 'text-gray-400 hover:text-white'}`} disabled={!config}>
                <Activity size={16}/> Monitoring
             </button>
             <button onClick={() => setActiveTab('config')} className={`px-4 py-1.5 rounded-md text-sm font-medium flex items-center gap-2 transition-colors ${activeTab === 'config' ? 'bg-gray-800 text-white shadow-sm' : 'text-gray-400 hover:text-white'}`} disabled={!config}>
                <Settings size={16}/> Configuration
             </button>
          </nav>
          <div className={`flex items-center gap-2 text-xs rounded border px-2 py-1 ${isSystemHealthy ? 'text-green-500 bg-green-900/20 border-green-900/30' : 'text-red-500 bg-red-900/20 border-red-900/30'}`}>
            <span className={`w-1.5 h-1.5 rounded-full ${isSystemHealthy ? 'bg-green-500' : 'bg-red-500'}`}></span>
            {isSystemHealthy ? 'System Online' : 'System Offline'}
          </div>
       </header>
       <main className="flex-1 overflow-hidden">
          {renderContent()}
       </main>
    </div>
  );
}