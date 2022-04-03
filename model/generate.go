package mysql

import (
	"bytes"
	"fmt"
	"go/format"
	"io"
	"os"
	"strings"
	"time"

	"github.com/golang/protobuf/protoc-gen-go/generator"
)

type Generate struct {
	dbInfo       *DBInfo
	tableInfo    *TableInfo
	buf          *bytes.Buffer
	imports      []string
	constants    map[string]interface{} // key==>value
	vars         map[string]interface{} // key==>value
	columnFields []string
	tableName    string
	structName   string
	afterFormat  []byte
}

func NewGenerate(dbInfo *DBInfo, tableInfo *TableInfo) *Generate {
	return &Generate{
		dbInfo:       dbInfo,
		tableInfo:    tableInfo,
		buf:          new(bytes.Buffer),
		imports:      make([]string, 0, 10),
		constants:    make(map[string]interface{}),
		vars:         make(map[string]interface{}),
		columnFields: make([]string, 0, 10),
	}
}

func (g *Generate) Parse() *Generate {
	if len(g.tableInfo.Fields) == 0 {
		return g
	}

	name := g.dbInfo.selectTableName
	if strings.Contains(name, "t_") {
		name = strings.TrimPrefix(name, "t_")
	}
	g.tableName = "TableName"
	g.structName = name

	g.generateStruct()

	g.generateGetOne()

	g.generateGetOneWithFields()

	g.generateSearch()

	g.generateSearchWithFields()

	g.generateSearchWithFieldsLimit()

	g.generateCount()

	g.generateCreate()

	g.generateUpdate()

	g.generateImports()
	g.generateConstant()
	g.generateVars()

	g.format()

	return g
}

func (g *Generate) generateCount() {
	fd := `
	func Count%s(ctx context.Context, exps interface{}) (int64, error) {
	var count int64
	var err error
   conditions := exp.NewExpressionList(exp.AndType)
	switch exps.(type) {
	case map[string]interface{}:
		for k, v := range exps.(map[string]interface{}) {
			conditions = conditions.Append(goqu.I(k).Eq(v))
		}
	case exp.ExpressionList:
		conditions = exps.(exp.ExpressionList)
	}
	if count, err := db.GetInstance("read").From(%s).
		Prepared(true).
		Where(conditions).CountContext(ctx); err != nil {
		return nil, err
	}
	if err != nil {
		return count, err
	}

	return count, nil
}
	`
	fd = fmt.Sprintf(fd, generator.CamelCase(g.structName), generator.CamelCase(g.tableName))
	g.buf.WriteString(fd)
}

func (g *Generate) getLowerName() string {
	n := g.structName
	n = strings.ToLower(n[0:1]) + n[1:]
	n = strings.ReplaceAll(n, "_", "")
	return n
}

func (g *Generate) getPackageName() []byte {
	n := g.getLowerName()
	return []byte("package " + n + "\n\n")
}

func (g *Generate) generateStruct() {
	s := fmt.Sprintf("type %s struct {\n  \n", generator.CamelCase(g.structName))
	g.buf.WriteString(s)
	timeIsFound := false
	for _, f := range g.tableInfo.Fields {
		filedName := generator.CamelCase(f.Field)
		jsonName := strings.ToLower(filedName[0:1]) + filedName[1:]
		goqu := g.tableInfo.ConvertGoQu(f)
		g.columnFields = append(g.columnFields, f.Field)
		filedType := g.tableInfo.ConvertType(f)
		if filedType == "time.Time" && !timeIsFound {
			g.imports = append(g.imports, "time")
			timeIsFound = true
		}
		s := fmt.Sprintf("%s\t%s\t `db:\"%s\" json:\"%s,omitempty\" goqu:\"%s\"`  // %s\n",
			filedName, filedType, f.Field, jsonName, goqu, strings.Trim(f.Comment, " "))

		g.buf.WriteString(s)
	}
	g.buf.WriteString("}")
}

