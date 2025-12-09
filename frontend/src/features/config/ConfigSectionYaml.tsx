import React, { useState } from 'react';
import {CheckCircle, Copy, Loader, Play} from 'lucide-react'; // Assuming Loader is from lucide-react or similar
import {Button} from '@/components/ui/Button';
import {Toast} from '@/components/ui/Toast'; // Import Toast
import {toYaml} from '@/utils/yaml';
import {DtoConfig} from '@/api';

interface ConfigSectionYamlProps {
  config: DtoConfig;
  handleApply: () => Promise<void>;
  isSaving: boolean;
  saveError: string | null;
}

export const ConfigSectionYaml = (
  { config, handleApply, isSaving, saveError }: ConfigSectionYamlProps
) => {
  const [showToast, setShowToast] = useState(false);
  const [toastMessage, setToastMessage] = useState('');
  const [toastType, setToastType] = useState<'success' | 'error'>('success');

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(toYaml(config));
      setToastMessage('Configuration copied to clipboard!');
      setToastType('success');
    } catch (err) {
      setToastMessage('Failed to copy configuration.');
      setToastType('error');
    } finally {
      setShowToast(true);
    }
  };

  return (
    <div className="max-w-2xl mx-auto text-center animate-in fade-in slide-in-from-bottom-4 pt-10">
        <CheckCircle size={64} className="text-green-500 mx-auto mb-6" />
        <h2 className="text-2xl font-bold text-gray-900 mb-2">Ready to Apply?</h2>
        <div className="w-full bg-gray-50 p-4 rounded text-left font-mono text-xs text-gray-600 border border-gray-200 mb-6 h-96 relative overflow-y-auto">
            <pre>{toYaml(config)}</pre>
            <button onClick={handleCopy} className="absolute top-2 right-2 p-1 rounded-md text-gray-500 hover:bg-gray-200">
                <Copy size={16} />
            </button>
        </div>
        {saveError && (
          <div className="bg-red-100 border border-red-200 text-red-700 text-sm p-3 rounded mb-4">
            <strong>Error:</strong> {saveError}
          </div>
        )}
        <div className="flex justify-center gap-4">
            <Button variant="action" onClick={handleApply} icon={isSaving ? Loader : Play} disabled={isSaving}>
              {isSaving ? 'Saving...' : 'Apply Configuration'}
            </Button>
        </div>
        {showToast && <Toast message={toastMessage} type={toastType} onClose={() => setShowToast(false)} />}
    </div>
  );
};
