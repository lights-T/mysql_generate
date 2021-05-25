# mysql_generate

mysql_generate包是根据数据库生成基础model和数据库脚本的工具


## 如何使用
### 安装生成可执行文件

* `cd ~/lib/database/mysql_generate`
* `go install`


### 使用工具
#### 第一种方式
* `cd ~/model` &&
  `mysql_generate -a address -d user:password@database -t table`

#### 第二种方式
* 在model.go 的init()上注释
  ` // go:generate  mysql_generate -a address -d database -t table`
* `cd ~/model`
* `go generate`


### 生成规则
在model目录下，生成数据库表名对应.go文件，里面包含对数据库的基本Get，Search，Create，Update方法，同时在doc下，生成
对应表的DDL，如果有变动会生成增量语句。

> 模型文件包含方法如下：

- CreateXX() 创建数据
- UpdateXX() 更新数据
- GetXX() 获取单条数据，指定的列字段将不会被返回
- GetXXWithFields() 获取单条数据，并返回指定的列字段
- SearchXX() 获取列表数据，指定的列字段将不会被返回，最大返回1000条
- SearchXXWithFields() 获取列表数据，并返回指定的列字段，最大返回1000条
- SearchXXWithFieldsLimit() 获取列表数据，并返回指定的列字段，可指定offset,limit，若limit大于1000，则返回1000



### 备注
* 修改字段名，生的语句需要注意，工具自动生成会有两条先删后加脚本
* 需要在运用项目的model目录下运行服务
* `-t`参数若不输入，则默认生成全表


