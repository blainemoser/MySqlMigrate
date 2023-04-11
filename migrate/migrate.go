package migrate

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/blainemoser/MySqlDB/database"
	_ "github.com/go-sql-driver/mysql"
)

const (
	MIGS_QUERY = `
	SELECT migration_id, name 
	FROM migrations 
	WHERE migrations.migrated = ? [batch] 
	ORDER BY migration_id [order];
`

	MIGS_TABLE = `CREATE TABLE migrations (
	id INT(6) UNSIGNED AUTO_INCREMENT PRIMARY KEY,
	migration_id BIGINT UNSIGNED,
	batch_id BIGINT UNSIGNED,
	name VARCHAR(1000) NOT NULL,
	migrated TINYINT,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
)`
	MIGRATE_DEFAULT = `-- add your UP SQL here

-- [DIRECTION] -- do not alter this line!
-- add your DOWN SQL here

`
	LAST_BATCH_QUERY = "SELECT batch_id FROM migrations where migrated = 1 ORDER BY migration_id DESC LIMIT 1"
	EXISTS_QUERY     = "SELECT count(*) as taken FROM migrations WHERE name = ?;"
	PERM             = 0700 // this is to give the caller full rights, but no other user or group.
)

var (
	Permission os.FileMode = PERM
)

// Migration is a migration
type Migration struct {
	direction  bool
	migrations map[int]string
	database   *database.Database
	path       string
}

// Make creates a new migration
func Make(database *database.Database, path string) *Migration {
	return &Migration{
		direction:  true,
		migrations: nil,
		database:   database,
		path:       path,
	}
}

func (m *Migration) MigrateUp() (string, error) {
	m.direction = true
	return m.migrate()
}

func (m *Migration) MigrateDown() (string, error) {
	m.direction = false
	return m.migrate()
}

func (m *Migration) migrate() (string, error) {
	err := m.bootstrap()
	if err != nil {
		return "", err
	}
	return m.runMigrations()
}

func (m *Migration) runMigrations() (message string, err error) {
	batchID := time.Time.Unix(time.Now())
	var sql string
	var properties map[string]interface{}
	var msg string
	messages := make([]string, 0)
	for _, id := range m.getSequenceIDs() {
		if len(m.migrations[id]) < 1 {
			continue
		}
		sql = m.migrations[id]
		properties = m.getProperties(id, batchID)
		if msg, err = m.executeMigration(sql, id, message); err != nil {
			return
		}
		messages = append(messages, msg)
		if _, err = m.database.MakeRecord(properties, "migrations").Update("migration_id"); err != nil {
			return
		}
	}
	message = fmt.Sprintf("%s migrations %s", m.getDirectionMessage(), strings.Join(messages, ", "))
	return
}

func (m *Migration) getSequenceIDs() []int {
	var sequenceIDs []int
	for sequence := range m.migrations {
		sequenceIDs = append(sequenceIDs, sequence)
	}
	sort.Ints(sequenceIDs)
	if !m.direction {
		sort.Sort(sort.Reverse(sort.IntSlice(sequenceIDs)))
	}
	return sequenceIDs
}

func (m *Migration) getProperties(id int, batchID int64) map[string]interface{} {
	properties := make(map[string]interface{})
	properties["migration_id"] = id
	if m.direction {
		properties["migrated"] = "1"
		properties["batch_id"] = strconv.FormatInt(batchID, 10)
	} else {
		properties["migrated"] = "0"
	}
	return properties
}

func (m *Migration) getDirectionMessage() string {
	if m.direction {
		return "Executed"
	}
	return "Reversed"
}

func (m *Migration) executeMigration(sql string, id int, message string) (mesage string, err error) {

	// Split by the individual statements in the query
	sqlSplit := strings.Split(sql, "[STATEMENT]")

	for _, sqlString := range sqlSplit {
		if strings.Replace(sqlString, " ", "", -1) == "" {
			continue
		}
		_, err = m.database.Exec(sqlString, nil)
		if err != nil {
			return
		}
	}
	message = fmt.Sprintf("%s migration #%d", message, id)
	return message, nil
}

// Create makes a new migration file
func (m *Migration) Create(migrationName string) (fullPath, message string, err error) {
	err = m.bootstrap()
	if err != nil {
		return
	}
	migrationName = migrationName + "." + strconv.FormatInt(time.Now().UnixNano(), 10)
	if err = m.alreadyExists(migrationName); err != nil {
		return
	}
	if fullPath, err = m.getFile(migrationName); err != nil {
		return
	}
	if message, err = m.createMigrationRecord(migrationName); err != nil {
		return
	}
	return
}

