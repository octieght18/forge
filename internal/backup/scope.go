// Package backup describes what a PostgreSQL dump contains.
// The drill uses a disposable database. It does not read the retained product data.
package backup

// Scope is the backup boundary. Encrypted and Offsite stay false unless a drill actually produced that copy.
type Scope struct {
	Included      []string
	Unrecoverable []string
	Dependencies  []string
	Encrypted     bool
	Offsite       bool
}

// Record is the backup this repository can describe. The dump is a local SQL file, not an encrypted offsite copy.
func Record() Scope {
	return Scope{
		Included: []string{
			"PostgreSQL schema and rows in the dumped database, including workloads and principals",
		},
		Unrecoverable: []string{
			"Temporal workflow history",
			"in-process evidence",
			"in-process audit events",
		},
		Dependencies: []string{
			"pg_dump and psql",
			"a second empty PostgreSQL database",
			"the dump file from this drill",
		},
		Encrypted: false,
		Offsite:   false,
	}
}
