package migrate

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/blainemoser/MySqlDB/database"
	trysql "github.com/blainemoser/TrySql"
)

var (
	ts           *trysql.TrySql     = nil
	db           *database.Database = nil
	responseCode int
)

const (
	TEST_DIR     = "mysql_migrate_testing_directory"
	SEEDED_QUERY = "select * from migrations where name in (?, ?)"
	TEST_DB_NAME = "test_mysql_mig"
)

func TestMain(m *testing.M) {
	defer recovery()
	var err error
	ts, err = trysql.Initialise([]string{"-v", "latest"})
	if err != nil {
		panic(err)
	}
	getDatabase()
	responseCode = m.Run()
}

func recovery() {
	r := recover()
	if r != nil {
		removeDB()
		trySqlTD()
		panic(r)
	}
	removeDB()
	trySqlTD()
	os.Exit(responseCode)
}

func TestSeed(t *testing.T) {
	// First we will create the new folder
	path, err := initTestDir()
	if err != nil {
		t.Error(err)
	}
	makeFiles, err := seedMigrations(path)
	if err != nil {
		t.Error(err)
	}
	m := Make(db, path)
	_, err = m.MigrateUp()
	if err != nil {
		t.Error(err)
	}
	checkTableSeeded(t, m, makeFiles)
	reset()
}

func TestCreate(t *testing.T) {
	path, err := getTestDir()
	f := "create_some_test_table"
	if err != nil {
		t.Error(err)
	}
	m := Make(db, path)
	fullPath, _, _, err := m.Create(f)
	if err != nil {
		t.Error(err)
	}
	created, err := DirExists(fullPath)
	if err != nil {
		t.Error(err)
		return
	}
	if !created {
		t.Errorf("expected file '%s' to have been created", f)
	}
	reset()
}

func TestMigrateUpAndDown(t *testing.T) {
	checkMigrateUp(t)
	checkMigrateDown(t)
	reset()
}

func checkMigrateUp(t *testing.T) {
	path, err := getTestDir()
	if err != nil {
		t.Error(err)
		return
	}
	runFirstMigration(t, path)
	fullname := createFaultyMigration(t, path)
	runSecondMigration(t, path, fullname)
}

func runFirstMigration(t *testing.T, path string) {
	err := createMigFile("create_table_widgets", path, TEST_TABLE_ONE)
	if err != nil {
		t.Error(err)
		return
	}
	_, err = Make(db, path).MigrateUp()
	if err != nil {
		t.Error(err)
		return
	}
	checkMigratedOne(t)
	// Try run again; should have nothing to migrate
	_, err = Make(db, path).MigrateUp()
	if err != nil {
		t.Error(err)
	}
}

// This test also tests that the faulty migration has been handled correctly
func runSecondMigration(t *testing.T, path, faultyMigName string) {
	err := createMigFile("alter_table_widgets_add_price_column", path, TEST_ALTER_TABLE_ONE)
	if err != nil {
		t.Error(err)
		return
	}

	// This will try run the migration that has been created as well as the faulty one
	_, err = Make(db, path).MigrateUp()
	if err != nil {
		t.Error(err)
		return
	}
	checkFaultyMigration(t, faultyMigName)
	checkMigratedTwo(t)
}

func createFaultyMigration(t *testing.T, path string) string {
	fullname, err := createMigWithoutFile("alter_table_widgets_add_pricing_type_column", path, TEST_MIG_TO_BE_REMOVED)
	if err != nil {
		t.Error(err)
		return ""
	}
	return fullname
}

func checkMigrateDown(t *testing.T) {
	path, err := getTestDir()
	if err != nil {
		t.Error(err)
		return
	}
	_, err = Make(db, path).MigrateDown()
	if err != nil {
		t.Error(err)
		return
	}
	checkDownMigrationResult(t)
}

func createMigFile(f, path, migContent string) error {
	m := Make(db, path)
	fullPath, _, _, err := m.Create(f)
	if err != nil {
		return err
	}
	// The user will alter the SQL accordingly...
	return writeFile(fullPath, migContent)
}

func createMigWithoutFile(f, path, migContent string) (fullname string, err error) {
	m := Make(db, path)
	fullpath, fullname, _, err := m.Create(f)
	if err != nil {
		return
	}
	err = deleteFile(fullpath)
	return
}