func (m *Migration) getFile(name string) (string, error) {
	file := []byte(MIGRATE_DEFAULT)
	fullPath := fmt.Sprintf("%s/%s.sql", m.path, name)
	if err := os.WriteFile(fullPath, file, Permission); err != nil {
		return "", err
	}
	os.Chmod(fullPath, Permission)
	return fullPath, nil
}

func (m *Migration) alreadyExists(name string) error {
	var exists bool
	var err error
	if exists, err = m.exists(name); err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("migration '%s' already exists", name)
	}
	return nil
}

func (m *Migration) bootstrap() error {
	if err := m.initTable(); err != nil {
		return err
	}
	if err := m.initDir(); err != nil {
		return err
	}
	if err := m.seed(); err != nil {
		return err
	}
	return m.getMigrationsSQL()
}

func (m *Migration) initTable() error {
	hasTable, err := m.database.CheckHasTable("migrations")
	if err != nil {
		return err
	}
	if !hasTable {
		return m.createTable()
	}
	return nil
}

func (m *Migration) initDir() error {
	exists, err := m.hasPathDir()
	if err != nil {
		return err
	}
	if !exists {
		return os.Mkdir(m.path, Permission)
	}
	return nil
}

func (m *Migration) hasPathDir() (bool, error) {
	return DirExists(m.path)
}

func (m *Migration) createTable() error {
	_, err := m.database.Exec(MIGS_TABLE, nil)
	return err
}

func (m *Migration) seed() error {
	files, err := m.findFiles()
	if err != nil {
		return err
	}
	errs := make([]error, len(files))
	// result := make([]string, 0)
	var migName string
	for i := 0; i < len(files); i++ {
		migName = files[i]
		_, err := m.seedMigrationRecord(migName, i+1)
		errs[i] = err
	}
	// Pull list any errors, if any
	return GetErrors(errs)
}

func (m *Migration) seedMigrationRecord(name string, id int) (message string, err error) {
	// Lookup the migration by the name
	exists, err := m.exists(name)
	if err != nil {
		return "", err
	}
	if exists {
		return "", nil
	}
	insertID, err := m.database.MakeRecord(m.zeroDayProperties(id, name), "migrations").Create()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Migration '%s' with id %d created successfully!", name, insertID), nil
}

func (m *Migration) zeroDayProperties(id interface{}, name string) map[string]interface{} {
	return map[string]interface{}{
		"migration_id": id,
		"batch_id":     0,
		"name":         name,
		"migrated":     0,
	}
}

func (m *Migration) createMigrationRecord(name string) (string, error) {
	now := time.Time.Unix(time.Now())
	insertID, err := m.database.MakeRecord(m.zeroDayProperties(now, name), "migrations").Create()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Migration '%s' with id %d created successfully!", name, insertID), nil
}

func (m *Migration) exists(name string) (bool, error) {
	checks := make([]interface{}, 0)
	checks = append(checks, name)
	exists, err := m.database.QueryRaw(EXISTS_QUERY, checks)
	if err != nil {
		return false, err
	}
	if taken, ok := (exists[0]["taken"]).(int64); ok {
		return taken > 0, nil
	}
	return false, errors.New("error in existence checker")
}

// Lists files in migrations directory
func (m *Migration) getMigrationsSQL() error {
	files, err := m.findFiles()
	if err != nil {
		return err
	}
	migs, err := m.getMigs()
	if err != nil {
		return err
	}
	// Check the files found against the database
	result := make(map[int]string)
	routineFailed := make([]string, 0)
	for _, v := range migs {
		err = m.appendContents(&result, &routineFailed, files, v)
		if err != nil {
			return err
		}
	}
	m.migrations = result
	return nil
}

func (m *Migration) getNameAndID(v map[string]interface{}) (string, int64, error) {
	name, ok := (v["name"]).(string)
	if !ok {
		return "", 0, errors.New("name of migration is not a string")
	}
	if id, ok := (v["migration_id"]).(int64); ok {
		return name, id, nil
	} else if id, ok := (v["migration_id"]).(int); ok {
		return name, int64(id), nil
	} else if id, ok := (v["migration_id"]).(string); ok {
		cID, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			return "", 0, fmt.Errorf("migration id is not an integer, %s", err.Error())
		}
		return name, cID, nil
	}
	return "", 0, errors.New("migration id is not an integer")
}

func (m *Migration) appendContents(
	result *map[int]string,
	routineFailed *[]string,
	files []string,
	value map[string]interface{},
) error {
	name, id, err := m.getNameAndID(value)
	if err != nil {
		return err
	}
	numFiles := len(files)
	nameInFile(name, files, numFiles, routineFailed)
	if len(*routineFailed) > 0 {
		return errors.New("Field(s) not in field list: " + strings.Join(*routineFailed, "; "))
	}
	contents, err := GetFileContents(m.path + "/" + name + ".sql")
	if err != nil {
		return errors.New("Could not get contents for migration " + name + " (id " + strconv.FormatInt(id, 10) + ")")
	}
	(*result)[int(id)] = m.getMigContents(contents)
	return nil
}