func (g *Generate) generateCreate() {
	fd := `
	func Create%s(ctx context.Context,%s *%s,excludeFields ...string) (int64,error){
	builder := db.GetInstance("").Insert(%s)
	cols := make([]interface{}, 0, 10)
	if len(excludeFields) > 0 {
		_tempMap := make(map[string]struct{})
		for _, e := range excludeFields {
			_tempMap[e] = struct{}{}
		}
		for _, s := range ColumnFields {
			_s := s.(string)
			if _, ok := _tempMap[_s]; !ok {
				cols = append(cols, s)
			}
		}
	} else {
		cols = ColumnFields
	}
   b, err := builder.Prepared(true).
		Cols(cols...).
		Rows(%s).
		Executor().ExecContext(ctx)
	if err != nil {
		return 0, err
	}

	return b.LastInsertId()
}
`

	firsS := strings.ToLower(generator.CamelCase(g.structName)[0:1])
	fd = fmt.Sprintf(fd, generator.CamelCase(g.structName), firsS, generator.CamelCase(g.structName), generator.CamelCase(g.tableName), firsS)
	g.buf.WriteString(fd)
}

func (g *Generate) generateUpdate() {
	fd := `
      func Update%s(ctx context.Context,data map[string]interface{},exps interface{}) (int64, error){
	builder := db.GetInstance("").Update(%s)
    rc := make(goqu.Record)
	for k,v := range data{
		rc[k]=v
	}
    conditions := exp.NewExpressionList(exp.AndType)
	switch exps.(type) {
	case map[string]interface{}:
		for k, v := range exps.(map[string]interface{}) {
			conditions = conditions.Append(goqu.I(k).Eq(v))
		}
	case exp.ExpressionList:
		conditions = exps.(exp.ExpressionList)
	}
    u,err := builder.Set(rc).Where(conditions).Executor().ExecContext(ctx)
	if err !=nil {
		return 0,err
}

 return u.RowsAffected()
}
`
	fd = fmt.Sprintf(fd, generator.CamelCase(g.structName), generator.CamelCase(g.tableName))
	g.buf.WriteString(fd)
}

func (g *Generate) generateGetOne() {
	fd := `
    // Get%s exps 支持 map[string]interface{} 或 goqu 表达式（eq: exp.NewExpressionList(exp.AndType).Append(goqu.C(k).Eq(v))）
	func Get%s(ctx context.Context, exps interface{}, excludeFields ...string) (*%s, error) {
	self := &%s{}
    cols := make([]interface{}, 0, len(ColumnFields))
	if len(excludeFields) > 0 {
		_tempMap := make(map[string]struct{})
		for _, e := range excludeFields {
			_tempMap[e] = struct{}{}
		}
		for _, s := range ColumnFields {
            _s := s.(string)
			if _, ok := _tempMap[_s]; !ok {
				cols = append(cols, s)
			}
		}
	} else {
		cols = ColumnFields
	}
    conditions := exp.NewExpressionList(exp.AndType)
	switch exps.(type) {
	case map[string]interface{}:
		for k, v := range exps.(map[string]interface{}) {
			conditions = conditions.Append(goqu.I(k).Eq(v))
		}
	case exp.ExpressionList:
		conditions = exps.(exp.ExpressionList)
	}
	if _, err := db.GetInstance("read").From(%s).
		Prepared(true).
		Select(cols...).
		Where(conditions).
		ScanStructContext(ctx, self); err != nil {
		return nil, err
	}

	return self, nil
}
	`
	fd = fmt.Sprintf(fd, generator.CamelCase(g.structName), generator.CamelCase(g.structName), generator.CamelCase(g.structName), generator.CamelCase(g.structName), generator.CamelCase(g.tableName))
	g.buf.WriteString(fd)
}

