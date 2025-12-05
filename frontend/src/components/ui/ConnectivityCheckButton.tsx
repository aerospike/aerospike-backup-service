import React, { useState } from 'react';
import { Activity, LucideIcon } from 'lucide-react';
import { Button } from '@/components/ui/Button';

interface ConnectivityCheckButtonProps {
    onCheck: () => Promise<void>;
    className?: string;
    label?: string;
    icon?: LucideIcon;
}

export const ConnectivityCheckButton = ({ onCheck, className, label = 'Check Connectivity', icon: CustomIcon }: ConnectivityCheckButtonProps) => {
    const [isChecking, setIsChecking] = useState(false);
    const [checkResult, setCheckResult] = useState<{ type: 'success' | 'error', message: string } | null>(null);

    const handleCheck = async () => {
        setIsChecking(true);
        setCheckResult(null);
        try {
            await onCheck();
            setCheckResult({ type: 'success', message: 'Success' });
        } catch (e: any) { // Keep message simple, no detailed error
            setCheckResult({ type: 'error', message: 'Failed' });
        } finally {
            setIsChecking(false);
            setTimeout(() => setCheckResult(null), 3000);
        }
    };

    const IconToRender = CustomIcon || Activity;

    return (
        <Button
            variant={checkResult?.type === 'success' ? 'action' : checkResult?.type === 'error' ? 'danger' : 'secondary'}
            className={`w-50 min-w-50 max-w-50 justify-center transition-all duration-200 ${className || ''}`}
            onClick={handleCheck}
            loading={isChecking}
            disabled={!!checkResult}
            icon={!checkResult ? IconToRender : undefined}
        >
            {checkResult ? checkResult.message : label}
        </Button>
    );
};
