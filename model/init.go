package mysql

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"os/exec"
	"strings"

	"github.com/doug-martin/goqu/v9"
	_ "github.com/doug-martin/goqu/v9/dialect/mysql"
	_ "github.com/go-sql-driver/mysql"
)

var db *goqu.Database
var conInfo *Connection
var Package = ""

type Connection struct {
	A string `jsonpb:"u"`
	D string `jsonpb:"d"`
	T string `jsonpb:"t"`
}

func Init() {
	if Package = getPackage(); len(Package) == 0 {
		fmt.Println("The dir no one file, Must exist go file")
		return
	}

	s, err := sql.Open("mysql", fmt.Sprintf("%s/%s", conInfo.A, conInfo.D))
	if err != nil {
		fmt.Println("open mysql err", err)
		return
	}
	if err := s.Ping(); err != nil {
		fmt.Printf("connection Host [%s], happend error:%v", conInfo.A, err)
		return
	}
	db = goqu.New("mysql", s)

	dbInfo := &DBInfo{
		selectDataBaseName: conInfo.D,
		ableTables:         NewInfo().FetchOriginTables(conInfo.D).ableTables,
	}

	var generator = func(tableName string) {
		tableInfo := NewTableInfo().TableProfit(tableName)
		if len(tableInfo.Fields) == 0 {
			return
		}
		g := NewGenerate(dbInfo, tableInfo).Parse()
		if err := g.Write(); err != nil {
			fmt.Println("write to file err:", err)
		}
		dbInfo.FetchTableDDL(tableName)
		if err := g.WriteDDL(); err != nil {
			fmt.Println("write to file err:", err)
		}
	}
	if len(conInfo.T) == 0 {
		for _, t := range dbInfo.ableTables {
			_t := conInfo.D + "." + t
			dbInfo.selectTableName = t
			generator(_t)
		}

	} else {
		dbInfo.selectTableName = conInfo.T
		generator(conInfo.D + "." + conInfo.T)

	}
	gitInit()
	fmt.Println("Congratulation! Finish...")
}

func gitInit() {
	cmd := exec.Command("go", "get", "-insecure", "-v", "git.xxg.com/cenddev/go/v2/lib")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Println("go list err:", err)
		return
	}
	defer stdout.Close()
	if err := cmd.Start(); err != nil {
		fmt.Println("start cmd err:", err)
		return
	}
	b, _ := ioutil.ReadAll(stdout)
	fmt.Print(string(b))
}

func getPackage() string {
	cmd := exec.Command("go", "list")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Println("go list err:", err)
		return ""
	}
	defer stdout.Close()
	if err := cmd.Start(); err != nil {
		fmt.Println("start cmd err:", err)
		return ""
	}

	b, _ := ioutil.ReadAll(stdout)
	s := strings.Trim(string(b), "\n")
	if arr := strings.Split(s, "/"); arr[len(arr)-1] != "model" {
		fmt.Printf("Package:Current path [%s], Must come into the model dir execute \n", s)
		return ""
	}
	return s

}

func SaveConfig(a, d, t string) bool {
	c := &Connection{A: a, D: d, T: t}
	conInfo = c
	return true
}