func (g *Generate) generateGetOneWithFields() {
	fd := `
	func Get%sWithFields(ctx context.Context, exps interface{}, includeFields ...string) (*%s, error) {
	self := &%s{}
    cols := make([]interface{}, 0, len(ColumnFields))
	if len(includeFields) > 0 {
		_tempMap := make(map[string]struct{})
		for _, e := range includeFields {
			_tempMap[e] = struct{}{}
		}
		for _, s := range ColumnFields {
            _s := s.(string)
			if _, ok := _tempMap[_s]; ok {
				cols = append(cols, s)
			}
		}

	} else {
		cols = ColumnFields
	}
   conditions := exp.NewExpressionList(exp.AndType)
	switch exps.(type) {
	case map[string]interface{}:
		for k, v := range exps.(map[string]interface{}) {
			conditions = conditions.Append(goqu.I(k).Eq(v))
		}
	case exp.ExpressionList:
		conditions = exps.(exp.ExpressionList)
	}
	if _, err := db.GetInstance("read").From(%s).
		Prepared(true).
		Select(cols...).
		Where(conditions).
		ScanStructContext(ctx, self); err != nil {
		return nil, err
	}

	return self, nil
}
	`
	fd = fmt.Sprintf(fd, generator.CamelCase(g.structName), generator.CamelCase(g.structName), generator.CamelCase(g.structName), generator.CamelCase(g.tableName))
	g.buf.WriteString(fd)
}

func (g *Generate) generateSearch() {
	fd := `
	func Search%s(ctx context.Context, exps interface{}, excludeFields ...string) ([]*%s, error) {
	var self  []*%s
    cols := make([]interface{}, 0, len(ColumnFields))
	if len(excludeFields) > 0 {
		_tempMap := make(map[string]struct{})
		for _, e := range excludeFields {
			_tempMap[e] = struct{}{}
		}
		for _, s := range ColumnFields {
			_s := s.(string)
			if _, ok := _tempMap[_s]; !ok {
				cols = append(cols, s)
			}
		}
	} else {
		cols = ColumnFields
	}
   conditions := exp.NewExpressionList(exp.AndType)
	switch exps.(type) {
	case map[string]interface{}:
		for k, v := range exps.(map[string]interface{}) {
			conditions = conditions.Append(goqu.I(k).Eq(v))
		}
	case exp.ExpressionList:
		conditions = exps.(exp.ExpressionList)
	}
	if  err := db.GetInstance("read").From(%s).
		Prepared(true).
		Select(cols...).
		Where(conditions).
        Limit(MaxLimit).
		ScanStructsContext(ctx, &self); err != nil {
		return nil, err
	}
	if len(self) == 0 {
		return nil, nil
	}

	return self, nil
}
	`
	fd = fmt.Sprintf(fd, generator.CamelCase(g.structName), generator.CamelCase(g.structName), generator.CamelCase(g.structName), generator.CamelCase(g.tableName))
	g.buf.WriteString(fd)
}

func (g *Generate) generateSearchWithFields() {
	fd := `
	func Search%sWithFields(ctx context.Context, exps interface{}, includeFields ...string) ([]*%s, error) {
	var self  []*%s
    cols := make([]interface{}, 0, len(ColumnFields))
	if len(includeFields) > 0 {
		_tempMap := make(map[string]struct{})
		for _, e := range includeFields {
			_tempMap[e] = struct{}{}
		}
		for _, s := range ColumnFields {
			_s := s.(string)
			if _, ok := _tempMap[_s]; ok {
				cols = append(cols, s)
			}
		}
	} else {
		cols = ColumnFields
	}
    conditions := exp.NewExpressionList(exp.AndType)
	switch exps.(type) {
	case map[string]interface{}:
		for k, v := range exps.(map[string]interface{}) {
			conditions = conditions.Append(goqu.I(k).Eq(v))
		}
	case exp.ExpressionList:
		conditions = exps.(exp.ExpressionList)
	}
	if  err := db.GetInstance("read").From(%s).
		Prepared(true).
		Select(cols...).
		Where(conditions).
         Limit(MaxLimit).
		ScanStructsContext(ctx, &self); err != nil {
		return nil, err
	}
	if len(self) == 0 {
		return nil, nil
	}

	return self, nil
}
	`
	fd = fmt.Sprintf(fd, generator.CamelCase(g.structName), generator.CamelCase(g.structName), generator.CamelCase(g.structName), generator.CamelCase(g.tableName))
	g.buf.WriteString(fd)
}

