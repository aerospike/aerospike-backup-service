package converter

import (
	"github.com/aerospike/aerospike-backup-service/internal/server/dto"
	"github.com/aerospike/aerospike-backup-service/pkg/model"
)

func DTOToModelConfig(dtoConfig *dto.Config) *model.Config {
	if dtoConfig == nil {
		return nil
	}

	modelConfig := &model.Config{
		ServiceConfig:     DTOTodtoackupServiceConfig(dtoConfig.ServiceConfig),
		AerospikeClusters: make(map[string]*model.AerospikeCluster),
		Storage:           make(map[string]*model.Storage),
		BackupPolicies:    make(map[string]*model.BackupPolicy),
		BackupRoutines:    make(map[string]*model.BackupRoutine),
		SecretAgents:      make(map[string]*model.SecretAgent),
	}

	for k, v := range dtoConfig.AerospikeClusters {
		modelConfig.AerospikeClusters[k] = DTOToModelAerospikeCluster(v)
	}

	for k, v := range dtoConfig.Storage {
		modelConfig.Storage[k] = DTOToModelStorage(v)
	}

	for k, v := range dtoConfig.BackupPolicies {
		modelConfig.BackupPolicies[k] = DTOTodtoackupPolicy(v)
	}

	for k, v := range dtoConfig.SecretAgents {
		modelConfig.SecretAgents[k] = DTOToModelSecretAgent(v)
	}

	// Convert BackupRoutines last to ensure all referenced entities are available
	for k, v := range dtoConfig.BackupRoutines {
		modelConfig.BackupRoutines[k] = DTOTodtoackupRoutine(v, modelConfig)
	}

	return modelConfig
}

func DTOTodtoackupServiceConfig(dto *dto.BackupServiceConfig) *model.BackupServiceConfig {
	if dto == nil {
		return nil
	}
	return &model.BackupServiceConfig{
		HTTPServer: DTOToModelHTTPServerConfig(dto.HTTPServer),
		Logger:     DTOToModelLoggerConfig(dto.Logger),
	}
}

func DTOToModelHTTPServerConfig(dto *dto.HTTPServerConfig) *model.HTTPServerConfig {
	if dto == nil {
		return nil
	}
	return &model.HTTPServerConfig{
		Address:     dto.Address,
		Port:        dto.Port,
		Rate:        DTOToModelRateLimiterConfig(dto.Rate),
		ContextPath: dto.ContextPath,
		Timeout:     dto.Timeout,
	}
}

func DTOToModelRateLimiterConfig(dto *dto.RateLimiterConfig) *model.RateLimiterConfig {
	if dto == nil {
		return nil
	}
	return &model.RateLimiterConfig{
		Tps:       dto.Tps,
		Size:      dto.Size,
		WhiteList: dto.WhiteList,
	}
}

func DTOToModelLoggerConfig(dto *dto.LoggerConfig) *model.LoggerConfig {
	if dto == nil {
		return nil
	}
	return &model.LoggerConfig{
		Level:        dto.Level,
		Format:       dto.Format,
		StdoutWriter: dto.StdoutWriter,
		FileWriter:   DTOToModelFileLoggerConfig(dto.FileWriter),
	}
}

func DTOToModelFileLoggerConfig(dto *dto.FileLoggerConfig) *model.FileLoggerConfig {
	if dto == nil {
		return nil
	}
	return &model.FileLoggerConfig{
		Filename:   dto.Filename,
		MaxSize:    dto.MaxSize,
		MaxAge:     dto.MaxAge,
		MaxBackups: dto.MaxBackups,
		Compress:   dto.Compress,
	}
}

func DTOToModelAerospikeCluster(dto *dto.AerospikeCluster) *model.AerospikeCluster {
	if dto == nil {
		return nil
	}
	return &model.AerospikeCluster{
		ClusterLabel:         dto.ClusterLabel,
		SeedNodes:            DTOToModelSeedNodes(dto.SeedNodes),
		ConnTimeout:          dto.ConnTimeout,
		UseServicesAlternate: dto.UseServicesAlternate,
		Credentials:          DTOToModelCredentials(dto.Credentials),
		TLS:                  DTOToModelTLS(dto.TLS),
		MaxParallelScans:     dto.MaxParallelScans,
	}
}

func DTOToModelSeedNodes(dtos []dto.SeedNode) []model.SeedNode {
	nodes := make([]model.SeedNode, len(dtos))
	for i, d := range dtos {
		nodes[i] = model.SeedNode{
			HostName: d.HostName,
			Port:     d.Port,
			TLSName:  d.TLSName,
		}
	}
	return nodes
}

