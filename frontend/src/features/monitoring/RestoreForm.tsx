import React, {useEffect, useState} from 'react';
import {
    DtoCompressionPolicy,
    DtoCompressionPolicyModeEnum,
    DtoConfig,
    DtoEncryptionPolicy,
    DtoEncryptionPolicyModeEnum,
    DtoRestoreNamespace,
    DtoRestorePolicy,
    DtoRestoreTimestampRequest,
    DtoRetryPolicy
} from '@/api';
import {Button} from '@/components/ui/Button';
import {Modal} from '@/components/ui/Feedback';
import {Checkbox, Input, MultiSelect, RadioGroup, Select} from '@/components/ui/Inputs';
import {Database, Filter, Lock, RefreshCcw, Settings, Zap} from 'lucide-react';

interface RestoreFormProps {
    isOpen: boolean;
    onClose: () => void;
    config: DtoConfig;
    activeRoutine: string;
    timestamp: number;
    chainLength: number;
    onRestore: (request: DtoRestoreTimestampRequest) => Promise<void>;
    isRestoring: boolean;
    restoreError: string | null;
}

const SECTIONS = [
    {id: 'general', label: 'General', icon: Settings},
    {id: 'performance', label: 'Performance', icon: Zap},
    {id: 'filters', label: 'Filters & Namespaces', icon: Filter},
    {id: 'compression', label: 'Compression & Encryption', icon: Lock},
    {id: 'records', label: 'Record Handling', icon: Database},
    {id: 'retry', label: 'Retry Logic', icon: RefreshCcw},
];

