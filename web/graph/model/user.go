package model

// User is an authenticated user of glabs-web. Identity comes from the auth proxy
// (the OIDC email); a user must be on the allowlist to be let in.
//
// glabs has no role hierarchy — every user manages only their own courses (strict
// per-user isolation), so there is nothing for roles to gate. The type is written
// by hand rather than generated so it carries bson tags for MongoDB; gqlgen binds
// to it via autobind.
type User struct {
	Email string `json:"email" bson:"email"`
	Name  string `json:"name" bson:"name"`
}
