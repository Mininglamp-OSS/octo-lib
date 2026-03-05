package module

import (
	"fmt"
	"io"
	"path"
	"sort"
	"strings"

	"github.com/Mininglamp-OSS/octo-lib/config"
	"github.com/Mininglamp-OSS/octo-lib/pkg/register"
	"github.com/gocraft/dbr/v2"
	migrate "github.com/rubenv/sql-migrate"
)

func Setup(ctx *config.Context) error {

	// 获取所有模块
	ms := register.GetModules(ctx)

	// 初始化SQL
	var sqlfss []*register.SQLFS
	for _, m := range ms {
		if m.SQLDir != nil {
			sqlfss = append(sqlfss, m.SQLDir)
		}

	}
	err := executeSQL(sqlfss, ctx.DB())
	if err != nil {
		return err
	}
	// 注册api
	for _, m := range ms {
		if m.SetupAPI != nil {
			a := m.SetupAPI()
			if a != nil {
				a.Route(ctx.GetHttpRoute())
			}
		}
		if ctx.SetupTask {
			if m.SetupTask != nil {
				t := m.SetupTask()
				if t != nil {
					t.RegisterTasks()
				}
			}
		}
	}
	return nil

}

func Start(ctx *config.Context) error {
	// 获取所有模块
	ms := register.GetModules(ctx)
	for _, m := range ms {
		if m.Start != nil {
			err := m.Start()
			if err != nil {
				return err
			}
		}

	}
	return nil
}
func Stop(ctx *config.Context) error {
	// 获取所有模块
	ms := register.GetModules(ctx)
	for _, m := range ms {
		if m.Stop != nil {
			err := m.Stop()
			if err != nil {
				return err
			}
		}

	}
	return nil
}

// 执行sql
func executeSQL(sqlfss []*register.SQLFS, session *dbr.Session) error {
	migrations := &FileDirMigrationSource{
		sqlfss: sqlfss,
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
	sqlfss []*register.SQLFS
}

// FindMigrations FindMigrations
func (f FileDirMigrationSource) FindMigrations() ([]*migrate.Migration, error) {

	if len(f.sqlfss) == 0 {
		return nil, nil
	}
	migrations := make([]*migrate.Migration, 0, 100)

	for _, sqlfs := range f.sqlfss {
		err := f.findMigrations(sqlfs, &migrations)
		if err != nil {
			return nil, err
		}
	}

	// Make sure migrations are sorted
	sort.Sort(byID(migrations))

	return migrations, nil
}

func (f FileDirMigrationSource) findMigrations(fs *register.SQLFS, migrations *[]*migrate.Migration) error {

	files, err := fs.ReadDir("sql")
	if err != nil {
		return err
	}
	for _, info := range files {

		if strings.HasSuffix(info.Name(), ".sql") {
			file, err := fs.Open(path.Join("sql", info.Name()))
			if err != nil {
				return fmt.Errorf("error while opening %s: %s", info.Name(), err)
			}

			migration, err := migrate.ParseMigration(info.Name(), file.(io.ReadSeeker))
			if err != nil {
				return fmt.Errorf("error while parsing %s: %s", info.Name(), err)
			}
			*migrations = append(*migrations, migration)

		}
	}

	return nil
}
