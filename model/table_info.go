package mysql

import (
	"fmt"
	"strings"
)

type TableInfo struct {
	Fields    []*FieldInfo
	TableName string
}

type FieldInfo struct {
	Field      string  `db:"Field"`
	Type       string  `db:"Type"`
	Comment    string  `db:"Comment"`
	Collation  *string `db:"Collation"`
	Null       string  `db:"Null"`
	Key        string  `db:"Key"`
	Default    *string `db:"Default"`
	Extra      *string `db:"Extra"`
	Privileges string  `db:"Privileges"`
}

type DDLInfo struct {
	Table       string `db:"Table"`
	CreateTable string `db:"Create Table"`
}

func NewTableInfo() *TableInfo {
	return &TableInfo{}
}

func (t *TableInfo) TableProfit(tableName string) *TableInfo {
	t.TableName = tableName
	sql := "show full columns from " + tableName
	var res []*FieldInfo
	if err := db.ScanStructs(&res, sql); err != nil {
		fmt.Println("get table info err:", err)
		return t
	}
	t.Fields = res
	return t
}

func (t *TableInfo) ConvertGoQu(f *FieldInfo) string {
	var goqu string
	switch f.Field {
	case "id":
		goqu = "pk,skipinsert,skipupdate"
	case "create_time", "update_time":
		goqu = "skipinsert,skipupdate"
	default:
		goqu = "defaultifempty"
	}

	filedType := t.ConvertType(f)
	if filedType == "time.Time" {
		goqu = "skipinsert,skipupdate"
	}

	return goqu
}

func (t *TableInfo) ConvertType(f *FieldInfo) string {
	typeArr := strings.Split(f.Type, "(")
	switch typeArr[0] {
	case "int":
		return "int32"
	case "integer":
		return "int32"
	case "mediumint":
		return "int32"
	case "bit":
		return "int32"
	case "year":
		return "int32"
	case "smallint":
		return "int8"
	case "tinyint":
		return "int8"
	case "bigint":
		return "int64"
	case "decimal":
		return "float32"
	case "double":
		return "float64"
	case "float":
		return "float32"
	case "real":
		return "float32"
	case "numeric":
		return "float32"
	case "timestamp":
		return "string"
	case "datetime":
		return "string"
	case "date":
		return "string"
	case "time":
		return "string"
	default:
		return "string"
	}
}
