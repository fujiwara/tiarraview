# tirraview

tiarraview is a simple viewer for tiarra logs.

## Usage

tiarraview

```
Usage: tiarraview <command> [flags]

Flags:
  -h, --help                            Show context-sensitive help.
      --dbfile="./db/database.sqlite3"

      --schemafile="./db/schema.sql"

Commands:
  server [flags]
    run web view server

  import --src-dir=STRING [flags]
    import log files to database

  init [flags]
    initialize database

Run "tiarraview <command> --help" for more information on a command.
```

### Initialize database

```console
$ tiarraview init
```

tiarraview initializes sqlite3 database. (default: `./db/database.sqlite3`)

### Import tiarra logs

```console
$ tiarraview import --src-dir=/path/to/tiarra/logs
```

tiarraview imports tiarra logs from the specified directory to sqlite3 database.

The directory structure should be like this:

```
tiarra/log
├── channel_name
│   ├── 2024.10.01.txt
│   ├── 2024.10.02.txt
```

### Start server

```console
$ tiarraview server
```

tiarraview starts a web server to view tiarra logs. You can access the server at `http://localhost:8080`.

## LICENSE

MIT

## Author

Fujiwara Shunichiro
