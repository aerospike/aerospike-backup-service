// --- Configuration Types ---

export interface SeedNode {
  "host-name": string;
  port: number;
}

export interface Credentials {
  user: string;
  password?: string;
  "auth-mode"?: "INTERNAL" | "EXTERNAL" | "PKI";
}

export interface Cluster {
  "seed-nodes": SeedNode[];
  credentials?: Credentials;
  "max-parallel-scans"?: number;
  "conn-timeout"?: number;
}

export interface StorageS3 {
  bucket: string;
  "s3-region": string;
  path?: string;
  "access-key-id"?: string;
  "secret-access-key"?: string;
  "s3-endpoint-override"?: string;
}

export interface StorageLocal {
  path: string;
}

export interface StorageConfig {
  "s3-storage"?: StorageS3;
  "local-storage"?: StorageLocal;
  "gcp-storage"?: Record<string, any>;
  "azure-storage"?: Record<string, any>;
}

export interface Policy {
  parallel: number;
  "parallel-write"?: number;
  "file-limit"?: number;
  compression?: { mode: "NONE" | "ZSTD"; level?: number };
  encryption?: { mode: "NONE" | "AES128" | "AES256"; "key-file"?: string };
  retention?: { full: number; incremental: number };
}

export interface Routine {
  "source-cluster": string;
  storage: string;
  "interval-cron": string;
  "incr-interval-cron"?: string;
  "backup-policy"?: string;
  namespaces?: string[];
  "set-list"?: string[];
  disabled?: boolean;
}

export interface ServiceConfig {
  http?: { port: number; "context-path"?: string };
  logger?: { level: string; format: string; "file-writer"?: any };
}

export interface AppConfig {
  "aerospike-clusters": Record<string, Cluster>;
  storage: Record<string, StorageConfig>;
  "backup-policies": Record<string, Policy>;
  "secret-agents": Record<string, any>;
  "backup-routines": Record<string, Routine>;
  service: ServiceConfig;
}

// --- Monitoring Types ---

export interface Metrics {
  "kilobytes-per-second": number;
  "pipeline": number;
  "records-per-second": number;
}

export interface RunningJob {
  "done-records": number;
  "duration": number;
  "estimated-end-time": string;
  "finish-time": string;
  "metrics": Metrics;
  "percentage-done": number;
  "start-time": string;
  "total-records": number;
}

export interface CurrentBackup {
  "full": RunningJob | null;
  "incremental": RunningJob | null;
  "last-full": string;
  "last-incremental": string;
  "next-full": string;
  "next-incremental": string;
}

export interface Backup {
  "byte-count": number;
  compression: string;
  created: string;
  duration: number; // now a number in seconds
  encryption: string;
  "file-count": number;
  finished: string;
  from: string;
  key: string; // Renamed from 'id' to 'key'
  namespace: string;
  "record-count": number;
  "secondary-index-count": number;
  storage: any; // Using any for now as dto.Storage definition is not provided
  timestamp: number;
  "udf-count": number;
  type: 'Full' | 'Incremental'; // Added for UI logic
}