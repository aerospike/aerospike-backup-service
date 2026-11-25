package service

import "context"

// ServerJobPoller follows the backup jobs that run on the Aerospike servers, whoever started them,
// and reports each routine's backup when its jobs end.
type ServerJobPoller interface {
	// Start polls in the background until ctx is canceled.
	Start(ctx context.Context)
}

// NoServerJobPoller returns the ServerJobPoller of scan mode, where the servers run no backup jobs.
func NoServerJobPoller() ServerJobPoller {
	return noServerJobPoller{}
}

type noServerJobPoller struct{}

func (noServerJobPoller) Start(context.Context) {}
