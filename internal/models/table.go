package models

type Table struct {
	Name        string       `json:"name"`
	Columns     []Column     `json:"columns"`
	Indexes     []string     `json:"indexes,omitempty"`
	ForeignKeys []ForeignKey `json:"foreign_keys,omitempty"` // Add this
}

type ForeignKey struct {
	ID       int    `json:"id"`        // Foreign key ID
	Seq      int    `json:"seq"`       // Sequence number for multi-column FKs
	Table    string `json:"table"`     // Referenced table
	From     string `json:"from"`      // Column in current table
	To       string `json:"to"`        // Column in referenced table
	OnUpdate string `json:"on_update"` // ON UPDATE action
	OnDelete string `json:"on_delete"` // ON DELETE action
	Match    string `json:"match"`     // Match type (usually NONE)
}

type Column struct {
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	NotNull      bool    `json:"not_null"`
	DefaultValue *string `json:"default_value,omitempty"`
	PrimaryKey   bool    `json:"primary_key"`
}

type QueryResult struct {
	Columns []string         `json:"columns"`
	Rows    []map[string]any `json:"rows"`
	Count   int              `json:"count"`
}

type ExecuteResult struct {
	RowsAffected int64  `json:"rows_affected"`
	LastInsertId int64  `json:"last_insert_id,omitempty"`
	Message      string `json:"message"`
}
