package model

// User is an authenticated user of glabs-web. Identity comes from the auth proxy
// (the OIDC email); there is no allowlist — anyone the proxy authenticates is let
// in.
//
// glabs has no role hierarchy — every user manages only their own courses (strict
// per-user isolation), so there is nothing for roles to gate. The type is written
// by hand rather than generated so it carries bson tags (it was once persisted);
// gqlgen binds to it via autobind.
type User struct {
	Email string `json:"email" bson:"email"`
	Name  string `json:"name" bson:"name"`
}