func (g *Generate) generateSearchWithFieldsLimit() {
	fd := `
	func Search%sWithFieldsLimit(ctx context.Context, exps interface{},offset,limit uint, includeFields ...string) ([]*%s, error) {
	var self  []*%s
    cols := make([]interface{}, 0, len(ColumnFields))
    if limit > MaxLimit {
        limit = MaxLimit
     }
	if len(includeFields) > 0 {
		_tempMap := make(map[string]struct{})
		for _, e := range includeFields {
			_tempMap[e] = struct{}{}
		}
		for _, s := range ColumnFields {
			_s := s.(string)
			if _, ok := _tempMap[_s]; ok {
				cols = append(cols, s)
			}
		}
	} else {
		cols = ColumnFields
	}
   conditions := exp.NewExpressionList(exp.AndType)
	switch exps.(type) {
	case map[string]interface{}:
		for k, v := range exps.(map[string]interface{}) {
			conditions = conditions.Append(goqu.I(k).Eq(v))
		}
	case exp.ExpressionList:
		conditions = exps.(exp.ExpressionList)
	}
	if  err := db.GetInstance("read").From(%s).
		Prepared(true).
		Select(cols...).
		Where(conditions).
        Offset(offset).
        Limit(limit).
		ScanStructsContext(ctx, &self); err != nil {
		return nil, err
	}
	if len(self) == 0 {
		return nil, nil
	}

	return self, nil
}
	`
	fd = fmt.Sprintf(fd, generator.CamelCase(g.structName), generator.CamelCase(g.structName), generator.CamelCase(g.structName), generator.CamelCase(g.tableName))
	g.buf.WriteString(fd)
}

func (g *Generate) generateImports() {
	imports := []string{
		"context",
		fmt.Sprint(""),
		"github.com/doug-martin/goqu/v9",
		"github.com/doug-martin/goqu/v9/exp",
		fmt.Sprint(""),
		fmt.Sprintf("db %s", Package),
	}
	g.imports = append(g.imports, imports...)
}

func (g *Generate) generateConstant() {

	// 定义表名
	g.constants[generator.CamelCase(g.tableName)] = fmt.Sprintf("\"%s\"", g.dbInfo.selectTableName)

	// limit限制
	g.constants["MaxLimit"] = 1000

}

func (g *Generate) generateVars() {
	// 定义所有的字段
	fstr := ""
	for _, f := range g.columnFields {
		fstr += fmt.Sprintf("\"%s\",", f)
	}
	fstr = strings.Trim(fstr, ",")
	g.vars["ColumnFields"] = fmt.Sprintf("[]interface{}{%s}", fstr)
}

func (g *Generate) format() {
	var b bytes.Buffer

	// package
	b.Write(g.getPackageName())

	// import
	if len(g.imports) > 0 {
		b.WriteString("import (\n")
		for _, s := range g.imports {
			if len(s) == 0 {
				b.WriteString("\n")
				continue
			}
			if arr := strings.Split(s, " "); len(arr) > 1 {
				b.WriteString(fmt.Sprintf("%s \"%s\"", arr[0], arr[1]))
			} else {
				b.WriteString("\"" + s + "\"\n")
			}
		}
		b.WriteString(")\n")
	}
	// constant
	if len(g.constants) > 0 {
		b.WriteString("const (\n")
		for k, v := range g.constants {
			b.WriteString(fmt.Sprintf("%s=%v\n", k, v))
		}
		b.WriteString(")\n")
	}

	// variables
	if len(g.vars) > 0 {
		b.WriteString("var (\n")
		for k, v := range g.vars {
			b.WriteString(fmt.Sprintf("%s=%v\n", k, v))
		}
		b.WriteString(")\n")
	}

	// content
	b.Write(g.buf.Bytes())
	sb := b.Bytes()
	by, err := format.Source(sb)
	if err != nil {
		fmt.Println("format err:", err)
		return
	}

	g.afterFormat = by
}