func DTOToModelCredentials(dto *dto.Credentials) *model.Credentials {
	if dto == nil {
		return nil
	}
	return &model.Credentials{
		User:         dto.User,
		Password:     dto.Password,
		PasswordPath: dto.PasswordPath,
		AuthMode:     dto.AuthMode,
	}
}

func DTOToModelTLS(dto *dto.TLS) *model.TLS {
	if dto == nil {
		return nil
	}
	return &model.TLS{
		CAFile:          dto.CAFile,
		CAPath:          dto.CAPath,
		Name:            dto.Name,
		Protocols:       dto.Protocols,
		CipherSuite:     dto.CipherSuite,
		Keyfile:         dto.Keyfile,
		KeyfilePassword: dto.KeyfilePassword,
		Certfile:        dto.Certfile,
	}
}

func DTOToModelStorage(dto *dto.Storage) *model.Storage {
	if dto == nil {
		return nil
	}
	return &model.Storage{
		Type:               model.StorageType(dto.Type),
		Path:               dto.Path,
		S3Region:           dto.S3Region,
		S3Profile:          dto.S3Profile,
		S3EndpointOverride: dto.S3EndpointOverride,
		S3LogLevel:         dto.S3LogLevel,
		MinPartSize:        dto.MinPartSize,
		MaxConnsPerHost:    dto.MaxConnsPerHost,
	}
}

func DTOTodtoackupPolicy(dto *dto.BackupPolicy) *model.BackupPolicy {
	if dto == nil {
		return nil
	}
	return &model.BackupPolicy{
		Parallel:          dto.Parallel,
		SocketTimeout:     dto.SocketTimeout,
		TotalTimeout:      dto.TotalTimeout,
		MaxRetries:        dto.MaxRetries,
		RetryDelay:        dto.RetryDelay,
		RemoveFiles:       (*model.RemoveFilesType)(dto.RemoveFiles),
		NoRecords:         dto.NoRecords,
		NoIndexes:         dto.NoIndexes,
		NoUdfs:            dto.NoUdfs,
		Bandwidth:         dto.Bandwidth,
		RecordsPerSecond:  dto.RecordsPerSecond,
		FileLimit:         dto.FileLimit,
		EncryptionPolicy:  DTOToModelEncryptionPolicy(dto.EncryptionPolicy),
		CompressionPolicy: DTOToModelCompressionPolicy(dto.CompressionPolicy),
		Sealed:            dto.Sealed,
	}
}

func DTOToModelEncryptionPolicy(dto *dto.EncryptionPolicy) *model.EncryptionPolicy {
	if dto == nil {
		return nil
	}
	return &model.EncryptionPolicy{
		Mode:      dto.Mode,
		KeyFile:   dto.KeyFile,
		KeyEnv:    dto.KeyEnv,
		KeySecret: dto.KeySecret,
	}
}

func DTOToModelCompressionPolicy(dto *dto.CompressionPolicy) *model.CompressionPolicy {
	if dto == nil {
		return nil
	}
	return &model.CompressionPolicy{
		Mode:  dto.Mode,
		Level: dto.Level,
	}
}

func DTOTodtoackupRoutine(dto *dto.BackupRoutine, config *model.Config) *model.BackupRoutine {
	if dto == nil {
		return nil
	}
	var agent *model.SecretAgent
	if dto.SecretAgent != nil {
		agent = config.SecretAgents[*dto.SecretAgent]
	}
	return &model.BackupRoutine{
		BackupPolicy:     config.BackupPolicies[dto.BackupPolicy],
		SourceCluster:    dto.SourceCluster,
		Storage:          config.Storage[dto.Storage],
		SecretAgent:      agent,
		IntervalCron:     dto.IntervalCron,
		IncrIntervalCron: dto.IncrIntervalCron,
		Namespaces:       dto.Namespaces,
		SetList:          dto.SetList,
		BinList:          dto.BinList,
		PreferRacks:      dto.PreferRacks,
		PartitionList:    dto.PartitionList,
	}
}

func DTOToModelSecretAgent(dto *dto.SecretAgent) *model.SecretAgent {
	if dto == nil {
		return nil
	}
	return &model.SecretAgent{
		ConnectionType: dto.ConnectionType,
		Address:        dto.Address,
		Port:           dto.Port,
		Timeout:        dto.Timeout,
		TLSCAString:    dto.TLSCAString,
		IsBase64:       dto.IsBase64,
	}
}

func DTOTodtoackupState(dto *dto.BackupState) *model.BackupState {
	if dto == nil {
		return nil
	}
	return &model.BackupState{
		LastFullRun: dto.LastFullRun,
		LastIncrRun: dto.LastIncrRun,
		Performed:   dto.Performed,
	}
}

