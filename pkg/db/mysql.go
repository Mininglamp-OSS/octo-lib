package db

import (
	"database/sql"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/gocraft/dbr/v2"
	"github.com/gocraft/dbr/v2/dialect"
	migrate "github.com/rubenv/sql-migrate"
)

// NewMySQL 创建一个MySQL db，[path]db存储路径 [sqlDir]sql脚本目录
//
// 不再走 dbr.Open（内部 sql.Open + 默认 driver），而是手动用
// sql.OpenDB(instrumentedConnector{...}) 打开，并手搭 *dbr.Connection。这样：
//   - 建连握手经 instrumentedConnector.Connect 计时（op="connect"）；
//   - 每条查询经 metricEventReceiver 计时（op="query"）。
//
// 池参数（MaxOpenConns/MaxIdleConns/ConnMaxLifetime）与迁移行为保持不变。
// 注：ConnMaxLifetime 建议设为小于 MySQL 服务端 wait_timeout（默认 28800s），
// 以免使用到被服务端单方面关闭的陈旧连接。
func NewMySQL(addr string, maxOpenConns int, maxIdleConns int, connMaxLifetime time.Duration) *dbr.Session {

	base, err := mysql.MySQLDriver{}.OpenConnector(addr)
	if err != nil {
		panic(err)
	}
	sqlDB := sql.OpenDB(instrumentedConnector{Connector: base})
	sqlDB.SetMaxOpenConns(maxOpenConns)
	sqlDB.SetMaxIdleConns(maxIdleConns)
	sqlDB.SetConnMaxLifetime(connMaxLifetime) //mysql 默认超时时间为 60*60*8=28800 SetConnMaxLifetime设置为小于数据库超时时间即可

	conn := &dbr.Connection{
		DB:            sqlDB,
		Dialect:       dialect.MySQL,
		EventReceiver: metricEventReceiver{&dbr.NullEventReceiver{}},
	}

	session := conn.NewSession(nil)

	return session
}

func Migration(sqlDir string, session *dbr.Session) error {
	migrations := &FileDirMigrationSource{
		Dir: sqlDir,
	}
	_, err := migrate.Exec(session.DB, "mysql", migrations, migrate.Up)
	if err != nil {
		return err
	}
	return nil
}

type byID []*migrate.Migration

func (b byID) Len() int           { return len(b) }
func (b byID) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }
func (b byID) Less(i, j int) bool { return b[i].Less(b[j]) }

// FileDirMigrationSource 文件目录源 遇到目录进行递归获取
type FileDirMigrationSource struct {
	Dir string
}

// FindMigrations FindMigrations
func (f FileDirMigrationSource) FindMigrations() ([]*migrate.Migration, error) {
	filesystem := http.Dir(f.Dir)
	migrations := make([]*migrate.Migration, 0, 100)
	err := f.findMigrations(filesystem, &migrations)
	if err != nil {
		return nil, err
	}
	// Make sure migrations are sorted
	sort.Sort(byID(migrations))

	return migrations, nil
}

func (f FileDirMigrationSource) findMigrations(dir http.FileSystem, migrations *[]*migrate.Migration) error {

	file, err := dir.Open("/")
	if err != nil {
		return err
	}

	files, err := file.Readdir(0)
	if err != nil {
		return err
	}

	for _, info := range files {

		if strings.HasSuffix(info.Name(), ".sql") {
			file, err := dir.Open(info.Name())
			if err != nil {
				return fmt.Errorf("Error while opening %s: %s", info.Name(), err)
			}

			migration, err := migrate.ParseMigration(info.Name(), file)
			if err != nil {
				return fmt.Errorf("Error while parsing %s: %s", info.Name(), err)
			}
			*migrations = append(*migrations, migration)

		} else if info.IsDir() {
			err = f.findMigrations(http.Dir(fmt.Sprintf("%s/%s", f.Dir, info.Name())), migrations)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
