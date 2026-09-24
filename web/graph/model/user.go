package model

// User is an authenticated user of glabs-web. Identity comes from the auth proxy
// (the OIDC email). Being authenticated is not being let in: only admins and
// users an admin approved get past the access gate (see web/app/access.go).
//
// glabs has no role hierarchy — every user manages only their own courses (strict
// per-user isolation), so there is nothing for roles to gate. The type is written
// by hand rather than generated so it carries bson tags (it was once persisted);
// gqlgen binds to it via autobind.
type User struct {
	Email string `json:"email" bson:"email"`
	Name  string `json:"name" bson:"name"`
	// Department is the faculty the proxy forwards, when it does. It is not part
	// of the GraphQL type; it travels along so an access request can show it.
	Department string `json:"-" bson:"-"`
}