export const RestoreForm = ({
                                isOpen,
                                onClose,
                                config,
                                activeRoutine,
                                timestamp,
                                chainLength,
                                onRestore,
                                isRestoring,
                                restoreError
                            }: RestoreFormProps) => {
    const clusters = Object.keys(config.aerospikeClusters || {});
    const [activeSection, setActiveSection] = useState('general');

    const [restoreDestinationName, setRestoreDestinationName] = useState<string>('');

    // Policy State
    const [bandwidth, setBandwidth] = useState<number | undefined>(undefined);
    const [batchSize, setBatchSize] = useState<number | undefined>(undefined);
    const [binList, setBinList] = useState<string[]>([]);
    const [disableBatchWrites, setDisableBatchWrites] = useState<boolean>(false);
    const [extraTtl, setExtraTtl] = useState<number | undefined>(undefined);
    const [maxAsyncBatches, setMaxAsyncBatches] = useState<number | undefined>(undefined);
    const [noIndexes, setNoIndexes] = useState<boolean>(false);
    const [noRecords, setNoRecords] = useState<boolean>(false);
    const [noUdfs, setNoUdfs] = useState<boolean>(false);
    const [parallel, setParallel] = useState<number | undefined>(undefined);
    const [setList, setSetList] = useState<string[]>([]);
    const [socketTimeout, setSocketTimeout] = useState<number | undefined>(undefined);
    const [totalTimeout, setTotalTimeout] = useState<number | undefined>(undefined);
    const [tps, setTps] = useState<number | undefined>(undefined);

    const [recordHandlingMode, setRecordHandlingMode] = useState<string>("default");

    // Compression Policy State
    const [compressionMode, setCompressionMode] = useState<DtoCompressionPolicyModeEnum | undefined>(undefined);
    const [compressionLevel, setCompressionLevel] = useState<number | undefined>(undefined);

    // Encryption Policy State
    const [encryptionMode, setEncryptionMode] = useState<DtoEncryptionPolicyModeEnum | undefined>(undefined);
    const [encryptionKeyEnv, setEncryptionKeyEnv] = useState<string>('');
    const [encryptionKeyFile, setEncryptionKeyFile] = useState<string>('');
    const [encryptionKeySecret, setEncryptionKeySecret] = useState<string>('');

    // Namespace Policy State
    const [namespaceSource, setNamespaceSource] = useState<string>('');
    const [namespaceDestination, setNamespaceDestination] = useState<string>('');

    // Retry Policy State
    const [retryBaseTimeout, setRetryBaseTimeout] = useState<number | undefined>(undefined);
    const [retryMaxRetries, setRetryMaxRetries] = useState<number | undefined>(undefined);
    const [retryMultiplier, setRetryMultiplier] = useState<number | undefined>(undefined);

    const [restoreDisableReordering, setRestoreDisableReordering] = useState<boolean>(false);

    // Set default cluster
    useEffect(() => {
        if (isOpen && clusters.length > 0 && !restoreDestinationName) {
            setRestoreDestinationName(clusters[0] || '');
        }
    }, [isOpen, clusters, restoreDestinationName]);

    const getRestorePolicy = (): DtoRestorePolicy | undefined => {
        const policy: DtoRestorePolicy = {};

        if (bandwidth !== undefined) policy.bandwidth = bandwidth;
        if (batchSize !== undefined) policy.batchSize = batchSize;
        if (binList.length > 0) policy.binList = binList;
        if (disableBatchWrites) policy.disableBatchWrites = disableBatchWrites;
        if (extraTtl !== undefined) policy.extraTtl = extraTtl;
        if (maxAsyncBatches !== undefined) policy.maxAsyncBatches = maxAsyncBatches;

        // Set record handling policy
        if (recordHandlingMode === 'alwaysOverwrite') policy.noGeneration = true;
        if (recordHandlingMode === 'replaceRecord') policy.replace = true;
        if (recordHandlingMode === 'skipExisting') policy.unique = true;

        if (noIndexes) policy.noIndexes = noIndexes;
        if (noRecords) policy.noRecords = noRecords;
        if (noUdfs) policy.noUdfs = noUdfs;
        if (parallel !== undefined) policy.parallel = parallel;
        if (setList.length > 0) policy.setList = setList;
        if (socketTimeout !== undefined) policy.socketTimeout = socketTimeout;
        if (totalTimeout !== undefined) policy.totalTimeout = totalTimeout;
        if (tps !== undefined) policy.tps = tps;

        // Compression Policy
        const compressionPolicy: DtoCompressionPolicy = {};
        if (compressionMode && compressionMode !== DtoCompressionPolicyModeEnum.None) {
            compressionPolicy.mode = compressionMode;
            if (compressionLevel !== undefined) compressionPolicy.level = compressionLevel;
            policy.compression = compressionPolicy;
        }

        // Encryption Policy
        const encryptionPolicy: DtoEncryptionPolicy = {};
        if (encryptionMode && encryptionMode !== DtoEncryptionPolicyModeEnum.None) {
            encryptionPolicy.mode = encryptionMode;
            if (encryptionKeyEnv) encryptionPolicy.keyEnv = encryptionKeyEnv;
            if (encryptionKeyFile) encryptionPolicy.keyFile = encryptionKeyFile;
            if (encryptionKeySecret) encryptionPolicy.keySecret = encryptionKeySecret;
            policy.encryption = encryptionPolicy;
        }

        // Namespace Policy
        const restoreNamespace: DtoRestoreNamespace = {destination: "", source: ""};
        if (namespaceSource) restoreNamespace.source = namespaceSource;
        if (namespaceDestination) restoreNamespace.destination = namespaceDestination;

        if (Object.keys(restoreNamespace).length > 0) {
            policy.namespace = restoreNamespace;
        }


        // Retry Policy
        const retryPolicy: DtoRetryPolicy = {};
        if (retryBaseTimeout !== undefined) retryPolicy.baseTimeout = retryBaseTimeout;
        if (retryMaxRetries !== undefined) retryPolicy.maxRetries = retryMaxRetries;
        if (retryMultiplier !== undefined) retryPolicy.multiplier = retryMultiplier;
        if (Object.keys(retryPolicy).length > 0) {
            policy.retryPolicy = retryPolicy;
        }


        // Check if any policy field is set. If not, return undefined to indicate no custom policy
        if (Object.keys(policy).length === 0) {
            return undefined;
        }

        return policy;
    };

    const handleRestoreClick = () => {
        if (!restoreDestinationName) return;

        const request: DtoRestoreTimestampRequest = {
            routine: activeRoutine,
            time: timestamp,
            destinationName: restoreDestinationName,
            policy: getRestorePolicy(),
            disableReordering: restoreDisableReordering || undefined,
        };
        onRestore(request);
    };

    const renderContent = () => {
        switch (activeSection) {
            case 'general':
                return (
                    <div className="space-y-4 animate-in fade-in slide-in-from-bottom-2 duration-200">
                        <div className="bg-gray-50 p-4 rounded-lg border border-gray-200 mb-4">
                            <h3 className="text-sm font-bold text-gray-900 mb-2">Restore Summary</h3>
                            <div className="grid grid-cols-2 gap-4 text-sm">
                                <div>
                                    <p className="text-gray-600 text-xs uppercase font-bold">Point-in-time</p>
                                    <p className="font-mono text-gray-900">{new Date(timestamp).toLocaleString()}</p>
                                </div>
                                <div>
                                    <p className="text-gray-600 text-xs uppercase font-bold">Backups in sequence</p>
                                    <p className="font-mono text-aerospike-dark">{chainLength}</p>
                                </div>
                            </div>
                        </div>

                        <Select
                            label="Destination Cluster"
                            value={restoreDestinationName}
                            options={clusters.map(k => ({label: k, value: k}))}
                            onChange={(e: React.ChangeEvent<HTMLSelectElement>) => setRestoreDestinationName(e.target.value)}
                            description="Select the Aerospike cluster where the data will be restored."
                            required
                        />

                        {chainLength > 1 && (
                            <Checkbox
                                label="Disable Incremental Reordering"
                                checked={restoreDisableReordering}
                                onChange={setRestoreDisableReordering}
                                description="If checked, the backup service will not attempt to reorder the incremental backups based on their timestamps."
                            />
                        )}
                    </div>
                );
            case 'performance':
                return (
                    <div className="grid grid-cols-1 gap-4 animate-in fade-in slide-in-from-bottom-2 duration-200">
                        <Input
                            label="Parallel"
                            type="number"
                            value={parallel || ''}
                            onChange={(e: React.ChangeEvent<HTMLInputElement>) => setParallel(Number(e.target.value) || undefined)}
                            description="The number of concurrent record readers from backup files. This value controls the level of parallelism used by the backup service when reading backup files. The optimal value depends on hardware and network configuration."
                        />
                        <Input
                            label="Batch Size"
                            type="number"
                            value={batchSize || ''}
                            onChange={(e: React.ChangeEvent<HTMLInputElement>) => setBatchSize(Number(e.target.value) || undefined)}
                            description="The max allowed number of records per an async batch write call. Only applicable when using batch writes."
                        />
                        <Input
                            label="Max Async Batches"
                            type="number"
                            value={maxAsyncBatches || ''}
                            onChange={(e: React.ChangeEvent<HTMLInputElement>) => setMaxAsyncBatches(Number(e.target.value) || undefined)}
                            description="The max number of outstanding async record batch write calls at a time."
                        />
                        <Input
                            label="Bandwidth (MiB/s)"
                            type="number"
                            value={bandwidth || ''}
                            onChange={(e: React.ChangeEvent<HTMLInputElement>) => setBandwidth(Number(e.target.value) || undefined)}
                            description="Throttles read operations from the backup file(s) to not exceed the given I/O bandwidth in MiB/s. Default: no limit."
                        />
                        <Input
                            label="TPS"
                            type="number"
                            value={tps || ''}
                            onChange={(e: React.ChangeEvent<HTMLInputElement>) => setTps(Number(e.target.value) || undefined)}
                            description="Throttles read operations from the backup file(s) to not exceed the given number of transactions per second. Default: no limit."
                        />
                        <Input
                            label="Socket Timeout (ms)"
                            type="number"
                            value={socketTimeout || ''}
                            onChange={(e: React.ChangeEvent<HTMLInputElement>) => setSocketTimeout(Number(e.target.value) || undefined)}
                            description="Timeout (ms) for Aerospike commands to write records, create indexes and create UDFs. Socket timeout in milliseconds. Default is 10 minutes. If this value is 0, it is set to total-timeout. If both are 0, there is no socket idle time limit."
                        />
                        <Input
                            label="Total Timeout (ms)"
                            type="number"
                            value={totalTimeout || ''}
                            onChange={(e: React.ChangeEvent<HTMLInputElement>) => setTotalTimeout(Number(e.target.value) || undefined)}
                            description="Total socket timeout in milliseconds. Default is 0, that is, no timeout."
                        />
                        <Checkbox
                            label="Disable Batch Writes"
                            checked={disableBatchWrites}
                            onChange={setDisableBatchWrites}
                            description="Disables the use of batch writes when restoring records to the Aerospike cluster. By default, the cluster is checked for batch write support."
                        />
                    </div>
                );
            case 'filters':
                return (
                    <div className="space-y-6 animate-in fade-in slide-in-from-bottom-2 duration-200">
                        <div className="space-y-4">
                            <MultiSelect
                                label="Bin List"
                                value={binList}
                                onChange={setBinList}
                                options={[]}
                                placeholder="Optional"
                                description="List of bins to restore. Only the bins specified here will be restored. Empty implies restoring all bins."
                            />
                            <MultiSelect
                                label="Set List"
                                value={setList}
                                onChange={setSetList}
                                options={[]}
                                placeholder="Optional"
                                description="List of sets to restore. Only the sets specified here will be restored. Empty implies restoring all sets."
                            />
                        </div>
                        <div className="border-t border-gray-200 pt-4 space-y-4">
                            <h4 className="text-sm font-semibold text-gray-700">Namespace Remapping</h4>
                            <Input
                                label="Source Namespace"
                                value={namespaceSource}
                                onChange={(e: React.ChangeEvent<HTMLInputElement>) => setNamespaceSource(e.target.value)}
                                description="Original namespace name. This field is required as a safeguard to ensure intentional namespace remapping."
                            />
                            <Input
                                label="Destination Namespace"
                                value={namespaceDestination}
                                onChange={(e: React.ChangeEvent<HTMLInputElement>) => setNamespaceDestination(e.target.value)}
                                description="Target namespace name to map the source namespace to."
                            />
                        </div>
                    </div>
                );
            case 'compression':
                return (
                    <div className="space-y-6 animate-in fade-in slide-in-from-bottom-2 duration-200">
                        <div className="space-y-4">
                            <h4 className="text-sm font-semibold text-gray-700">Compression</h4>
                            <Select
                                label="Compression Mode"
                                value={compressionMode || ''}
                                options={[
                                    {label: "None", value: DtoCompressionPolicyModeEnum.None},
                                    {label: "ZSTD", value: DtoCompressionPolicyModeEnum.Zstd}
                                ]}
                                onChange={(e: React.ChangeEvent<HTMLSelectElement>) => setCompressionMode(e.target.value as DtoCompressionPolicyModeEnum)}
                                description="Compression algorithm to use for the backup files. Default is no compression."
                            />
                            {compressionMode === DtoCompressionPolicyModeEnum.Zstd && (
                                <Input
                                    label="Level"
                                    type="number"
                                    value={compressionLevel || ''}
                                    onChange={(e: React.ChangeEvent<HTMLInputElement>) => setCompressionLevel(Number(e.target.value) || undefined)}
                                    placeholder="-1 to 22"
                                    description="The compression level to use. Algorithm-specific; for zstd: from -1 (fastest) to 22 (best compression)."
                                />
                            )}
                        </div>
                        <div className="border-t border-gray-200 pt-4 space-y-4">
                            <h4 className="text-sm font-semibold text-gray-700">Encryption</h4>
                            <Select
                                label="Encryption Mode"
                                value={encryptionMode || ''}
                                options={[
                                    {label: "None", value: DtoEncryptionPolicyModeEnum.None},
                                    {label: "AES128", value: DtoEncryptionPolicyModeEnum.Aes128},
                                    {label: "AES256", value: DtoEncryptionPolicyModeEnum.Aes256}
                                ]}
                                onChange={(e: React.ChangeEvent<HTMLSelectElement>) => setEncryptionMode(e.target.value as DtoEncryptionPolicyModeEnum)}
                                description="Encryption algorithm to use for the backup files. Default is no encryption."
                            />
                            {encryptionMode !== DtoEncryptionPolicyModeEnum.None && (
                                <>
                                    <Input
                                        label="Key Env Var"
                                        value={encryptionKeyEnv}
                                        onChange={(e: React.ChangeEvent<HTMLInputElement>) => setEncryptionKeyEnv(e.target.value)}
                                        description="The name of the environment variable containing the encryption key."
                                    />
                                    <Input
                                        label="Key File"
                                        value={encryptionKeyFile}
                                        onChange={(e: React.ChangeEvent<HTMLInputElement>) => setEncryptionKeyFile(e.target.value)}
                                        description="The path to the file containing the encryption key."
                                    />
                                    <Input
                                        label="Key Secret"
                                        value={encryptionKeySecret}
                                        onChange={(e: React.ChangeEvent<HTMLInputElement>) => setEncryptionKeySecret(e.target.value)}
                                        description="The secret keyword in Aerospike Secret Agent containing the encryption key."
                                    />
                                </>
                            )}
                        </div>
                    </div>
                );
            case 'records':
                return (
                    <div className="space-y-6 animate-in fade-in slide-in-from-bottom-2 duration-200">
                        <Input
                            label="Extra TTL (s)"
                            type="number"
                            value={extraTtl || ''}
                            onChange={(e: React.ChangeEvent<HTMLInputElement>) => setExtraTtl(Number(e.target.value) || undefined)}
                            description="Amount of extra time-to-live to add to records that have expirable void-times. Must be set in seconds."
                        />
                        <RadioGroup
                            label="Existing Records Mode"
                            value={recordHandlingMode || 'default'}
                            onChange={(val) => setRecordHandlingMode(val)}
                            options={[
                                {
                                    label: "Default (Merge)",
                                    value: "default",
                                    description: "Merge backup bins into existing record if backup generation is higher or equal."
                                },
                                {
                                    label: "Always Overwrite (Merge)",
                                    value: "alwaysOverwrite",
                                    description: "Merge backup bins into existing record regardless of generation."
                                },
                                {
                                    label: "Replace Record",
                                    value: "replaceRecord",
                                    description: "Completely replace existing record (removing other bins) if backup generation is higher or equal."
                                },
                                {
                                    label: "Skip Existing",
                                    value: "skipExisting",
                                    description: "Do not restore if record already exists."
                                }
                            ]}
                        />
                        <div className="space-y-3 border-t border-gray-200 pt-4">
                            <h4 className="text-sm font-semibold text-gray-700">Flags</h4>
                            <Checkbox
                                label="No Indexes"
                                checked={noIndexes}
                                onChange={setNoIndexes}
                                description="Do not restore any secondary index definitions."
                            />
                            <Checkbox
                                label="No Records"
                                checked={noRecords}
                                onChange={setNoRecords}
                                description="Do not restore any record data (metadata or bin data). By default, record data, secondary index definitions, and UDF modules will be restored."
                            />
                            <Checkbox
                                label="No UDFs"
                                checked={noUdfs}
                                onChange={setNoUdfs}
                                description="Do not restore any UDF modules."
                            />
                        </div>
                    </div>
                );
            case 'retry':
                return (
                    <div className="grid grid-cols-1 gap-4 animate-in fade-in slide-in-from-bottom-2 duration-200">
                        <Input
                            label="Base Timeout (ms)"
                            type="number"
                            value={retryBaseTimeout || ''}
                            onChange={(e: React.ChangeEvent<HTMLInputElement>) => setRetryBaseTimeout(Number(e.target.value) || undefined)}
                            description="BaseTimeout is the initial delay between retry attempts, in milliseconds."
                        />
                        <Input
                            label="Max Retries"
                            type="number"
                            value={retryMaxRetries || ''}
                            onChange={(e: React.ChangeEvent<HTMLInputElement>) => setRetryMaxRetries(Number(e.target.value) || undefined)}
                            description="MaxRetries is the maximum number of retry attempts that will be made. If set to 0, no retries will be performed."
                        />
                        <Input
                            label="Multiplier"
                            type="number"
                            value={retryMultiplier || ''}
                            onChange={(e: React.ChangeEvent<HTMLInputElement>) => setRetryMultiplier(Number(e.target.value) || undefined)}
                            description="Multiplier is used to increase the delay between subsequent retry attempts. The actual delay is calculated as: BaseTimeout * (Multiplier ^ attemptNumber)."
                        />
                    </div>
                );
            default:
                return null;
        }
    };

    return (
        <Modal isOpen={isOpen} onClose={onClose} title="Confirm Restore" maxWidth="max-w-4xl">
            <div className="flex flex-col h-[calc(100vh-10rem)] max-h-[700px] gap-4">
                {/* Sidebar and Main Content Area */}
                <div className="flex flex-1 min-h-0">
                    {/* Sidebar */}
                    <div className="w-64 bg-gray-50 border-r border-gray-200 p-4 flex flex-col gap-1">
                        {SECTIONS.map((section) => {
                            const Icon = section.icon;
                            return (
                                <button
                                    key={section.id}
                                    onClick={() => setActiveSection(section.id)}
                                    className={`w-full text-left px-3 py-2 rounded flex items-center gap-3 text-sm font-medium transition-colors ${
                                        activeSection === section.id
                                            ? 'bg-aerospike-primary text-black'
                                            : 'text-gray-600 hover:bg-gray-200 hover:text-gray-900'
                                    }`}
                                >
                                    <Icon size={16}/>
                                    {section.label}
                                </button>
                            );
                        })}
                    </div>

                    {/* Content Area */}
                    <div className="flex-1 bg-white p-6 overflow-y-scroll custom-scrollbar">
                        {renderContent()}
                    </div>
                </div>

                {/* Footer with buttons and error message */}
                <div className="flex justify-between items-center pt-4 border-t border-gray-200">
                    <div className="text-red-600 text-sm font-semibold flex-grow">
                        {restoreError && `Error: ${restoreError}`}
                    </div>
                    <div className="flex gap-2">
                        <Button variant="ghost" onClick={onClose} disabled={isRestoring}>Cancel</Button>
                        <Button variant="action" onClick={handleRestoreClick}
                                disabled={isRestoring || !restoreDestinationName}>
                            {isRestoring ? 'Restoring...' : 'Start Restore'}
                        </Button>
                    </div>
                </div>
            </div>
        </Modal>
    );
};