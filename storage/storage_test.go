package storage

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/0xdevelop/project_template_go/ability/ability_task/ability_task_model"
	"github.com/0xdevelop/project_template_go/config"
	"github.com/0xdevelop/project_template_go/storage/storage_config"
	"github.com/george012/gtbox/gtbox_orm/gtbox_orm_sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type storageProbe struct {
	ID   uint `gorm:"primaryKey"`
	Name string
}

func TestStorageTaskPayloadColumnTypes(t *testing.T) {
	model, err := schema.Parse(&ability_task_model.AsyncTask{}, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatal(err)
	}
	for _, testCase := range []struct {
		dialect gorm.Dialector
		want    string
	}{
		{mysql.New(mysql.Config{}), "mediumtext"},
		{postgres.New(postgres.Config{}), "text"},
		{sqlite.Open(":memory:"), "text"},
	} {
		for _, name := range []string{"InputPayload", "ResultPayload"} {
			if got := testCase.dialect.DataTypeOf(model.LookUpField(name)); got != testCase.want {
				t.Errorf("%s %s: got %s, want %s", testCase.dialect.Name(), name, got, testCase.want)
			}
		}
	}
}

func TestStorageCloseWhileReading(t *testing.T) {
	previous := config.GlobalConfig
	t.Cleanup(func() {
		_ = CloseStorage()
		config.GlobalConfig = previous
	})
	config.GlobalConfig = &config.FileConfig{StorageCfg: &storage_config.StorageConfig{
		Primary: "sqlite",
		Sqlite: &storage_config.StorageSqliteConfig{
			Enabled:           true,
			GTORMSqliteConfig: gtbox_orm_sqlite.GTORMSqliteConfig{DBFilePath: filepath.Join(t.TempDir(), "close.db")},
		},
	}}
	if err := OpenStorage(); err != nil {
		t.Fatal(err)
	}
	original, err := GormDB(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := OpenStorage(); err == nil {
		t.Fatal("repeated open must reject replacing the active connection pool")
	}
	pool, err := original.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := pool.Ping(); err != nil {
		t.Fatalf("rejected reopen changed the original pool: %v", err)
	}

	var readers, ready sync.WaitGroup
	readers.Add(8)
	ready.Add(8)
	for range 8 {
		go func() {
			defer readers.Done()
			_, _ = GormDB(context.Background())
			ready.Done()
			for range 1000 {
				_, _ = GormDB(context.Background())
				_, _ = SqliteDB(context.Background())
				_, _ = MysqlDB(context.Background())
				_, _ = PostgreSQLDB(context.Background())
				_, _ = Redis()
				_, _ = RedisDatabase()
				runtime.Gosched()
			}
		}()
	}
	ready.Wait()
	if err := CloseStorage(); err != nil {
		t.Error(err)
	}
	readers.Wait()
	if handle, err := GormDB(context.Background()); err == nil || handle != nil {
		t.Fatal("closed storage returned a primary handle")
	}
	if err := OpenStorage(); err != nil {
		t.Fatalf("reopen after close: %v", err)
	}
}

func TestSqliteInstancesStaySeparate(t *testing.T) {
	dir := t.TempDir()
	previous := config.GlobalConfig
	t.Cleanup(func() {
		_ = CloseStorage()
		config.GlobalConfig = previous
	})
	config.GlobalConfig = &config.FileConfig{StorageCfg: &storage_config.StorageConfig{
		Sqlite: &storage_config.StorageSqliteConfig{
			Enabled:           true,
			GTORMSqliteConfig: gtbox_orm_sqlite.GTORMSqliteConfig{DBFilePath: filepath.Join(dir, "app.db")},
		},
	}}
	if err := OpenStorage(); err != nil {
		t.Fatal(err)
	}
	left, err := gtbox_orm_sqlite.New(gtbox_orm_sqlite.GTORMSqliteConfig{DBFilePath: filepath.Join(dir, "left.db")})
	if err != nil {
		t.Fatal(err)
	}
	right, err := gtbox_orm_sqlite.New(gtbox_orm_sqlite.GTORMSqliteConfig{DBFilePath: filepath.Join(dir, "right.db")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = left.Close()
		_ = right.Close()
	})
	if err := left.SqliteDB.AutoMigrate(&storageProbe{}); err != nil {
		t.Fatal(err)
	}
	if err := right.SqliteDB.AutoMigrate(&storageProbe{}); err != nil {
		t.Fatal(err)
	}
	if err := left.SqliteDB.Create(&storageProbe{Name: "left"}).Error; err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := right.SqliteDB.Model(&storageProbe{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("right instance saw %d rows from left", count)
	}
	if err := CloseStorage(); err != nil {
		t.Fatal(err)
	}
	for _, instance := range []*gtbox_orm_sqlite.GTORMSqlite{left, right} {
		pool, err := instance.SqliteDB.DB()
		if err != nil {
			t.Fatal(err)
		}
		if err := pool.Ping(); err != nil {
			t.Fatalf("closing application storage affected an independent SDK instance: %v", err)
		}
	}
}

type storageTaskLegacyPayload struct {
	ability_task_model.AsyncTask
	InputPayload  string `gorm:"type:mediumtext"`
	ResultPayload string `gorm:"type:mediumtext"`
}

func (storageTaskLegacyPayload) TableName() string { return "async_tasks" }

func TestStorageTaskPayloadMigrationKeepsRows(t *testing.T) {
	previous := config.GlobalConfig
	t.Cleanup(func() {
		_ = CloseStorage()
		config.GlobalConfig = previous
	})
	config.GlobalConfig = &config.FileConfig{StorageCfg: &storage_config.StorageConfig{
		Sqlite: &storage_config.StorageSqliteConfig{
			Enabled:           true,
			GTORMSqliteConfig: gtbox_orm_sqlite.GTORMSqliteConfig{DBFilePath: filepath.Join(t.TempDir(), "migration.db")},
		},
	}}
	if err := OpenStorage(); err != nil {
		t.Fatal(err)
	}
	db, err := GormDB(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&storageTaskLegacyPayload{}); err != nil {
		t.Fatal(err)
	}
	legacy := &storageTaskLegacyPayload{
		AsyncTask:    ability_task_model.AsyncTask{TaskID: "00000000-0000-4000-8000-000000000001", Method: "existing", RunStatus: "queued"},
		InputPayload: strings.Repeat("x", 70<<10), ResultPayload: `{"preserved":true}`,
	}
	if err := db.Create(legacy).Error; err != nil {
		t.Fatal(err)
	}
	if err := GormAutoMigrate(); err != nil {
		t.Fatal(err)
	}
	var saved ability_task_model.AsyncTask
	if err := db.Where("task_id = ?", legacy.TaskID).Take(&saved).Error; err != nil {
		t.Fatal(err)
	}
	if saved.InputPayload != legacy.InputPayload || saved.ResultPayload != legacy.ResultPayload || saved.RunStatus != legacy.RunStatus {
		t.Fatal("payload migration changed an existing task")
	}
	saved.Model = gorm.Model{}
	if err := db.Create(&saved).Error; !IsDuplicateKeyError(err) {
		t.Fatalf("payload migration lost the unique task ID constraint: %v", err)
	}
}

func TestSqliteConcurrentReadAndRollback(t *testing.T) {
	previous := config.GlobalConfig
	t.Cleanup(func() {
		_ = CloseStorage()
		config.GlobalConfig = previous
	})
	config.GlobalConfig = &config.FileConfig{
		StorageCfg: &storage_config.StorageConfig{
			Sqlite: &storage_config.StorageSqliteConfig{
				Enabled: true,
				GTORMSqliteConfig: gtbox_orm_sqlite.GTORMSqliteConfig{
					DBFilePath: filepath.Join(t.TempDir(), "app.db"),
				},
			},
		},
	}
	if err := OpenStorage(); err != nil {
		t.Fatal(err)
	}
	db, err := GormDB(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&storageProbe{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&storageProbe{Name: "kept"}).Error; err != nil {
		t.Fatal(err)
	}
	rollbackErr := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&storageProbe{Name: "rolled"}).Error; err != nil {
			return err
		}
		return errors.New("rollback")
	})
	if rollbackErr == nil {
		t.Fatal("rollback transaction returned nil")
	}
	var rolled int64
	if err := db.Model(&storageProbe{}).Where("name = ?", "rolled").Count(&rolled).Error; err != nil {
		t.Fatal(err)
	}
	if rolled != 0 {
		t.Fatalf("rolled back row still present: %d", rolled)
	}

	const readers = 8
	var group sync.WaitGroup
	errs := make(chan error, readers+1)
	group.Add(readers + 1)
	for i := 0; i < readers; i++ {
		go func() {
			defer group.Done()
			handle, handleErr := GormDB(context.Background())
			if handleErr != nil {
				errs <- handleErr
				return
			}
			var seen int64
			if countErr := handle.Model(&storageProbe{}).Where("name = ?", "kept").Count(&seen).Error; countErr != nil {
				errs <- countErr
				return
			}
			if seen != 1 {
				errs <- errors.New("concurrent read missed the committed row")
			}
		}()
	}
	go func() {
		defer group.Done()
		handle, handleErr := GormDB(context.Background())
		if handleErr != nil {
			errs <- handleErr
			return
		}
		var createErr error
		for attempt := 0; attempt < 20; attempt++ {
			createErr = handle.Create(&storageProbe{Name: "extra"}).Error
			if createErr == nil || !strings.Contains(createErr.Error(), "database is locked") {
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
		if createErr != nil {
			errs <- createErr
		}
	}()
	group.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}
