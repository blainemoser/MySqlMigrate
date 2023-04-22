package migrate

const (
	TEST_USERS_TABLE_MIG = `
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

	TEST_ALTER_USERS_MIG = `
-- add your UP SQL here
[STATEMENT] ALTER TABLE users MODIFY phone VARCHAR(50) NULL;

-- [DIRECTION] -- do not alter this line!
-- add your DOWN SQL here
[STATEMENT] ALTER TABLE users MODIFY phone BIGINT NULL;
`

	TEST_TABLE_ONE = `
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

	TEST_ALTER_TABLE_ONE = `
-- add your UP SQL here

[STATEMENT] ALTER TABLE widgets ADD price FLOAT NULL AFTER id;

-- [DIRECTION] -- do not alter this line!
-- add your DOWN SQL here

[STATEMENT] ALTER TABLE widgets DROP COLUMN price;
`

	TEST_MIG_TO_BE_REMOVED = `
-- add your UP SQL here

[STATEMENT] ALTER TABLE widgets ADD pricing_type int(6) NULL AFTER id;

-- [DIRECTION] -- do not alter this line!
-- add your DOWN SQL here

[STATEMENT] ALTER TABLE widgets DROP COLUMN pricing_type;
`
)
