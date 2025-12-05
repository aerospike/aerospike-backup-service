import React, { useState } from 'react';
import { Activity } from 'lucide-react';
import { Button } from '@/components/ui/Button';

interface ConnectivityCheckButtonProps {
    onCheck: () => Promise<void>;
    className?: string;
    label?: string;
}

export const ConnectivityCheckButton = ({ onCheck, className, label = 'Check Connectivity' }: ConnectivityCheckButtonProps) => {
    const [isChecking, setIsChecking] = useState(false);
    const [checkResult, setCheckResult] = useState<{ type: 'success' | 'error', message: string } | null>(null);

    const handleCheck = async () => {
        setIsChecking(true);
        setCheckResult(null);
        try {
            await onCheck();
            setCheckResult({ type: 'success', message: 'Success' });
        } catch (e) {
            setCheckResult({ type: 'error', message: 'Failed' });
        } finally {
            setIsChecking(false);
            setTimeout(() => setCheckResult(null), 3000);
        }
    };

    return (
        <Button
            variant={checkResult?.type === 'success' ? 'action' : checkResult?.type === 'error' ? 'danger' : 'secondary'}
            className={`transition-all duration-200 ${checkResult ? 'w-32 justify-center' : ''} ${className || ''}`}
            onClick={handleCheck}
            loading={isChecking}
            disabled={!!checkResult}
            icon={!checkResult ? Activity : undefined}
        >
            {checkResult ? checkResult.message : label}
        </Button>
    );
};
