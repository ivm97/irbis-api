package domain

// AccessLevel for a database.
type AccessLevel string

const (
	AccessRead  AccessLevel = "read"
	AccessWrite AccessLevel = "write"
)

// User is an API principal (local AUTH_USERS or IRBIS client_m + db_*.mnu).
type User struct {
	Username  string
	Password  string // plaintext (only when allow_plaintext / local store)
	PassHash  string // bcrypt hash
	// Unrestricted: local AUTH_USERS file with empty/* ACL — any DB, any level; ARM ignored.
	Unrestricted bool
	Databases    map[string][]AccessLevel
}

// DBAccess is the ACL resolved for a user at a given ARM.
type DBAccess struct {
	Unrestricted bool
	Databases    map[string][]AccessLevel
}

func (u User) Can(database string, need AccessLevel) bool {
	if u.Unrestricted {
		return true
	}
	levels, ok := u.Databases[database]
	if !ok {
		// case-insensitive DB name match
		for name, lv := range u.Databases {
			if equalFoldASCII(name, database) {
				levels = lv
				ok = true
				break
			}
		}
	}
	if !ok {
		return false
	}
	for _, l := range levels {
		if l == need {
			return true
		}
		// write implies read
		if need == AccessRead && l == AccessWrite {
			return true
		}
	}
	return false
}

func equalFoldASCII(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