func (m *Migration) findFiles() ([]string, error) {
	files := make(map[int]string)
	keys := make([]int, 0)
	var key int
	err := filepath.Walk(m.path, getWalkFunc(key, &files, &keys))
	if err != nil {
		return nil, err
	}
	return fileResult(keys, files)
}

func getWalkFunc(key int, files *map[int]string, keys *[]int) filepath.WalkFunc {
	return func(path string, fileInfo os.FileInfo, err error) error {
		if !fileInfo.IsDir() && len(fileInfo.Name()) > 4 && strings.Index(fileInfo.Name(), ".sql") == len(fileInfo.Name())-4 {
			fileSplit := strings.Split(fileInfo.Name(), ".")
			if len(fileSplit) != 3 {
				return errors.New("Migration name is malformed: Should be {name}.{timestamp}.sql")
			}
			fileName := fileSplit[0] + "." + fileSplit[1]
			key, err = strconv.Atoi(fileSplit[1])
			if err != nil {
				return errors.New("could not parse migration file timestamp")
			}
			(*files)[key] = fileName
			*keys = append(*keys, key)
		}
		return nil
	}
}

func fileResult(keys []int, files map[int]string) ([]string, error) {
	sort.Ints(keys)
	result := make([]string, 0)
	for _, v := range keys {
		if files[v] != "" {
			result = append(result, files[v])
		}
	}
	return result, nil
}

func (m *Migration) getMigs() ([]map[string]interface{}, error) {
	upOrDown := make([]interface{}, 0)
	order := "ASC"
	batch, err := m.getLastBatch()
	if err != nil {
		return nil, err
	}
	batchStr := ""
	if m.direction {
		upOrDown = append(upOrDown, "0")
	} else {
		upOrDown = append(upOrDown, "1")
		order = "DESC"
	}
	query := strings.Replace(MIGS_QUERY, "[order]", order, -1)
	if batch != 0 {
		batchStr = "and batch_id = ?"
		upOrDown = append(upOrDown, strconv.FormatInt(batch, 10))
	}
	query = strings.Replace(query, "[batch]", batchStr, -1)
	return m.database.QueryRaw(query, upOrDown)
}

func (m *Migration) getMigContents(contents string) string {
	result := strings.Split(contents, "[DIRECTION]")
	if m.direction {
		return result[0]
	}

	return result[1]
}

func nameInFile(name string, files []string, numFiles int, routineFailed *[]string) {
	nameInFile := false
	for i := 0; i < numFiles; i++ {
		if name == files[i] {
			nameInFile = true
		}
	}

	if !nameInFile {
		*routineFailed = append(*routineFailed, name)
	}
}

func (m *Migration) getLastBatch() (int64, error) {
	if m.direction {
		return 0, nil
	}
	result, err := m.database.QueryRaw(LAST_BATCH_QUERY, nil)
	if err != nil {
		return 0, err
	}
	if len(result) < 1 {
		return 0, nil
	}
	return getBatchID(result[0]["batch_id"])
}

func getBatchID(batchID interface{}) (int64, error) {
	if id, ok := batchID.(int64); ok {
		return id, nil
	} else if id, ok := batchID.(string); ok {
		idInt, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			return 0, err
		}
		return idInt, nil
	} else if id, ok := batchID.(int); ok {
		return int64(id), nil
	}
	return 0, fmt.Errorf("batch id not an int 64")
}

// GetFileContents gets file contents from the file at the path
func GetFileContents(fileName string) (string, error) {
	file, err := os.OpenFile(fileName, os.O_RDONLY, Permission)
	if err != nil {
		return "", err
	}
	fi, err := file.Stat()
	if err != nil {
		return "", err
	}
	fileUnmarshalled := make([]byte, int(fi.Size()))
	fileRead, err := file.Read(fileUnmarshalled)
	if err != nil {
		return "", err
	}
	fileContents := string(fileUnmarshalled[:fileRead])
	// Close the file
	if err := file.Close(); err != nil {
		log.Fatal(err)
	}
	return fileContents, nil
}

func DirExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}

func GetErrors(errs []error) error {
	if len(errs) < 1 {
		return nil
	}
	result := make([]string, 0)
	for _, err := range errs {
		if err != nil {
			result = append(result, err.Error())
		}
	}
	if len(result) < 1 {
		return nil
	}
	return fmt.Errorf(strings.Join(result, ", "))
}