func (g *Generate) Write() error {
	_dir, err := os.Getwd()
	if err != nil {
		return err
	}
	if arr := strings.Split(_dir, "/"); arr[len(arr)-1] != "model" {

		return fmt.Errorf("Current path [%s], Must come into the model dir execute ", _dir)
	}

	pfn := g.getLowerName()
	dir := _dir + "/" + pfn
	if _, _err := os.Stat(dir); _err != nil {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	file := dir + "/" + pfn + ".go"
	var f *os.File
	if _, err := os.Stat(file); !os.IsExist(err) {
		f, err = os.Create(file)
		if err != nil {
			return err
		}
	} else {
		f, err = os.OpenFile(file, os.O_RDWR|os.O_CREATE, 0755)
		if err != nil {
			return err
		}
	}
	defer f.Close()
	_, err = f.Write(g.afterFormat)
	fmt.Println("Create [" + file + "] Success")
	return err
}

func (g *Generate) String() string {
	return string(g.afterFormat)
}

func (g *Generate) WriteDDL() error {

	_dir, err := os.Getwd()
	if err != nil {
		return err
	}
	if arr := strings.Split(_dir, "/"); arr[len(arr)-1] != "model" {

		return fmt.Errorf("Current path [%s], Must come into the model dir execute ", _dir)
	}

	arr := strings.Split(_dir, "/")
	createDirs := append(arr[:len(arr)-1], "doc")
	createDir := strings.Join(createDirs, "/")
	dir := createDir + "/" + strings.ToLower(g.structName[0:1]) + g.structName[1:]

	var readDDL string
	if _, _err := os.Stat(dir); _err != nil {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	file := dir + "/" + strings.ToLower(g.structName[0:1]) + g.structName[1:] + ".sql"
	var f *os.File
	if _, err := os.Stat(file); os.IsNotExist(err) {
		f, err = os.Create(file)
		if err != nil {
			return err
		}
	} else {
		f, err = os.OpenFile(file, os.O_RDWR|os.O_CREATE, 0755)
		if err != nil {
			return err
		}
		var content []byte
		temp := make([]byte, 1024)
		for {
			n, err := f.Read(temp)
			if err == io.EOF {
				break
			}
			content = append(content, temp[:n]...)
		}
		if strings.Contains(string(content), "Create@") {
			arr := strings.Split(string(content), "Create@")
			readDDL = arr[len(arr)-1]
		}
	}
	defer f.Close()

	var b bytes.Buffer
	host, _ := os.Hostname()
	b.WriteString(fmt.Sprintf("# Create@%s,By: %s\n", time.Now().Format("2006-01-02 15:04:05"), host))

	// content
	b.Write([]byte(fmt.Sprintf("create table if not exists %s\n", g.tableInfo.TableName)))
	b.Write([]byte(g.dbInfo.selectTableDDL[strings.Index(g.dbInfo.selectTableDDL, "("):]))
	b.WriteString(";\n")
	b.WriteString("# =====================================================================================\n")
	sb := b.Bytes()

	if len(readDDL) == 0 {
		_, err = f.Write(sb)
		fmt.Println("Create [" + file + "] Success")
		return err
	}

	arr2 := strings.Split(string(sb), "Create@")
	readFieldStr := readDDL[strings.Index(readDDL, "("):strings.LastIndex(readDDL, "ENGINE")]
	writeFieldStr := arr2[1][strings.Index(arr2[1], "("):strings.LastIndex(arr2[1], "ENGINE")]
	if readFieldStr != writeFieldStr {
		readDDLFields := strings.Split(readFieldStr, "\n")
		writeDDLFields := strings.Split(writeFieldStr, "\n")
		var addFields []string
		var removeFields []string
		var modifyFields []string
		for _, w := range writeDDLFields {
			if !strings.Contains(readFieldStr, w) {
				if strings.Contains(readFieldStr, w[strings.Index(w, "`"):strings.LastIndex(w, "`")]) {
					modifyFields = append(modifyFields, w)
					continue
				}
				addFields = append(addFields, w)
			}
		}

		for _, r := range readDDLFields {
			if !strings.Contains(writeFieldStr, r) {
				if strings.Contains(writeFieldStr, r[strings.Index(r, "`"):strings.LastIndex(r, "`")]) {
					continue
				}
				removeFields = append(removeFields, r)
			}
		}
		var updateByte bytes.Buffer
		for _, a := range addFields {
			a = strings.TrimPrefix(a, " ")
			if strings.Contains(a, "UNIQUE KEY") {
				updateByte.WriteString(fmt.Sprintf("CREATE UNIQUE INDEX `%s` ON %s %s;\n", strings.Split(a, "`")[1],
					g.tableInfo.TableName, a[strings.Index(a, "("):strings.LastIndex(a, ")")+1]))
				continue
			} else if strings.Contains(a, "PRIMARY KEY") {
				updateByte.WriteString(fmt.Sprintf("ALTER  TABLE %s ADD  PRIMARY  KEY %s;\n",
					g.tableInfo.TableName, a[strings.Index(a, "("):strings.LastIndex(a, ")")+1]))
				continue
			} else if strings.Contains(a, " KEY") {
				updateByte.WriteString(fmt.Sprintf("CREATE  INDEX `%s` ON %s %s;\n", strings.Split(a, "`")[1],
					g.tableInfo.TableName, a[strings.Index(a, "("):strings.LastIndex(a, ")")+1]))
				continue
			} else {
				updateByte.WriteString(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s;\n", g.tableInfo.TableName,
					strings.Replace(a, ",", "", -1)))
			}

		}
		for _, r := range removeFields {
			r = strings.TrimPrefix(r, " ")
			if strings.Contains(r, " KEY") {
				updateByte.WriteString(fmt.Sprintf("DROP  INDEX %s ON %s;\n", strings.Split(r, "`")[1],
					g.tableInfo.TableName))
				continue
			} else {
				updateByte.WriteString(fmt.Sprintf("ALTER TABLE %s DROP COLUMN `%s`;\n", g.tableInfo.TableName,
					strings.Split(r, "`")[1]))
			}
		}
		for _, m := range modifyFields {
			m = strings.TrimPrefix(m, " ")
			if !strings.Contains(m, "KEY") {
				updateByte.WriteString(fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s;\n", g.tableInfo.TableName,
					strings.Replace(m, ",", "", -1)))
			}

		}
		var title bytes.Buffer
		host, _ := os.Hostname()
		title.WriteString("\n")
		title.WriteString(fmt.Sprintf("# Create@%s,By: %s\n", time.Now().Format("2006-01-02 15:04:05"), host))
		_, err = f.Write(title.Bytes())
		_, err = f.Write([]byte(fmt.Sprintf("create table if not exists %s\n", g.tableInfo.TableName)))
		_, err = f.Write([]byte(g.dbInfo.selectTableDDL[strings.Index(g.dbInfo.selectTableDDL, "("):]))
		_, err = f.Write([]byte(";\n# change：\n"))
		updateByte.WriteString("# =====================================================================================\n")
		_, err = f.Write(updateByte.Bytes())
		fmt.Println("Update [" + file + "] Success")
		return err
	}

	return nil
}
