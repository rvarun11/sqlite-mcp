# SQLite MCP Server

A Model Context Protocol (MCP) server for SQLite database operations. This server provides a standardized interface for SQLite database interactions including schema introspection, query execution, and database modifications.

## Features

- **get_schema**: Complete schema introspection with column details, types, constraints, and indexes
- **query(sql)**: Safe execution of SELECT queries with result formatting  
- **execute(sql)**: Execution of DDL/DML operations (INSERT, UPDATE, DELETE, CREATE, ALTER, DROP, etc.)
- Built-in SQL operation validation and sanitized error messages

## Available Tools

The MCP server exposes three main tools:

#### get_schema

- Description: List all tables in the SQLite database with their schema information
- Parameters: None
- Usage: Provides complete schema introspection including columns, types, constraints, and indexes

#### query

- Description: Execute SELECT queries against the SQLite database
- Parameters: 
  - `sql` (required): SELECT query to execute
-Usage: Only SELECT, WITH, and EXPLAIN queries are allowed
- Example: `SELECT * FROM users WHERE age > 25`

#### execute

- Description: Execute DDL/DML operations against the SQLite database
- Parameters:
  - `sql` (required): SQL statement to execute (non-SELECT operations)
- Usage: INSERT, UPDATE, DELETE, CREATE, ALTER, DROP operations
- Example: `INSERT INTO users (name, email) VALUES ('John Doe', 'john@example.com')`


## Get Started

### Prerequisites

- **Go**: SQLite MCP Server requires Go 1.24.4 or later. Download from [go.dev/doc/install](https://go.dev/doc/install)
- **Task**: Install the [go-task](https://taskfile.dev/) to run automated development tasks. Install using Homebrew with `brew install go-task`
- **SQLite3**: For creating and managing SQLite databases locally
- **Docker**: To run the server with Docker (optional)

### Installation

1. Clone the repository:
```bash
git clone https://github.com/rvarun11/sqlite-mcp.git
cd sqlite-mcp
```

2. Build:
```bash
# Builds the binary and the example database
task build

# Or if you prefer Docker:
task docker-build
```

### MCP Client Configuration

After installation, add the following configuration to your MCP client:

#### Using built binary:

```json
{
  "mcpServers": {
    "sqlite": {
      "command": "/path/to/your/sqlite-mcp/build/sqlite-mcp",
      "args": [
        "--database",
        "/path/to/your/database.db"
      ]
    }
  }
}
```
Args:
- `--database, -d`: Path to SQLite database file (required)
- `--debug`: Enable debug mode for verbose logging (optional)

#### Using Docker:

```json
{
  "mcpServers": {
    "sqlite": {
      "command": "/path/to/your/.docker/bin/docker",
      "args": [
        "run",
        "-i",
        "--rm",
        "sqlite-mcp-server"
      ]
    }
  }
}
```

The above uses `example.sql` to build the example database. To use a custom schema sql, [see the Docker commands below](#docker).

**Important Note for GUI Applications**: When configuring MCP clients in GUI applications (like Claude Desktop), you must use absolute paths for both the command and database file paths. Do not use:
- Tilde (`~`) for home directory shortcuts
- Environment variables like `$HOME` or `$PATH`
- Relative paths like `./build/sqlite-mcp`
- Command shortcuts that rely on PATH resolution (like just `docker`)

## Development

To see a list of all available development tasks, run:
```bash
task --list
```

Available tasks include:
- `fmt`: Tidy modules and format code
- `lint`: Run `goclangci-lint` static analysis
- `test:` Run unit tests
- `check`: Run all the code quality checks including `fmt`, `lint`, and `test`
- `build-example-db`: Create example db from `example.sql`
- `run-dev`: Run from source with example db, includes all checks
- `build`: Build the binary with example db, including all checks
- `docker-build`: Build Docker image


### Run the Build

#### Binary

- After building the binary with `task build` or `task build-with-db`, run:
```bash
./build/sqlite-mcp --database ./build/example.db [--debug]
```

#### Docker 

- After building the docker image with `task docker-build`, run the Docker container:
```bash
# Run with default example.sql database
docker run -i --rm sqlite-mcp-server

# Or run with custom schema.sql file
docker run -i --rm -v "/path/to/your/schema.sql:/data/schema.sql" sqlite-mcp-server
```
