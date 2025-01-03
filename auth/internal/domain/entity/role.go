package entity

type Role string

const (
	RoleAdmin    Role = "admin"
	RoleOperator Role = "operator"
	RoleRegular  Role = "regular"
)

func (r Role) String() string {
	return string(r)
}

func (r Role) Valid() bool {
	return r == RoleAdmin || r == RoleRegular || r == RoleOperator
}