func DTOTodtoackupDetails(dto *dto.BackupDetails) *model.BackupDetails {
	if dto == nil {
		return nil
	}
	return &model.BackupDetails{
		BackupMetadata: DTOTodtoackupMetadata(dto.BackupMetadata),
		Key:            dto.Key,
	}
}

func DTOTodtoackupMetadata(dto dto.BackupMetadata) model.BackupMetadata {
	return model.BackupMetadata{
		Created:             dto.Created,
		From:                dto.From,
		Namespace:           dto.Namespace,
		RecordCount:         dto.RecordCount,
		ByteCount:           dto.ByteCount,
		FileCount:           dto.FileCount,
		SecondaryIndexCount: dto.SecondaryIndexCount,
		UDFCount:            dto.UDFCount,
	}
}

func DTOToModelCurrentBackups(dto *dto.CurrentBackups) *model.CurrentBackups {
	if dto == nil {
		return nil
	}
	return &model.CurrentBackups{
		Full:        DTOToModelRunningJob(dto.Full),
		Incremental: DTOToModelRunningJob(dto.Incremental),
	}
}

func DTOToModelRunningJob(dto *dto.RunningJob) *model.RunningJob {
	if dto == nil {
		return nil
	}
	return &model.RunningJob{
		TotalRecords:     dto.TotalRecords,
		DoneRecords:      dto.DoneRecords,
		StartTime:        dto.StartTime,
		PercentageDone:   dto.PercentageDone,
		EstimatedEndTime: dto.EstimatedEndTime,
	}
}

func DTOToModelRestoreRequest(dto *dto.RestoreRequest) *model.RestoreRequest {
	if dto == nil {
		return nil
	}
	return &model.RestoreRequest{
		DestinationCuster: DTOToModelAerospikeCluster(dto.DestinationCuster),
		Policy:            DTOToModelRestorePolicy(dto.Policy),
		SourceStorage:     DTOToModelStorage(dto.SourceStorage),
		SecretAgent:       DTOToModelSecretAgent(dto.SecretAgent),
	}
}

func DTOToModelRestorePolicy(dto *dto.RestorePolicy) *model.RestorePolicy {
	if dto == nil {
		return nil
	}
	return &model.RestorePolicy{
		Parallel:           dto.Parallel,
		NoRecords:          dto.NoRecords,
		NoIndexes:          dto.NoIndexes,
		NoUdfs:             dto.NoUdfs,
		Timeout:            dto.Timeout,
		DisableBatchWrites: dto.DisableBatchWrites,
		MaxAsyncBatches:    dto.MaxAsyncBatches,
		BatchSize:          dto.BatchSize,
		Namespace:          DTOToModelRestoreNamespace(dto.Namespace),
		SetList:            dto.SetList,
		BinList:            dto.BinList,
		Replace:            dto.Replace,
		Unique:             dto.Unique,
		NoGeneration:       dto.NoGeneration,
		Bandwidth:          dto.Bandwidth,
		Tps:                dto.Tps,
		EncryptionPolicy:   DTOToModelEncryptionPolicy(dto.EncryptionPolicy),
		CompressionPolicy:  DTOToModelCompressionPolicy(dto.CompressionPolicy),
		RetryPolicy:        DTOToModelRetryPolicy(dto.RetryPolicy),
	}
}

func DTOToModelRestoreNamespace(dto *dto.RestoreNamespace) *model.RestoreNamespace {
	if dto == nil {
		return nil
	}
	return &model.RestoreNamespace{
		Source:      dto.Source,
		Destination: dto.Destination,
	}
}

func DTOToModelRetryPolicy(dto *dto.RetryPolicy) *model.RetryPolicy {
	if dto == nil {
		return nil
	}
	return &model.RetryPolicy{
		BaseTimeout: dto.BaseTimeout,
		Multiplier:  dto.Multiplier,
		MaxRetries:  dto.MaxRetries,
	}
}

func DTOToModelRestoreTimestampRequest(dto *dto.RestoreTimestampRequest) *model.RestoreTimestampRequest {
	if dto == nil {
		return nil
	}
	return &model.RestoreTimestampRequest{
		DestinationCuster: DTOToModelAerospikeCluster(dto.DestinationCuster),
		Policy:            DTOToModelRestorePolicy(dto.Policy),
		SecretAgent:       DTOToModelSecretAgent(dto.SecretAgent),
		Time:              dto.Time,
		Routine:           dto.Routine,
	}
}
