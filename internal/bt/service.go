package bt

// BTService is the orchestration layer that coordinates across all components
// to perform high-level backup operations needed by the CLI.
type BTService struct {
	database    Database
	stagingArea StagingArea
	vaults      []Vault
}

// NewBTService creates a new BTService with the provided dependencies.
func NewBTService(database Database, stagingArea StagingArea, vaults []Vault) *BTService {
	return &BTService{
		database:    database,
		stagingArea: stagingArea,
		vaults:      vaults,
	}
}
