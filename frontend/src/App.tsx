import React, { useState } from 'react';
import { Activity, Settings } from 'lucide-react';
import ConfigEditor from './features/config/ConfigEditor';
import MonitoringDashboard from './features/monitoring/MonitoringDashboard';
import { AppConfig } from './types';

// Initial state matching the AppConfig interface
const initialConfig: AppConfig = {
  "aerospike-clusters": {},
  "storage": {},
  "backup-policies": {},
  "secret-agents": {},
  "backup-routines": {
      "daily-demo": {
          "source-cluster": "test",
          "storage": "s3",
          "interval-cron": "@daily"
      }
  },
  "service": {
      logger: { level: "INFO", format: "PLAIN" }
  }
};

export default function App() {
  const [activeTab, setActiveTab] = useState<'monitor' | 'config'>('monitor');
  const [config, setConfig] = useState<AppConfig>(initialConfig);

  return (
    <div className="h-screen flex flex-col bg-gray-950 text-gray-200 font-sans">
       <header className="bg-gray-900 border-b border-gray-800 px-6 h-16 flex items-center justify-between shrink-0">
          <div className="flex items-center gap-3">
             <div className="w-8 h-8 bg-red-600 rounded flex items-center justify-center font-bold text-white shadow-lg shadow-red-900/20">A</div>
             <h1 className="text-lg font-bold tracking-tight">Aerospike Backup Service</h1>
          </div>
          <nav className="flex bg-gray-950 p-1 rounded-lg border border-gray-800">
             <button onClick={() => setActiveTab('monitor')} className={`px-4 py-1.5 rounded-md text-sm font-medium flex items-center gap-2 transition-colors ${activeTab === 'monitor' ? 'bg-gray-800 text-white shadow-sm' : 'text-gray-400 hover:text-white'}`}>
                <Activity size={16}/> Monitoring
             </button>
             <button onClick={() => setActiveTab('config')} className={`px-4 py-1.5 rounded-md text-sm font-medium flex items-center gap-2 transition-colors ${activeTab === 'config' ? 'bg-gray-800 text-white shadow-sm' : 'text-gray-400 hover:text-white'}`}>
                <Settings size={16}/> Configuration
             </button>
          </nav>
          <div className="flex items-center gap-2 text-xs text-green-500 bg-green-900/20 px-2 py-1 rounded border border-green-900/30">
            <span className="w-1.5 h-1.5 rounded-full bg-green-500 animate-pulse"></span>
            System Online
          </div>
       </header>
       <main className="flex-1 overflow-hidden">
          {activeTab === 'config' ? (
              <ConfigEditor config={config} setConfig={setConfig} />
          ) : (
              <MonitoringDashboard config={config} />
          )}
       </main>
    </div>
  );
}