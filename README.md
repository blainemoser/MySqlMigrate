# MySqlMigrate

This package manages database migrations and versioning for MySQL. 

It's best used in conjunction with a command-line programme.

## Usage

### Create migration files

```go
package main

import (
	"log"

	database "github.com/blainemoser/MySqlDB"
	migrate "github.com/blainemoser/MySqlMigrate"
)

func main() {
	configs := &database.Configs{
		Host:     "127.0.0.1",
		Port:     "3306",
		Username: "root",
		Password: "secret",
		Driver:   "mysql",
		Database: "name_of_schema",
	}
	db, err := database.Make(configs)
	if err != nil {
		log.Fatal(err)
	}
	m := migrate.Make(&db, "/path/to/migrations/folder")
	_, err = m.Create("name_of_migration_eg_create_users_table")
	if err != nil {
		log.Fatal(err)
	}
}
```
To create a migration file, first create a pointer to `migrate.Migration` using [migrate.Make(db \*database.Database, path string) \*migrate.Migration](https://github.com/blainemoser/MySqlMigrate/blob/d4e9073b60967a68466eecd44455bf1fff5b96af/migrate.go#L55). 
> The first argument (**\*database.Database**) provides the connection to MySQL. The **path** string indicates the directory to which to save the migration files. If the directory does not exist it will be created, provided the base directory exists.

Second, call [Create(name string) (string, error)](https://github.com/blainemoser/MySqlMigrate/blob/d4e9073b60967a68466eecd44455bf1fff5b96af/migrate.go#L157) on the pointer. Provide the name of the migration as the sole argument. 

> This will not create the relavant schema. This can be achieved by using the function [database.MakeSchemaless(configs \*Configs) (Database, error)](https://github.com/blainemoser/MySqlDB/blob/6ac74670d7b24b6c82afb21be086c7afc139b384/database.go#L51), followed by [database.Exec("create schema name_of_schema", nil)](https://github.com/blainemoser/MySqlDB/blob/6ac74670d7b24b6c82afb21be086c7afc139b384/database.go#L70).

This will create a `.sql` file in the specified directory that looks as follows:
```sql
-- add your UP SQL here

-- [DIRECTION] -- do not alter this line!
-- add your DOWN SQL here
```

Alter the SQL appropriately, for instance:
```sql
-- add your UP SQL here

[STATEMENT] CREATE TABLE users (
	id INT(6) UNSIGNED AUTO_INCREMENT PRIMARY KEY,
	password VARCHAR(255) NOT NULL, 
    	role int(10) NOT NULL,
	name VARCHAR(1000) NOT NULL,
    	email VARCHAR(1000) NOT NULL,
    	phone BIGINT,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- [DIRECTION] -- do not alter this line!
-- add your DOWN SQL here

[STATEMENT] DROP TABLE 'users';	
```
Execute multiple statements in one migration by using `[STATEMENT]` to separate the queries.

Do not delete the line `-- [DIRECTION] -- do not alter this line!`, it delineates between the "up" SQL and the "down" SQL (which will be run if and when the migration is reversed).

### Run and Reverse the Migrations
```go
err := migrate.Make(&db, "/path/to/migrations/folder").MigrateUp()
```
Use the function [\*migrate.Migration.MigrateUp() error](https://github.com/blainemoser/MySqlMigrate/blob/d4e9073b60967a68466eecd44455bf1fff5b96af/migrate.go#L65) to run any migrations that have yet to be executed. 

```go
err := migrate.Make(&db, "/path/to/migrations/folder").MigrateDown()
```
Use the function [\*migrate.Migration.MigrateDown() error](https://github.com/blainemoser/MySqlMigrate/blob/d4e9073b60967a68466eecd44455bf1fff5b96af/migrate.go#L70) to reverse the migrations; this will execute the "down" SQL specified in the migration files.

> **Note** that reversing migrations does so in batches; groupings of migrations that were run "up" at the same time. It will not reverse _all_ migrations unless they were all run at the same time.
