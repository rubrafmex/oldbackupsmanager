package api

const (
	// endpoints

	// trigger backup command in the configured CRDB
	endpointCRDBBackup = "/crdbBackup/"

	// endpoint used by CRDB to store the backups
	endpointBackups = "/backups/"

	// list the contents of the backups directory
	endpointListBackups = "/listBackups"

	// get backup from bucket
	endpointFromBucket = "/fromBucket/"
)

var Paths paths

type paths struct{}

func (paths) Backups() string {
	return endpointBackups
}
