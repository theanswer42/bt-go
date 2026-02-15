package bt

// StagingArea provides an interface for staging files before backup.
// Files are staged in a queue and processed during backup operations.
// The staging area enforces a maximum size to prevent filling up the filesystem.
//
// Methods will be added as we implement BtService operations.
type StagingArea interface {
	// TODO: Methods to be defined during BtService implementation
}
