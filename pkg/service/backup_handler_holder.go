package service

type BackupHandlerHolder struct {
	Handlers map[string]*BackupHandler
}
