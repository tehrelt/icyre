drop index if exists roles_uid_role_unq;

DROP TABLE IF EXISTS roles;

DROP TABLE IF EXISTS users;

DROP TYPE IF EXISTS ROLE;

DROP EXTENSION IF EXISTS "uuid-ossp";