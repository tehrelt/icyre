CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TYPE ROLE AS ENUM ('admin', 'operator', 'regular');

CREATE TABLE IF NOT EXISTS users (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  email VARCHAR UNIQUE NOT NULL,
  hashed_password VARCHAR NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT now(),
  updated_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS roles (
  uid UUID,
  role ROLE NOT NULL DEFAULT 'regular'
);

create unique index roles_uid_role_unq on roles (uid, role);