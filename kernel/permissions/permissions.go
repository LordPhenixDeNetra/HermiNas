// Package permissions defines HermiNas' four RBAC roles (cahier des
// charges §7.1). Full RBAC middleware and per-token quotas land in M1.5 and
// M7.1; this L0 module only fixes the role vocabulary and hierarchy so
// every later layer references the same four names.
package permissions

type Role string

const (
	RoleViewer   Role = "viewer"
	RoleAnalyst  Role = "analyst"
	RoleEngineer Role = "engineer"
	RoleAdmin    Role = "admin"
)

var rank = map[Role]int{
	RoleViewer:   0,
	RoleAnalyst:  1,
	RoleEngineer: 2,
	RoleAdmin:    3,
}

// Rank returns the role's position in the hierarchy (higher = more
// privileged). Unknown roles rank 0, same as viewer.
func Rank(r Role) int { return rank[r] }

// AtLeast reports whether r is at least as privileged as min.
func AtLeast(r, min Role) bool { return Rank(r) >= Rank(min) }

func All() []Role { return []Role{RoleViewer, RoleAnalyst, RoleEngineer, RoleAdmin} }

func Valid(r Role) bool {
	_, ok := rank[r]
	return ok
}