func getDatabase() {
	dataB, err := database.MakeSchemaless(&database.Configs{
		Host:     "127.0.0.1",
		Port:     ts.HostPortStr(),
		Username: "root",
		Password: ts.Password(),
		Driver:   "mysql",
	})
	if err != nil {
		panic(err)
	}
	db = &dataB
	db.Exec(fmt.Sprintf("DROP SCHEMA %s", TEST_DB_NAME), nil)
	_, err = db.Exec(fmt.Sprintf("CREATE SCHEMA %s", TEST_DB_NAME), nil)
	if err != nil {
		panic(err)
	}
	db.SetSchema(TEST_DB_NAME)
}

func trySqlTD() {
	if ts != nil {
		err := ts.TearDown()
		if err != nil {
			log.Println(err.Error())
		}
	}
}

func removeDB() {
	err := clearTestDir()
	if err != nil {
		log.Println(err)
	}
	if db != nil {
		db.Close()
	}
}

func clearTestDir() error {
	var err error
	testDir, _ := getTestDir()
	if len(testDir) > 0 {
		err = os.RemoveAll(testDir)
	}
	return err
}

func reset() {
	_, err := db.Exec("DROP TABLE IF EXISTS migrations;", nil)
	if err != nil {
		panic(err)
	}
	err = clearTestDir()
	if err != nil {
		panic(err)
	}
}

func getTestDir() (string, error) {
	path, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%s", path, TEST_DIR), nil
}

func seedMigrations(path string) (map[string]string, error) {
	makeFiles, err := getFileContent(path)
	if err != nil {
		return nil, err
	}
	var errs []string
	for name, content := range makeFiles {
		err = writeFile(name, content)
		if err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf(strings.Join(errs, ", "))
	}
	return makeFiles, nil
}

func getFileContent(path string) (map[string]string, error) {
	result := make(map[string]string)
	lastYear := getLastYear()
	createMig := fmt.Sprintf(
		"%s/create_users_table.%s.sql",
		path,
		strconv.FormatInt(lastYear.Unix(), 10),
	)
	alterMig := fmt.Sprintf(
		"%s/alter_users_table_change_phone_varchar.%s.sql",
		path,
		strconv.FormatInt(lastYear.Unix()+1, 10),
	)
	result[createMig] = TEST_USERS_TABLE_MIG
	result[alterMig] = TEST_ALTER_USERS_MIG
	return result, nil
}

func getLastYear() time.Time {
	return time.Now().Add(time.Hour * -8766)
}

func writeFile(name, content string) error {
	return os.WriteFile(name, []byte(content), 0755)
}

func deleteFile(name string) error {
	return os.Remove(name)
}

func initTestDir() (string, error) {
	path, err := getTestDir()
	if err != nil {
		return "", err
	}
	err = makeTestDir(path)
	if err != nil {
		return "", err
	}
	return path, nil
}

func makeTestDir(path string) error {
	exists, err := DirExists(path)
	if err != nil {
		return err
	}
	if !exists {
		err = os.Mkdir(path, 0777)
		if err != nil {
			return err
		}
	}
	return nil
}

func checkTableSeeded(t *testing.T, m *Migration, shouldHave map[string]string) {
	checkHasMigrationsTable(t, m)
	escaped := make([]interface{}, 0)
	for name := range shouldHave {
		escaped = append(escaped, getMigName(name))
	}
	rows, err := m.database.QueryRaw(SEEDED_QUERY, escaped)
	if err != nil {
		t.Error(err)
		return
	}
	if len(rows) < 1 {
		t.Errorf("no migrations added to migrations table")
	}
	checkSeededRows(t, rows)
	// This will check that the result of the migrations has been applied
	checkUsersTable(t, m)
}

func checkHasMigrationsTable(t *testing.T, m *Migration) {
	hasTable, err := m.database.CheckHasTable("migrations")
	if err != nil {
		t.Error(err)
	}
	if !hasTable {
		t.Errorf("expected table 'migrations' to exist")
	}
}

