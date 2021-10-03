package migrate

const testUsersTableMigration = `
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
`

const testAlterUserTableMigration = `
-- add your UP SQL here
[STATEMENT] ALTER TABLE users MODIFY phone VARCHAR(50) NULL;

-- [DIRECTION] -- do not alter this line!
-- add your DOWN SQL here
[STATEMENT] ALTER TABLE users MODIFY phone BIGINT NULL;
`

const testTableOne = `
-- add your UP SQL here

[STATEMENT] CREATE TABLE widgets (
	id INT(6) UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    description VARCHAR(1000),
    sku VARCHAR(50) NOT NULL,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- [DIRECTION] -- do not alter this line!
-- add your DOWN SQL here

[STATEMENT] DROP TABLE widgets;	
`

const testAlterTableOne = `
-- add your UP SQL here

[STATEMENT] ALTER TABLE widgets ADD price FLOAT NULL AFTER id;

-- [DIRECTION] -- do not alter this line!
-- add your DOWN SQL here

[STATEMENT] ALTER TABLE widgets DROP COLUMN price;
`
