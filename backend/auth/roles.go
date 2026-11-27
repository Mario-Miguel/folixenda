package auth

// Roles issued by Better Auth in the JWT "role" claim.
const (
	RoleAdmin     = "admin"
	RoleConsumer  = "consumer"
	RolePublisher = "publisher"
)

// ValidRole reports whether r is one of the known roles.
func ValidRole(r string) bool {
	switch r {
	case RoleAdmin, RoleConsumer, RolePublisher:
		return true
	}
	return false
}

// IsAdmin reports whether the claims belong to an admin.
func (c *Claims) IsAdmin() bool {
	return c != nil && c.Role == RoleAdmin
}