func checkSeededRows(t *testing.T, rows []map[string]interface{}) {
	// Check that the migrations ran
	for i := 0; i < len(rows); i++ {
		if rows[i]["migrated"] == nil {
			t.Errorf("expected migration record to have the column 'migrated'")
		}
		if rows[i]["name"] == nil {
			t.Errorf("expected migration record to have the column 'name'")
		}
		isRowMigrated(t, rows[i])
	}
}

// This removes the path and extension from the filename
func getMigName(name string) string {
	nameSplit := strings.Split(name, "/")
	if len(nameSplit) < 2 {
		return name
	}
	return strings.Replace(nameSplit[len(nameSplit)-1], ".sql", "", 1)
}

func isRowMigrated(t *testing.T, row map[string]interface{}) {
	var migrated bool
	if isMig, ok := row["migrated"].(int64); ok {
		migrated = isMig == 1
	} else if isMig, ok := row["migrated"].(int); ok {
		migrated = isMig == 1
	} else if isMig, ok := row["migrated"].(string); ok {
		isMigInt, err := strconv.ParseInt(isMig, 10, 8)
		if err != nil {
			t.Error(err)
		}
		migrated = isMigInt == 1
	} else {
		t.Errorf("expected migration record to have the column 'migrated'")
	}
	if !migrated {
		t.Errorf("expected migration record '%s' to have the column 'migrated'", row["name"])
	}
}

func checkUsersTable(t *testing.T, m *Migration) {
	hasTable, err := m.database.CheckHasTable("users")
	if err != nil {
		t.Error(err)
		return
	}
	if !hasTable {
		t.Errorf("expected 'users' table to exist")
		return
	}
	details, err := m.database.QueryRaw("describe users", nil)
	if err != nil {
		t.Error(err)
		return
	}
	if !phoneChanged(details) {
		t.Errorf("expected 'phone' field in the 'users' table to have changed to a varchar(50)")
	}
}

func phoneChanged(details []map[string]interface{}) bool {
	for _, row := range details {
		if row["Field"] == nil || row["Type"] == nil {
			continue
		}
		if field, ok := row["Field"].(string); ok {
			if typeCol, ok := row["Type"].(string); ok {
				if field == "phone" && typeCol == "varchar(50)" {
					return true
				}
			}
		}
	}
	return false
}

func hasPrice(details []map[string]interface{}) bool {
	for _, row := range details {
		if row["Field"] == nil || row["Type"] == nil {
			continue
		}
		if field, ok := row["Field"].(string); ok {
			if typeCol, ok := row["Type"].(string); ok {
				if field == "price" && typeCol == "float" {
					return true
				}
			}
		}
	}
	return false
}

func hasFaultyMigration(details []map[string]interface{}) (bool, error) {
	if len(details) < 1 {
		return false, errors.New("failed to run query to check for faulty migration")
	}
	for _, row := range details {
		if row["has_mig"] == nil {
			continue
		}
		if count, ok := row["has_mig"].(int64); ok {
			return count > 0, nil
		}
	}
	return false, errors.New("query for faulty migration has a problem")
}

func checkMigratedOne(t *testing.T) {
	hasWidgetsTable, err := db.CheckHasTable("widgets")
	if err != nil {
		t.Error(err)
		return
	}
	if !hasWidgetsTable {
		t.Errorf("expected 'widgets' table to have been created")
	}
}

func checkMigratedTwo(t *testing.T) {
	details, err := db.QueryRaw("describe widgets", nil)
	if err != nil {
		t.Error(err)
		return
	}
	if !hasPrice(details) {
		t.Errorf("expected 'price' field in the 'widgets' table")
	}
}

func checkFaultyMigration(t *testing.T, fullname string) {
	details, err := db.QueryRaw("select count(*) as has_mig from migrations where name = ?", []interface{}{fullname})
	if err != nil {
		t.Error(err)
		return
	}
	hasFault, err := hasFaultyMigration(details)
	if err != nil {
		t.Fatal(err)
	}
	if hasFault {
		t.Errorf("expected faulty migration to have been deleted from the migrations table")
	}
}

func checkDownMigrationResult(t *testing.T) {
	// The price field should have been removed in the widgets table
	details, err := db.QueryRaw("describe widgets", nil)
	if err != nil {
		t.Error(err)
		return
	}
	if hasPrice(details) {
		t.Errorf("expected 'price' field in the 'widgets' table to have been removed")
	}
}
