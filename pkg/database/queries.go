package database

import (
	"fmt"
	"strings"
)

type ColumnValues []struct {
	Column string
	Value  interface{}
}

func (np *ColumnValues) Add(column string, value interface{}) {
	*np = append(*np, struct {
		Column string
		Value  interface{}
	}{column, value})
}

func (np ColumnValues) Columns() string {
	columns := make([]string, len(np))
	for i, entry := range np {
		columns[i] = entry.Column
	}
	return strings.Join(columns, ",")
}

func (np ColumnValues) Placeholders() string {
	placeholders := make([]string, len(np))
	for i := range np {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}
	return strings.Join(placeholders, ",")
}

func (np ColumnValues) Args() []interface{} {
	args := make([]interface{}, len(np))
	for i, entry := range np {
		args[i] = entry.Value
	}
	return args
}
