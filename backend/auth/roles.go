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

// AccountTypeChangeAllowed reports whether a user's role may change from one
// role to another. The account type is fixed at signup: a consumer (person)
// can't become a publisher (company/platform) or vice versa; they must create
// a new account. Granting or removing admin is still allowed.
func AccountTypeChangeAllowed(from, to string) bool {
	switch {
	case from == RoleConsumer && to == RolePublisher, from == RolePublisher && to == RoleConsumer:
		return false
	}
	return true
}

// IsAdmin reports whether the claims belong to an admin.
func (c *Claims) IsAdmin() bool {
	return c != nil && c.Role == RoleAdmin
}

// CanPublish reports whether the claims may publish (create) events: admins
// and publishers.
//
// This is THE single place that decides who may publish. When publishing
// becomes a paid feature, add the publisher subscription check here (e.g. an
// active-subscription claim or lookup) and every publish gate follows.
func CanPublish(c *Claims) bool {
	if c == nil {
		return false
	}
	switch c.Role {
	case RoleAdmin, RolePublisher:
		return true
	}
	return false
}
