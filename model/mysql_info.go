package mysql

import (
	"fmt"
)

type DBInfo struct {
	ableDatabases      []string
	ableTables         []string
	selectDataBaseName string
	selectTableName    string
	selectTableDDL     string
	showAction         int
}

func NewInfo() *DBInfo {
	return &DBInfo{}
}

func (i *DBInfo) FetchOriginTables(database string) *DBInfo {
	if len(database) < 0 {
		return i
	}
	sql := fmt.Sprintf("SELECT TABLE_NAME FROM information_schema.TABLES WHERE TABLE_SCHEMA='%s'", database)
	res := i.scanColumn(sql)
	if len(res) > 0 {
		i.ableTables = res
	}
	return i
}

func (i *DBInfo) scanColumn(sql string) []string {
	res, err := db.Query(sql)
	strs := make([]string, 0, 10)
	if err != nil {
		fmt.Println("show database err:", err)
		return strs
	}
	for {
		if !res.Next() {
			break
		}
		var s string
		err := res.Scan(&s)

		if err != nil {
			fmt.Println("get columns err:", err)
			return strs
		}
		strs = append(strs, s)
	}
	return strs
}

func (i *DBInfo) FetchTableDDL(tableName string) *DBInfo {
	if len(tableName) == 0 {
		return i
	}

	sql := fmt.Sprintf("SHOW CREATE TABLE %s", tableName)
	var res []*DDLInfo
	if err := db.ScanStructs(&res, sql); err != nil {
		fmt.Println("get table info err:", err)
		return i
	}
	if len(res) != 0 {
		i.selectTableDDL = res[0].CreateTable
	}

	return i
}
