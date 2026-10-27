package repository

import "testing"

// Compile-time assertions: the GORM user repository backs both the auth
// contract and the admin management contract.
var (
	_ UserRepository      = (*userRepository)(nil)
	_ UserAdminRepository = (*userRepository)(nil)
)

func TestNewUserAdminRepositoryReturnsNonNil(t *testing.T) {
	if NewUserAdminRepository(nil) == nil {
		t.Error("NewUserAdminRepository(nil) = nil, want non-nil repository")
	}
}

func TestNewAdminStatsRepositoryReturnsNonNil(t *testing.T) {
	if NewAdminStatsRepository(nil, nil) == nil {
		t.Error("NewAdminStatsRepository(nil, nil) = nil, want non-nil repository")
	}
}
